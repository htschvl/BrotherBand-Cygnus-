package user_test

import (
	"errors"
	"testing"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
)

func TestNewUsername(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"alphanumeric", "alice99", true},
		{"with_underscore", "alice_bob", true},
		{"with_dash", "alice-bob", true},
		{"too_short", "ab", false},
		{"too_long", "this_username_is_far_too_long_to_be_accepted_by_the_validator", false},
		{"with_space", "alice bob", false},
		{"empty", "", false},
		{"whitespace_trimmed_to_too_short", "  a  ", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := user.NewUsername(tc.in)
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok && !errors.Is(err, user.ErrInvalidUsername) {
				t.Fatalf("expected ErrInvalidUsername, got %v", err)
			}
		})
	}
}

func TestNewFavorites_RequiresExactlyFive(t *testing.T) {
	t.Parallel()
	for _, count := range []int{0, 1, 4, 6, 10} {
		count := count
		t.Run("count_"+string(rune('0'+count)), func(t *testing.T) {
			t.Parallel()
			vals := make([]string, count)
			for i := range vals {
				vals[i] = "x"
			}
			_, err := user.NewFavorites(vals)
			if !errors.Is(err, user.ErrInvalidFavorites) {
				t.Fatalf("count=%d: expected ErrInvalidFavorites, got %v", count, err)
			}
		})
	}
	if _, err := user.NewFavorites([]string{"a", "b", "c", "d", "e"}); err != nil {
		t.Fatalf("five non-empty values must succeed, got %v", err)
	}
}

func TestValidateRawPassword(t *testing.T) {
	t.Parallel()
	if err := user.ValidateRawPassword("short"); !errors.Is(err, user.ErrPasswordTooWeak) {
		t.Fatalf("expected ErrPasswordTooWeak, got %v", err)
	}
	if err := user.ValidateRawPassword("Hunter2!Hunter2"); err != nil {
		t.Fatalf("good password rejected: %v", err)
	}
}

// Sanity: domain errors classify under shared sentinels so the HTTP
// error map can fall back to category if a specific case is missed.
func TestUserErrors_WrapSharedSentinels(t *testing.T) {
	t.Parallel()
	checks := map[error]error{
		user.ErrUsernameAlreadyTaken: shared.ErrConflict,
		user.ErrInvalidCredentials:   shared.ErrUnauthenticated,
		user.ErrPasswordTooWeak:      shared.ErrInvalidInput,
		user.ErrNotFound:             shared.ErrNotFound,
	}
	for specific, category := range checks {
		if !errors.Is(specific, category) {
			t.Errorf("%v does not wrap %v", specific, category)
		}
	}
}
