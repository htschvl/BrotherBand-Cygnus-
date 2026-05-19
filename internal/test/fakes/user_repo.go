// Package fakes hosts hand-rolled in-memory implementations of the
// domain repository ports. They are the unit-test substitute for the
// real Postgres adapters; use cases get state-assertion ergonomics
// (after RegisterUser, can I find the user?) without the cost of
// spinning up a container.
//
// These are NOT mocks: they have real behaviour and shared state, so
// their usefulness is exactly the *behavioural* contract — not the
// "did Foo get called with X" question that mocks answer best.
package fakes

import (
	"context"
	"sync"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/user"
)

// UserRepo is a goroutine-safe in-memory user repository. It
// satisfies user.Reader, Writer, StatusUpdater and AvatarUpdater so
// it can stand in for the full Postgres repository during unit tests.
type UserRepo struct {
	mu      sync.RWMutex
	byID    map[string]user.User
	byUname map[string]string
}

// NewUserRepo returns an empty in-memory user repository.
func NewUserRepo() *UserRepo {
	return &UserRepo{byID: map[string]user.User{}, byUname: map[string]string{}}
}

// Compile-time interface checks.
var (
	_ user.Reader        = (*UserRepo)(nil)
	_ user.Writer        = (*UserRepo)(nil)
	_ user.StatusUpdater = (*UserRepo)(nil)
	_ user.AvatarUpdater = (*UserRepo)(nil)
)

func (r *UserRepo) Save(_ context.Context, u user.User) (user.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, taken := r.byUname[u.Username().String()]; taken {
		return user.User{}, user.ErrUsernameAlreadyTaken
	}
	r.byID[u.ID().String()] = u
	r.byUname[u.Username().String()] = u.ID().String()
	return u, nil
}

func (r *UserRepo) FindByID(_ context.Context, id shared.ID) (user.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byID[id.String()]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}

func (r *UserRepo) FindByUsername(_ context.Context, username user.Username) (user.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byUname[username.String()]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return r.byID[id], nil
}

func (r *UserRepo) UpdateStatus(_ context.Context, id shared.ID, status user.Status) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id.String()]
	if !ok {
		return user.ErrNotFound
	}
	r.byID[id.String()] = u.WithStatus(status)
	return nil
}

func (r *UserRepo) UpdateAvatar(_ context.Context, id shared.ID, avatarKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id.String()]
	if !ok {
		return user.ErrNotFound
	}
	r.byID[id.String()] = u.WithAvatarKey(avatarKey)
	return nil
}
