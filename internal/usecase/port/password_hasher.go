// Package port declares the cross-cutting interfaces that the
// application layer (use cases) consumes from the outside world.
//
// The dependency inversion principle in action: the inner layer
// owns the interface, the outer layer (`adapter/auth`,
// `platform/clock`, …) implements it. Use cases import port; nothing
// imports back.
package port

// PasswordHasher is the port for password hashing. The argon2id
// adapter implements it. Use cases must not depend on a specific
// algorithm.
type PasswordHasher interface {
	// Hash produces an encoded hash string (algorithm + params + salt
	// + digest in a single string) from the plaintext password.
	Hash(plain string) (string, error)
	// Verify returns nil iff the plaintext matches the encoded hash.
	// Implementations must use a constant-time comparison.
	Verify(encoded, plain string) error
}
