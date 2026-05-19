// Package config loads and validates configuration from the
// process environment. Failure at boot is preferable to a partial
// startup, so every required setting is verified up front.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the validated, immutable configuration the rest of the
// application depends on.
type Config struct {
	Env  Env
	HTTP HTTPConfig
	DB   DatabaseConfig
	JWT  JWTConfig
	R2   R2Config
}

// Env classifies the runtime environment for log and cookie behaviour.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvStaging     Env = "staging"
	EnvProduction  Env = "production"
)

func (e Env) IsProduction() bool { return e == EnvProduction }

// HTTPConfig holds the HTTP server + cookie + CORS settings.
type HTTPConfig struct {
	Addr           string
	AllowedOrigins []string
	CookieDomain   string
	SecureCookies  bool
}

// DatabaseConfig holds the Postgres connection settings.
type DatabaseConfig struct {
	DSN string
}

// JWTConfig holds the session-token signing settings.
type JWTConfig struct {
	Secret   string
	Issuer   string
	Audience string
	TTL      time.Duration
}

// R2Config holds the Cloudflare R2 credentials and bucket/CDN settings.
type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	CDNBaseURL      string
}

// Load reads from os.Getenv and validates each field. The boot
// sequence is the only place this is ever called.
func Load() (Config, error) {
	env := Env(getEnvOr("APP_ENV", string(EnvDevelopment)))
	jwtTTLHours, err := strconv.Atoi(getEnvOr("JWT_TTL_HOURS", "720")) // 30 days
	if err != nil {
		return Config{}, fmt.Errorf("config: JWT_TTL_HOURS: %w", err)
	}

	cfg := Config{
		Env: env,
		HTTP: HTTPConfig{
			Addr:           getEnvOr("HTTP_ADDR", ":8080"),
			AllowedOrigins: splitCSV(getEnvOr("HTTP_ALLOWED_ORIGINS", "http://localhost:5173")),
			CookieDomain:   os.Getenv("HTTP_COOKIE_DOMAIN"),
			SecureCookies:  env.IsProduction(),
		},
		DB: DatabaseConfig{
			DSN: os.Getenv("DATABASE_URL"),
		},
		JWT: JWTConfig{
			Secret:   os.Getenv("JWT_SECRET"),
			Issuer:   getEnvOr("JWT_ISSUER", "brotherband"),
			Audience: getEnvOr("JWT_AUDIENCE", "brotherband-web"),
			TTL:      time.Duration(jwtTTLHours) * time.Hour,
		},
		R2: R2Config{
			AccountID:       os.Getenv("R2_ACCOUNT_ID"),
			AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
			Bucket:          os.Getenv("R2_BUCKET"),
			CDNBaseURL:      os.Getenv("R2_CDN_BASE_URL"),
		},
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	var errs []error
	if c.DB.DSN == "" {
		errs = append(errs, errors.New("config: DATABASE_URL is required"))
	}
	if len(c.JWT.Secret) < 32 {
		errs = append(errs, errors.New("config: JWT_SECRET must be at least 32 characters"))
	}
	if c.R2.Bucket == "" || c.R2.AccountID == "" {
		errs = append(errs, errors.New("config: R2 settings are required (R2_ACCOUNT_ID, R2_BUCKET, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_CDN_BASE_URL)"))
	}
	if c.R2.CDNBaseURL == "" {
		errs = append(errs, errors.New("config: R2_CDN_BASE_URL is required"))
	}
	return errors.Join(errs...)
}

func getEnvOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
