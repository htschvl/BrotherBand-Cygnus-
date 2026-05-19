package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

// JWTIssuer implements port.TokenIssuer using HS256-signed JWTs.
//
// The token is the only piece of authentication state the server
// keeps. There is no DB-backed session table, no refresh rotation,
// no revocation list. Trade-offs are documented in
// docs/brotherband-cygnus-doc-architecture.md §5.
type JWTIssuer struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
}

// JWTConfig groups the constructor parameters so additions later do
// not produce a five-argument constructor.
type JWTConfig struct {
	Secret   []byte
	Issuer   string
	Audience string
	TTL      time.Duration
}

// NewJWTIssuer constructs the issuer. A panicking constructor is
// fine here because misconfiguration at boot is unrecoverable.
func NewJWTIssuer(cfg JWTConfig) *JWTIssuer {
	if len(cfg.Secret) < 32 {
		panic("jwt: secret must be at least 32 bytes")
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 30 * 24 * time.Hour
	}
	return &JWTIssuer{
		secret:   cfg.Secret,
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		ttl:      cfg.TTL,
	}
}

// Compile-time interface check.
var _ port.TokenIssuer = (*JWTIssuer)(nil)

// Issue mints a new HS256 token whose `sub` claim is the user ID.
func (j *JWTIssuer) Issue(userID shared.ID, now time.Time) (port.IssuedToken, error) {
	expiresAt := now.Add(j.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		Issuer:    j.issuer,
		Audience:  jwt.ClaimStrings{j.audience},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(j.secret)
	if err != nil {
		return port.IssuedToken{}, fmt.Errorf("jwt: sign: %w", err)
	}
	return port.IssuedToken{Value: signed, ExpiresAt: expiresAt}, nil
}

// Verify parses and validates the token. The `now` argument is
// supplied so the test clock can rule on expiry deterministically.
func (j *JWTIssuer) Verify(raw string, now time.Time) (shared.ID, error) {
	parsed, err := jwt.ParseWithClaims(
		raw,
		&jwt.RegisteredClaims{},
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("jwt: unexpected signing method: %v", token.Header["alg"])
			}
			return j.secret, nil
		},
		jwt.WithIssuer(j.issuer),
		jwt.WithAudience(j.audience),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		return shared.ID{}, errors.Join(shared.ErrUnauthenticated, err)
	}
	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || !parsed.Valid {
		return shared.ID{}, shared.ErrUnauthenticated
	}
	id, err := shared.ParseID(claims.Subject)
	if err != nil {
		return shared.ID{}, shared.ErrUnauthenticated
	}
	return id, nil
}
