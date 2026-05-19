package middleware_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/middleware"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/infrastructure/observability"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/clock"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/test/fakes"
)

// okHandler is the terminal handler used to confirm the chain ran.
func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func bodyCode(t *testing.T, raw []byte) string {
	t.Helper()
	var b struct {
		Code      string `json:"code"`
		RequestID string `json:"requestId"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("response body is not JSON: %q (%v)", raw, err)
	}
	return b.Code
}

func bodyRequestID(t *testing.T, raw []byte) string {
	t.Helper()
	var b struct {
		RequestID string `json:"requestId"`
	}
	_ = json.Unmarshal(raw, &b)
	return b.RequestID
}

// ─── RequestID ───────────────────────────────────────────────────────

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	t.Parallel()
	var seen string
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = middleware.RequestIDFromContext(r.Context())
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if seen == "" {
		t.Fatal("a request id must be put in the context")
	}
	if rec.Header().Get(middleware.RequestIDHeader) != seen {
		t.Fatal("the same id must be echoed in the response header")
	}
}

func TestRequestID_PropagatesInbound(t *testing.T) {
	t.Parallel()
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if middleware.RequestIDFromContext(r.Context()) != "client-123" {
			t.Errorf("inbound id not propagated: %q", middleware.RequestIDFromContext(r.Context()))
		}
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(middleware.RequestIDHeader, "client-123")
	h.ServeHTTP(httptest.NewRecorder(), req)
}

// ─── Logger ──────────────────────────────────────────────────────────

func TestLogger_BindsRequestIDTaggedLoggerToContext(t *testing.T) {
	t.Parallel()
	cap := logging.NewCapture(slog.LevelDebug)

	// RequestID must run first so the Logger middleware can tag with it.
	chain := middleware.RequestID(middleware.Logger(cap.Logger())(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			logging.FromContext(r.Context()).Info("inside handler")
			w.WriteHeader(200)
		})))
	req := httptest.NewRequest("GET", "/thing", nil)
	req.Header.Set(middleware.RequestIDHeader, "rid-9")
	chain.ServeHTTP(httptest.NewRecorder(), req)

	rec, ok := cap.FindByMessage("inside handler")
	if !ok {
		t.Fatal("handler log line not captured")
	}
	if rec.Attrs[logging.AttrRequestID] != "rid-9" {
		t.Fatalf("logger not tagged with request id: %#v", rec.Attrs)
	}
	if rec.Attrs[logging.AttrMethod] != "GET" {
		t.Fatalf("logger not tagged with method: %#v", rec.Attrs)
	}
}

// ─── AccessLog ───────────────────────────────────────────────────────

func TestAccessLog_LevelTracksStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
		want   slog.Level
	}{
		{"success_is_info", 200, slog.LevelInfo},
		{"client_error_is_warn", 404, slog.LevelWarn},
		{"server_error_is_error", 500, slog.LevelError},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cap := logging.NewCapture(slog.LevelDebug)
			h := middleware.RequestID(middleware.AccessLog(cap.Logger())(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status) })))
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))

			rec, ok := cap.FindByMessage("http request")
			if !ok {
				t.Fatal("no access-log line emitted")
			}
			if rec.Level != tc.want {
				t.Fatalf("status %d: got level %v want %v", tc.status, rec.Level, tc.want)
			}
			if rec.Attrs[logging.AttrStatus] != int64(tc.status) {
				t.Fatalf("status attr wrong: %#v", rec.Attrs[logging.AttrStatus])
			}
		})
	}
}

// ─── Recover ─────────────────────────────────────────────────────────

func TestRecover_TurnsPanicInto500WithRequestID(t *testing.T) {
	t.Parallel()
	cap := logging.NewCapture(slog.LevelDebug)
	chain := middleware.RequestID(middleware.Logger(cap.Logger())(middleware.Recover(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) { panic("boom") }))))

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic must yield 500, got %d", rec.Code)
	}
	if bodyCode(t, rec.Body.Bytes()) != "internal_error" {
		t.Fatalf("unexpected error body: %s", rec.Body.String())
	}
	if bodyRequestID(t, rec.Body.Bytes()) == "" {
		t.Fatal("panic response must include the request id for correlation")
	}
	if r, ok := cap.FindByMessage("handler panic"); !ok || r.Level != slog.LevelError {
		t.Fatalf("panic must be logged at ERROR with stack, got %+v ok=%v", r, ok)
	}
}

// ─── CORS ────────────────────────────────────────────────────────────

func TestCORS_AllowsConfiguredOriginOnly(t *testing.T) {
	t.Parallel()
	h := middleware.CORS([]string{"https://app.example"})(http.HandlerFunc(okHandler))

	allowed := httptest.NewRecorder()
	r1 := httptest.NewRequest("GET", "/", nil)
	r1.Header.Set("Origin", "https://app.example")
	h.ServeHTTP(allowed, r1)
	if allowed.Header().Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatal("allowed origin must be echoed")
	}
	if allowed.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("credentials must be allowed for the cookie auth model")
	}

	denied := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("Origin", "https://evil.example")
	h.ServeHTTP(denied, r2)
	if denied.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("non-allowlisted origin must NOT get CORS headers")
	}
}

func TestCORS_PreflightShortCircuits(t *testing.T) {
	t.Parallel()
	called := false
	h := middleware.CORS([]string{"https://app.example"})(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) { called = true }))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://app.example")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS preflight must be 204, got %d", rec.Code)
	}
	if called {
		t.Fatal("preflight must not reach the downstream handler")
	}
}

// ─── CSRF ────────────────────────────────────────────────────────────

func TestCSRF_SafeMethodsBypass(t *testing.T) {
	t.Parallel()
	h := middleware.CSRF(http.HandlerFunc(okHandler))
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(m, "/", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s must bypass CSRF, got %d", m, rec.Code)
		}
	}
}

func TestCSRF_StateChangingRequiresMatchingDoubleSubmit(t *testing.T) {
	t.Parallel()
	cap := logging.NewCapture(slog.LevelDebug)
	build := func() http.Handler {
		return middleware.RequestID(middleware.Logger(cap.Logger())(middleware.CSRF(http.HandlerFunc(okHandler))))
	}

	cases := []struct {
		name   string
		cookie string
		header string
		pass   bool
	}{
		{"match", "tok", "tok", true},
		{"mismatch", "tok", "different", false},
		{"missing_header", "tok", "", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.AddCookie(&http.Cookie{Name: middleware.CookieCSRF, Value: tc.cookie})
			if tc.header != "" {
				req.Header.Set(middleware.HeaderCSRF, tc.header)
			}
			build().ServeHTTP(rec, req)
			if tc.pass {
				if rec.Code != http.StatusOK {
					t.Fatalf("matching token must pass, got %d", rec.Code)
				}
				return
			}
			if rec.Code != http.StatusForbidden {
				t.Fatalf("bad token must be 403, got %d", rec.Code)
			}
			if bodyCode(t, rec.Body.Bytes()) != "csrf.mismatch" {
				t.Fatalf("unexpected body: %s", rec.Body.String())
			}
			if bodyRequestID(t, rec.Body.Bytes()) == "" {
				t.Fatal("csrf rejection must carry the request id")
			}
		})
	}
}

func TestCSRF_MissingCookieRejected(t *testing.T) {
	t.Parallel()
	h := middleware.CSRF(http.HandlerFunc(okHandler))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(middleware.HeaderCSRF, "x")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("missing cookie must be 403, got %d", rec.Code)
	}
}

// ─── Auth ────────────────────────────────────────────────────────────

func TestAuth_RejectsMissingAndBadCookies(t *testing.T) {
	t.Parallel()
	tokens := fakes.NewTokenIssuer()
	mw := middleware.Auth(tokens, clock.System{})

	t.Run("no_cookie", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/me", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("no cookie must be 401, got %d", rec.Code)
		}
	})
	t.Run("bad_token", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/v1/me", nil)
		req.AddCookie(&http.Cookie{Name: middleware.CookieSession, Value: "garbage"})
		mw(http.HandlerFunc(okHandler)).ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("bad token must be 401, got %d", rec.Code)
		}
	})
}

func TestAuth_ValidTokenInjectsUserAndTagsLogger(t *testing.T) {
	t.Parallel()
	tokens := fakes.NewTokenIssuer()
	cap := logging.NewCapture(slog.LevelDebug)

	userID, err := mintFor(tokens)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	chain := middleware.RequestID(middleware.Logger(cap.Logger())(
		middleware.Auth(tokens, clock.System{})(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if middleware.UserIDFromContext(r.Context()) != userID {
					t.Error("user id not threaded into context")
				}
				logging.FromContext(r.Context()).Info("authed work")
				w.WriteHeader(200)
			}))))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: middleware.CookieSession, Value: "tok-" + userID.String()})
	chain.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("valid token must pass, got %d", rec.Code)
	}
	r, ok := cap.FindByMessage("authed work")
	if !ok || r.Attrs[logging.AttrUserID] != userID.String() {
		t.Fatalf("authed logs must carry user_id, got %+v ok=%v", r, ok)
	}
}

// ─── RateLimit ───────────────────────────────────────────────────────

func TestRateLimit_AllowsThenBlocks(t *testing.T) {
	t.Parallel()
	cap := logging.NewCapture(slog.LevelDebug)
	// burst of exactly 1, no refill within the test window.
	limiter := rate.NewLimiter(rate.Every(time.Hour), 1)
	chain := middleware.RequestID(middleware.Logger(cap.Logger())(
		middleware.RateLimit(limiter, "test")(http.HandlerFunc(okHandler))))

	first := httptest.NewRecorder()
	chain.ServeHTTP(first, httptest.NewRequest("POST", "/v1/media/upload-url", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first request must pass the burst, got %d", first.Code)
	}

	second := httptest.NewRecorder()
	chain.ServeHTTP(second, httptest.NewRequest("POST", "/v1/media/upload-url", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request must be rate limited, got %d", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("429 must carry Retry-After")
	}
	if bodyCode(t, second.Body.Bytes()) != "rate_limited" {
		t.Fatalf("unexpected body: %s", second.Body.String())
	}
	if _, ok := cap.FindByMessage("rate limited"); !ok {
		t.Fatal("a rate-limited request must be logged")
	}
}

// ─── Metrics ─────────────────────────────────────────────────────────

func TestMetrics_RecordsRequest(t *testing.T) {
	t.Parallel()
	m := observability.NewMetrics()
	h := middleware.Metrics(m)(http.HandlerFunc(okHandler))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/healthz", nil))

	// The collector should now have at least one observation; scraping
	// the registry must not error and must contain our metric name.
	if got := countSeries(t, m); got == 0 {
		t.Fatal("expected http_requests_total to have at least one series")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────

func mintFor(ti *fakes.TokenIssuer) (shared.ID, error) {
	id := shared.NewID()
	_, err := ti.Issue(id, time.Now())
	return id, err
}

func countSeries(t *testing.T, m *observability.Metrics) int {
	t.Helper()
	fams, err := m.Registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	n := 0
	for _, f := range fams {
		if f.GetName() == "http_requests_total" {
			n += len(f.GetMetric())
		}
	}
	return n
}
