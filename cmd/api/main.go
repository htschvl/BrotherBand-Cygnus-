// Package main is the composition root. It loads configuration,
// constructs every adapter, wires the use cases against the
// resolved ports, mounts the HTTP router, and runs the server with
// graceful shutdown.
//
// This is the *only* file in the codebase that imports from every
// layer; that one-place coupling is the whole point of the dependency
// rule the rest of the project obeys.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/auth"
	httplayer "github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/handler"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/respond"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/persistence/postgres"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/storage/r2"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/infrastructure/config"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/infrastructure/observability"
	infrapostgres "github.com/htschvl/BrotherBand-Cygnus-/internal/infrastructure/postgres"
	infras3 "github.com/htschvl/BrotherBand-Cygnus-/internal/infrastructure/s3"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/clock"
	usecasebb "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/brotherband"
	usecasemedia "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/media"
	usecasemsg "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/message"
	usecaseuser "github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/user"
)

// version is set at build time via -ldflags="-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		// run() has already logged the specific cause with context;
		// this is the final, unconditional failure marker.
		slog.Error("startup failed; process exiting",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}
}

func run() (err error) {
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		// The logger is not configured yet, so use the stdlib default.
		slog.Error("configuration invalid", slog.String("error", cfgErr.Error()))
		return cfgErr
	}

	logger := observability.NewLogger(os.Getenv("LOG_LEVEL"))
	metrics := observability.NewMetrics()

	// Any panic during wiring or shutdown is converted to a returned
	// error so `main` exits non-zero with a structured log line rather
	// than an unrecovered stack dump.
	defer func() {
		if rec := recover(); rec != nil {
			logger.Error("panic during startup/shutdown",
				slog.Any("panic", rec),
			)
			err = errlike(rec)
		}
	}()

	logger.Info("starting brotherband api",
		slog.String("version", version),
		slog.String("env", string(cfg.Env)),
		slog.String("addr", cfg.HTTP.Addr),
	)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ─── Infrastructure ───────────────────────────────────────────────
	pool, poolErr := infrapostgres.NewPool(rootCtx, infrapostgres.PoolConfig{DSN: cfg.DB.DSN})
	if poolErr != nil {
		logger.Error("postgres pool init failed", slog.String("error", poolErr.Error()))
		return poolErr
	}
	defer pool.Close()
	logger.Info("postgres pool ready")

	if migErr := infrapostgres.RunMigrations(rootCtx, pool); migErr != nil {
		logger.Error("migrations failed", slog.String("error", migErr.Error()))
		return migErr
	}
	logger.Info("migrations applied")

	s3Client, s3Err := infras3.NewR2Client(rootCtx, infras3.R2Settings{
		AccountID:       cfg.R2.AccountID,
		AccessKeyID:     cfg.R2.AccessKeyID,
		SecretAccessKey: cfg.R2.SecretAccessKey,
	})
	if s3Err != nil {
		logger.Error("r2 client init failed", slog.String("error", s3Err.Error()))
		return s3Err
	}

	// ─── Outbound adapters (implement inner ports) ────────────────────
	imageStore := r2.NewPresigner(r2.Config{
		Client:     s3Client,
		Bucket:     cfg.R2.Bucket,
		CDNBaseURL: cfg.R2.CDNBaseURL,
	})
	hasher := auth.NewArgon2idHasher()
	tokens := auth.NewJWTIssuer(auth.JWTConfig{
		Secret:   []byte(cfg.JWT.Secret),
		Issuer:   cfg.JWT.Issuer,
		Audience: cfg.JWT.Audience,
		TTL:      cfg.JWT.TTL,
	})
	csrfMinter := auth.NewRandomCSRFMinter()
	systemClock := clock.New()

	// ─── Repositories ────────────────────────────────────────────────
	userRepo := postgres.NewUserRepository(pool)
	requestRepo := postgres.NewBrotherbandRequestRepository(pool)
	brotherhoodRepo := postgres.NewBrotherhoodRepository(pool)
	conversationRepo := postgres.NewConversationRepository(pool)
	messageRepo := postgres.NewMessageRepository(pool)

	// ─── Use cases ───────────────────────────────────────────────────
	registerUC := usecaseuser.NewRegisterUser(userRepo, userRepo, hasher, tokens, csrfMinter, systemClock, imageStore)
	loginUC := usecaseuser.NewLoginUser(userRepo, hasher, tokens, csrfMinter, systemClock, imageStore)
	getProfileUC := usecaseuser.NewGetProfile(userRepo, imageStore)
	updateStatusUC := usecaseuser.NewUpdateStatus(userRepo)
	updateAvatarUC := usecaseuser.NewUpdateAvatar(userRepo, imageStore)

	sendBBUC := usecasebb.NewSendRequest(requestRepo, brotherhoodRepo, userRepo, systemClock)
	acceptBBUC := usecasebb.NewAcceptRequest(requestRepo, brotherhoodRepo, userRepo, imageStore, systemClock)
	denyBBUC := usecasebb.NewDenyRequest(requestRepo)
	cutBBUC := usecasebb.NewCutBrotherband(brotherhoodRepo)
	listRequestsUC := usecasebb.NewListRequests(requestRepo)
	listBrothersUC := usecasebb.NewListBrothers(brotherhoodRepo, imageStore)
	getBrotherUC := usecasebb.NewGetBrother(brotherhoodRepo, userRepo, imageStore)

	sendMsgUC := usecasemsg.NewSendMessage(conversationRepo, messageRepo, brotherhoodRepo, systemClock, imageStore)
	listMsgUC := usecasemsg.NewListMessages(conversationRepo, messageRepo, brotherhoodRepo, imageStore)
	attachUC := usecasemsg.NewAttachMedia(messageRepo, imageStore, systemClock)
	listConvUC := usecasemsg.NewListConversations(brotherhoodRepo, conversationRepo, imageStore)

	requestUploadUC := usecasemedia.NewRequestUpload(imageStore)

	// ─── HTTP handlers ───────────────────────────────────────────────
	cookies := respond.CookieConfig{Domain: cfg.HTTP.CookieDomain, Secure: cfg.HTTP.SecureCookies}
	routes := httplayer.Routes{
		Auth:        handler.NewAuthHandler(registerUC, loginUC, cookies),
		User:        handler.NewUserHandler(getProfileUC, updateStatusUC, updateAvatarUC),
		Brotherband: handler.NewBrotherbandHandler(sendBBUC, acceptBBUC, denyBBUC, cutBBUC, listRequestsUC, listBrothersUC, getBrotherUC),
		Message:     handler.NewMessageHandler(sendMsgUC, listMsgUC, attachUC, listConvUC),
		Media:       handler.NewMediaHandler(requestUploadUC),
		Health:      handler.NewHealthHandler(pool, version),
	}

	router := httplayer.NewRouter(httplayer.RouterConfig{
		Logger:         logger,
		Metrics:        metrics,
		AllowedOrigins: cfg.HTTP.AllowedOrigins,
		TokenIssuer:    tokens,
		Clock:          systemClock,
	}, routes)

	server := httplayer.NewServer(httplayer.ServerConfig{
		Addr:    cfg.HTTP.Addr,
		Handler: router,
		Logger:  logger,
	})

	// ─── Run + graceful shutdown ─────────────────────────────────────
	serverErrCh := make(chan error, 1)
	go func() {
		// The goroutine has its own recover so a panic in the serve
		// path becomes a clean shutdown instead of a process abort.
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic in http server goroutine",
					slog.Any("panic", rec),
				)
				serverErrCh <- errlike(rec)
			}
		}()
		serverErrCh <- server.ListenAndServe()
	}()

	select {
	case srvErr := <-serverErrCh:
		if srvErr != nil {
			logger.Error("http server terminated unexpectedly",
				slog.String("error", srvErr.Error()),
			)
		}
		return srvErr
	case <-rootCtx.Done():
		logger.Info("shutdown signal received; draining in-flight requests")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if shErr := server.Shutdown(shutdownCtx, 15*time.Second); shErr != nil && !errors.Is(shErr, context.Canceled) {
		logger.Error("graceful shutdown failed", slog.String("error", shErr.Error()))
		return shErr
	}
	logger.Info("server stopped cleanly")
	return nil
}

// errlike normalises a recovered panic value into an error so the
// named return of run() can carry it.
func errlike(rec any) error {
	if e, ok := rec.(error); ok {
		return fmt.Errorf("panic: %w", e)
	}
	return fmt.Errorf("panic: %v", rec)
}
