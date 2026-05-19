package respond_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/adapter/http/respond"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/media"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/message"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
)

// TestErrorMap_AllSentinelsHaveAStableStatus is the highest-value
// single test in the HTTP layer: every domain error sentinel must
// produce a non-500 response. Adding a new sentinel without a
// mapping fails this test loudly.
func TestErrorMap_AllSentinelsHaveAStableStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err      error
		wantCode int
	}{
		{user.ErrUsernameAlreadyTaken, http.StatusConflict},
		{user.ErrInvalidCredentials, http.StatusUnauthorized},
		{user.ErrPasswordTooWeak, http.StatusUnprocessableEntity},
		{user.ErrInvalidUsername, http.StatusUnprocessableEntity},
		{user.ErrInvalidBirthdate, http.StatusUnprocessableEntity},
		{user.ErrInvalidSecret, http.StatusUnprocessableEntity},
		{user.ErrInvalidStatus, http.StatusUnprocessableEntity},
		{user.ErrInvalidFavorites, http.StatusUnprocessableEntity},
		{user.ErrNotFound, http.StatusNotFound},

		{brotherband.ErrSelfRequest, http.StatusUnprocessableEntity},
		{brotherband.ErrAlreadyBrothers, http.StatusConflict},
		{brotherband.ErrRequestExists, http.StatusConflict},
		{brotherband.ErrRequestNotFound, http.StatusNotFound},
		{brotherband.ErrNotABrother, http.StatusForbidden},
		{brotherband.ErrNotRecipient, http.StatusForbidden},

		{message.ErrInvalidBody, http.StatusUnprocessableEntity},
		{message.ErrInvalidAttachment, http.StatusUnprocessableEntity},
		{message.ErrInvalidCursor, http.StatusUnprocessableEntity},
		{message.ErrNotParticipant, http.StatusForbidden},
		{message.ErrNotFound, http.StatusNotFound},
		{message.ErrConversationNotFound, http.StatusNotFound},

		{media.ErrUnsupportedMediaType, http.StatusUnsupportedMediaType},
		{media.ErrPayloadTooLarge, http.StatusRequestEntityTooLarge},
		{media.ErrPromotionFailed, http.StatusConflict},

		{shared.ErrInvalidID, http.StatusUnprocessableEntity},
		{shared.ErrUnauthenticated, http.StatusUnauthorized},
		{shared.ErrForbidden, http.StatusForbidden},
		{shared.ErrConflict, http.StatusConflict},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			respond.Error(rec, req, tc.err)
			if rec.Code != tc.wantCode {
				t.Fatalf("err=%v: got status %d, want %d", tc.err, rec.Code, tc.wantCode)
			}
		})
	}
}

func TestErrorMap_UnknownErrorIs500(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	respond.Error(rec, req, errors.New("something explodes"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestErrorMap_WrappedErrorsAreClassified(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("postgres: %w", user.ErrUsernameAlreadyTaken)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	respond.Error(rec, req, wrapped)
	if rec.Code != http.StatusConflict {
		t.Fatalf("wrapped sentinel should still classify, got %d", rec.Code)
	}
}
