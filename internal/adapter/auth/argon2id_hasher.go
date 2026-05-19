// Package auth implements the password-hash and token-issuer ports.
// Both implementations are stateless and trivially testable.
//
// The argon2id parameters follow OWASP's 2024 recommendations for
// the "interactive" tier (m=19 MiB, t=2, p=1). Tune via benchmark on
// the production host (see argon2id_hasher_test.go).
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/platform/logging"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/usecase/port"
)

const (
	argonTime    uint32 = 2
	argonMemory  uint32 = 19 * 1024 // 19 MiB
	argonThreads uint8  = 1
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

// Argon2idHasher implements port.PasswordHasher.
type Argon2idHasher struct{}

// NewArgon2idHasher constructs the hasher.
func NewArgon2idHasher() *Argon2idHasher { return &Argon2idHasher{} }

// Compile-time interface check.
var _ port.PasswordHasher = (*Argon2idHasher)(nil)

// Hash returns the encoded `$argon2id$...` string suitable for storage.
// Salt and parameters are embedded; no separate columns are needed.
func (h *Argon2idHasher) Hash(plain string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2id: read salt: %w", err)
	}
	digest := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest),
	), nil
}

// Verify recomputes the hash with the parameters embedded in `encoded`
// and constant-time compares the digest. A mismatch — including any
// parsing failure — surfaces as ErrInvalidCredentials so callers do
// not have to translate per-cause.
func (h *Argon2idHasher) Verify(encoded, plain string) error {
	params, salt, hash, err := parseEncoded(encoded)
	if err != nil {
		// A malformed *stored* hash is data corruption, not a user
		// typo. The caller (login use case) cannot distinguish the two
		// — Verify intentionally collapses both to ErrInvalidCredentials
		// so the wire response never leaks which it was. We still emit
		// a data-integrity alarm here so operators can react. This is a
		// leaf adapter with no request context, hence slog.Default().
		slog.Default().Warn("argon2id: stored hash is malformed (data integrity)",
			slog.String(logging.AttrComponent, "adapter.auth.argon2id"),
			slog.String(logging.AttrError, err.Error()),
		)
		return user.ErrInvalidCredentials
	}
	candidate := argon2.IDKey([]byte(plain), salt, params.time, params.memory, params.threads, uint32(len(hash)))
	if subtle.ConstantTimeCompare(candidate, hash) != 1 {
		return user.ErrInvalidCredentials
	}
	return nil
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

// parseEncoded inverts the format produced by Hash. Any structural
// deviation returns errMalformedHash; callers map that to
// ErrInvalidCredentials so storage corruption does not leak as a
// distinguishable signal.
func parseEncoded(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// Expected layout: ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return argonParams{}, nil, nil, errMalformedHash
	}
	p, err := parseParamSegment(parts[3])
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, errMalformedHash
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, errMalformedHash
	}
	return p, salt, hash, nil
}

func parseParamSegment(seg string) (argonParams, error) {
	var memory, t uint64
	var threads uint64
	for _, kv := range strings.Split(seg, ",") {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			return argonParams{}, errMalformedHash
		}
		key, val := kv[:eq], kv[eq+1:]
		n, err := strconv.ParseUint(val, 10, 32)
		if err != nil {
			return argonParams{}, errMalformedHash
		}
		switch key {
		case "m":
			memory = n
		case "t":
			t = n
		case "p":
			threads = n
		default:
			return argonParams{}, errMalformedHash
		}
	}
	if memory == 0 || t == 0 || threads == 0 {
		return argonParams{}, errMalformedHash
	}
	return argonParams{memory: uint32(memory), time: uint32(t), threads: uint8(threads)}, nil
}

var errMalformedHash = errors.New("argon2id: malformed encoded hash")
