// Package fixtures hosts builder-style helpers that absorb
// constructor changes. If the User entity later gains a parameter,
// the test files don't change — the builder does.
package fixtures

import (
	"testing"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
)

// UserBuilder constructs `user.User` values in tests without forcing
// every test to spell out every value object.
type UserBuilder struct {
	username  string
	password  string
	birthdate string
	secret    string
	status    string
	favorites []string
	now       time.Time
}

// NewUser returns a builder with workable defaults.
func NewUser() *UserBuilder {
	return &UserBuilder{
		username:  "test_user",
		password:  "Hunter2!Hunter2!",
		birthdate: "1995-04-12",
		secret:    "lobster reflects on the moon",
		status:    "Quietly online",
		favorites: []string{"matcha", "bach", "rain", "longboards", "campfires"},
		now:       time.Now().UTC(),
	}
}

func (b *UserBuilder) WithUsername(v string) *UserBuilder    { b.username = v; return b }
func (b *UserBuilder) WithPassword(v string) *UserBuilder    { b.password = v; return b }
func (b *UserBuilder) WithBirthdate(v string) *UserBuilder   { b.birthdate = v; return b }
func (b *UserBuilder) WithSecret(v string) *UserBuilder      { b.secret = v; return b }
func (b *UserBuilder) WithStatus(v string) *UserBuilder      { b.status = v; return b }
func (b *UserBuilder) WithFavorites(v []string) *UserBuilder { b.favorites = v; return b }
func (b *UserBuilder) At(t time.Time) *UserBuilder           { b.now = t; return b }

// Build constructs a user.User. The password hash is a deterministic
// fake string ("hash:<password>") so unit tests don't pay the
// argon2id cost; flows that test verification should use a real
// hasher.
func (b *UserBuilder) Build(t *testing.T) user.User {
	t.Helper()
	un, err := user.NewUsername(b.username)
	if err != nil {
		t.Fatalf("fixtures: bad username: %v", err)
	}
	ph, err := user.NewPasswordHash("hash:" + b.password)
	if err != nil {
		t.Fatalf("fixtures: bad password hash: %v", err)
	}
	bd, err := user.NewBirthdate(b.birthdate)
	if err != nil {
		t.Fatalf("fixtures: bad birthdate: %v", err)
	}
	sc, err := user.NewSecret(b.secret)
	if err != nil {
		t.Fatalf("fixtures: bad secret: %v", err)
	}
	st, err := user.NewStatus(b.status)
	if err != nil {
		t.Fatalf("fixtures: bad status: %v", err)
	}
	favs, err := user.NewFavorites(b.favorites)
	if err != nil {
		t.Fatalf("fixtures: bad favorites: %v", err)
	}
	return user.New(un, ph, bd, sc, st, favs, b.now)
}
