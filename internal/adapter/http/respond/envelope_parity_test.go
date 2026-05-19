package respond_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/middleware"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/respond"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// The middleware package re-declares the error envelope (it cannot
// import respond — respond imports middleware for the cookie/CSRF
// constants, which would be a cycle). This test is the contract that
// keeps the two shapes byte-identical: if respond.ErrorBody ever
// gains/renames a field without the middleware writer following, the
// JSON key sets diverge and this fails loudly.
func TestErrorEnvelope_MiddlewareAndRespondAgreeOnShape(t *testing.T) {
	t.Parallel()

	// respond.Error output for a plain (no-details) error — run
	// behind RequestID so it carries a requestId, exactly like the
	// middleware path below. Comparing one envelope with a request id
	// against one without would be a false "drift".
	rrRespond := httptest.NewRecorder()
	middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond.Error(w, r, shared.ErrUnauthenticated)
	})).ServeHTTP(rrRespond, httptest.NewRequest(http.MethodGet, "/", nil))

	// middleware writer output, via the real CSRF middleware (which
	// now routes through the shared writeError). RequestID runs first
	// so both envelopes carry a requestId.
	chain := middleware.RequestID(middleware.CSRF(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) { t.Fatal("must not reach handler") })))
	rrMW := httptest.NewRecorder()
	chain.ServeHTTP(rrMW, httptest.NewRequest(http.MethodPost, "/", nil)) // no CSRF → 403

	keys := func(raw []byte) []string {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("not JSON: %s", raw)
		}
		ks := make([]string, 0, len(m))
		for k := range m {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		return ks
	}

	respKeys := keys(rrRespond.Body.Bytes())
	mwKeys := keys(rrMW.Body.Bytes())

	if len(respKeys) != len(mwKeys) {
		t.Fatalf("envelope drift: respond keys %v vs middleware keys %v", respKeys, mwKeys)
	}
	for i := range respKeys {
		if respKeys[i] != mwKeys[i] {
			t.Fatalf("envelope drift at %d: respond=%v middleware=%v", i, respKeys, mwKeys)
		}
	}
	// Lock the expected contract explicitly too.
	want := []string{"code", "message", "requestId"}
	for i, k := range want {
		if i >= len(respKeys) || respKeys[i] != k {
			t.Fatalf("expected envelope keys %v, got %v", want, respKeys)
		}
	}
}
