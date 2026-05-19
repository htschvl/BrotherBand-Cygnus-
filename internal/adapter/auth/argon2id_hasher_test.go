package auth_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/auth"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
)

func TestArgon2idHasher_Roundtrip(t *testing.T) {
	t.Parallel()
	h := auth.NewArgon2idHasher()
	encoded, err := h.Hash("Hunter2!Hunter2")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$") {
		t.Fatalf("encoded hash has wrong prefix: %q", encoded)
	}
	if err := h.Verify(encoded, "Hunter2!Hunter2"); err != nil {
		t.Fatalf("verify success path failed: %v", err)
	}
	if err := h.Verify(encoded, "wrong-password"); !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials on wrong password, got %v", err)
	}
}

// BenchmarkArgon2idHash is the on-host tuning hook from the
// architecture doc. Run `go test -bench=. -benchtime=3s ./internal/adapter/auth/...`
// and aim for ~100–300 ms/op on production hardware.
func BenchmarkArgon2idHash(b *testing.B) {
	h := auth.NewArgon2idHasher()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Hash("Hunter2!Hunter2")
	}
}
