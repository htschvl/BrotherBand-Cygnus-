package fakes

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/brotherband"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// RequestRepo is the in-memory implementation of
// brotherband.RequestRepository.
type RequestRepo struct {
	mu           sync.RWMutex
	byID         map[string]brotherband.Request
	usernameByID func(shared.ID) string // optional; set to enrich list outputs
}

// NewRequestRepo returns an empty in-memory request repository.
func NewRequestRepo() *RequestRepo {
	return &RequestRepo{byID: map[string]brotherband.Request{}}
}

// SetUsernameLookup attaches an optional resolver so list outputs
// include usernames. Real Postgres joins do this in SQL.
func (r *RequestRepo) SetUsernameLookup(fn func(shared.ID) string) { r.usernameByID = fn }

var _ brotherband.RequestRepository = (*RequestRepo)(nil)

func (r *RequestRepo) Save(_ context.Context, req brotherband.Request) (brotherband.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.byID {
		if existing.RequesterID().Equals(req.RequesterID()) && existing.RecipientID().Equals(req.RecipientID()) {
			return brotherband.Request{}, brotherband.ErrRequestExists
		}
	}
	r.byID[req.ID().String()] = req
	return req, nil
}

func (r *RequestRepo) FindByID(_ context.Context, id shared.ID) (brotherband.Request, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	req, ok := r.byID[id.String()]
	if !ok {
		return brotherband.Request{}, brotherband.ErrRequestNotFound
	}
	return req, nil
}

func (r *RequestRepo) Delete(_ context.Context, id shared.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id.String()]; !ok {
		return brotherband.ErrRequestNotFound
	}
	delete(r.byID, id.String())
	return nil
}

func (r *RequestRepo) ListReceived(_ context.Context, userID shared.ID) ([]brotherband.ReceivedRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []brotherband.ReceivedRequest{}
	for _, req := range r.byID {
		if req.RecipientID().Equals(userID) {
			out = append(out, brotherband.ReceivedRequest{Request: req, RequesterUsername: r.lookup(req.RequesterID())})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt().After(out[j].CreatedAt()) })
	return out, nil
}

func (r *RequestRepo) ListSent(_ context.Context, userID shared.ID) ([]brotherband.SentRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []brotherband.SentRequest{}
	for _, req := range r.byID {
		if req.RequesterID().Equals(userID) {
			out = append(out, brotherband.SentRequest{Request: req, RecipientUsername: r.lookup(req.RecipientID())})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt().After(out[j].CreatedAt()) })
	return out, nil
}

func (r *RequestRepo) lookup(id shared.ID) string {
	if r.usernameByID == nil {
		return ""
	}
	return r.usernameByID(id)
}

// BrotherhoodRepo is the in-memory brotherhood repository. Pairs are
// canonicalised on every operation so it behaves like the real one.
type BrotherhoodRepo struct {
	mu     sync.RWMutex
	bonds  map[string]brotherband.Brotherhood // key = canonical(low,high)
	now    func() time.Time
	lookup func(shared.ID) brotherband.Brother // optional, for ListBrothers
}

// NewBrotherhoodRepo returns an empty in-memory brotherhood repository.
func NewBrotherhoodRepo() *BrotherhoodRepo {
	return &BrotherhoodRepo{bonds: map[string]brotherband.Brotherhood{}, now: time.Now}
}

func (r *BrotherhoodRepo) SetUserLookup(fn func(shared.ID) brotherband.Brother) { r.lookup = fn }

var _ brotherband.BrotherhoodRepository = (*BrotherhoodRepo)(nil)

func canonicalKey(a, b shared.ID) string {
	x, y := a.String(), b.String()
	if y < x {
		x, y = y, x
	}
	return x + "|" + y
}

func (r *BrotherhoodRepo) Save(_ context.Context, b brotherband.Brotherhood) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bonds[canonicalKey(b.UserA(), b.UserB())] = b
	return nil
}

func (r *BrotherhoodRepo) Delete(_ context.Context, a, b shared.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := canonicalKey(a, b)
	if _, ok := r.bonds[key]; !ok {
		return brotherband.ErrNotABrother
	}
	delete(r.bonds, key)
	return nil
}

func (r *BrotherhoodRepo) Exists(_ context.Context, a, b shared.ID) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.bonds[canonicalKey(a, b)]
	return ok, nil
}

func (r *BrotherhoodRepo) ListBrothers(_ context.Context, userID shared.ID) ([]brotherband.Brother, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []brotherband.Brother{}
	for _, bond := range r.bonds {
		if !bond.Includes(userID) {
			continue
		}
		other := bond.Other(userID)
		if r.lookup != nil {
			b := r.lookup(other)
			b.BecameBrothersAt = bond.BecameBrothersAt()
			out = append(out, b)
			continue
		}
		out = append(out, brotherband.Brother{ID: other, BecameBrothersAt: bond.BecameBrothersAt()})
	}
	return out, nil
}
