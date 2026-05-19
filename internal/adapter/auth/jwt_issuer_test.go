package auth_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/auth"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

func TestJWTIssuer_Roundtrip(t *testing.T) {
	t.Parallel()
	issuer := auth.NewJWTIssuer(auth.JWTConfig{
		Secret:   []byte(strings.Repeat("a", 32)),
		Issuer:   "brotherband-test",
		Audience: "test-clients",
		TTL:      time.Hour,
	})
	now := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	id := shared.NewID()

	tok, err := issuer.Issue(id, now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if tok.Value == "" {
		t.Fatal("issued token is empty")
	}
	if !tok.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("unexpected expiry: %v", tok.ExpiresAt)
	}

	got, err := issuer.Verify(tok.Value, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !got.Equals(id) {
		t.Fatalf("subject mismatch: %v vs %v", got, id)
	}
}

func TestJWTIssuer_RejectsExpired(t *testing.T) {
	t.Parallel()
	issuer := auth.NewJWTIssuer(auth.JWTConfig{
		Secret:   []byte(strings.Repeat("b", 32)),
		Issuer:   "brotherband-test",
		Audience: "test-clients",
		TTL:      time.Hour,
	})
	now := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	tok, err := issuer.Issue(shared.NewID(), now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	_, err = issuer.Verify(tok.Value, now.Add(2*time.Hour))
	if !errors.Is(err, shared.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated for expired token, got %v", err)
	}
}

func TestJWTIssuer_RejectsTampered(t *testing.T) {
	t.Parallel()
	issuer := auth.NewJWTIssuer(auth.JWTConfig{
		Secret:   []byte(strings.Repeat("c", 32)),
		Issuer:   "brotherband-test",
		Audience: "test-clients",
		TTL:      time.Hour,
	})
	now := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	tok, _ := issuer.Issue(shared.NewID(), now)

	tampered := tok.Value[:len(tok.Value)-2] + "xx"
	_, err := issuer.Verify(tampered, now)
	if !errors.Is(err, shared.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated for tampered signature, got %v", err)
	}
}
