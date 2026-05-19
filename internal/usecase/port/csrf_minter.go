package port

// CSRFMinter produces a random, opaque value that the HTTP adapter
// stores in the readable `bb_csrf` cookie and that the client echoes
// back as the `X-CSRF-Token` header on state-changing requests.
//
// It lives in `usecase/port` (rather than the HTTP adapter) because
// the registration and login use cases mint a CSRF token alongside
// the session token, so the contract is shared.
type CSRFMinter interface {
	Mint() (string, error)
}
