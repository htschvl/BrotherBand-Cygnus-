package fakes

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/message"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// ConversationRepo is the in-memory implementation of
// message.ConversationRepository.
type ConversationRepo struct {
	mu            sync.RWMutex
	byID          map[string]message.Conversation
	participants  map[string]map[string]struct{} // conv → set of users
	lastReadByKey map[string]time.Time           // conv|user → last_read
}

// NewConversationRepo returns an empty in-memory conversation repository.
func NewConversationRepo() *ConversationRepo {
	return &ConversationRepo{
		byID:          map[string]message.Conversation{},
		participants:  map[string]map[string]struct{}{},
		lastReadByKey: map[string]time.Time{},
	}
}

var _ message.ConversationRepository = (*ConversationRepo)(nil)

func (r *ConversationRepo) Create(_ context.Context, c message.Conversation, ps []shared.ID) (message.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[c.ID().String()] = c
	set := map[string]struct{}{}
	for _, p := range ps {
		set[p.String()] = struct{}{}
	}
	r.participants[c.ID().String()] = set
	return c, nil
}

func (r *ConversationRepo) FindDirectBetween(_ context.Context, a, b shared.ID) (message.Conversation, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for cid, set := range r.participants {
		if len(set) != 2 {
			continue
		}
		if _, ok := set[a.String()]; !ok {
			continue
		}
		if _, ok := set[b.String()]; !ok {
			continue
		}
		return r.byID[cid], true, nil
	}
	return message.Conversation{}, false, nil
}

func (r *ConversationRepo) IsParticipant(_ context.Context, conv, user shared.ID) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	set, ok := r.participants[conv.String()]
	if !ok {
		return false, nil
	}
	_, ok = set[user.String()]
	return ok, nil
}

func (r *ConversationRepo) UpdateLastRead(_ context.Context, conv, user shared.ID, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastReadByKey[conv.String()+"|"+user.String()] = at
	return nil
}

// MessageRepo is the in-memory implementation of message.MessageReader
// + message.MessageWriter.
type MessageRepo struct {
	mu     sync.RWMutex
	byID   map[string]message.Message
	byConv map[string][]string // conv → ordered message IDs (newest last)
	atts   map[string][]message.Attachment
}

// NewMessageRepo returns an empty in-memory message repository.
func NewMessageRepo() *MessageRepo {
	return &MessageRepo{
		byID:   map[string]message.Message{},
		byConv: map[string][]string{},
		atts:   map[string][]message.Attachment{},
	}
}

var (
	_ message.MessageReader = (*MessageRepo)(nil)
	_ message.MessageWriter = (*MessageRepo)(nil)
)

func (r *MessageRepo) Save(_ context.Context, m message.Message) (message.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[m.ID().String()] = m
	r.byConv[m.ConversationID().String()] = append(r.byConv[m.ConversationID().String()], m.ID().String())
	return m, nil
}

func (r *MessageRepo) FindByID(_ context.Context, id shared.ID) (message.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.byID[id.String()]
	if !ok {
		return message.Message{}, message.ErrNotFound
	}
	return m.WithAttachments(r.atts[id.String()]), nil
}

func (r *MessageRepo) SaveAttachment(_ context.Context, a message.Attachment) (message.Attachment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.atts[a.MessageID().String()] = append(r.atts[a.MessageID().String()], a)
	return a, nil
}

func (r *MessageRepo) FindByConversation(
	_ context.Context,
	conversationID shared.ID,
	cursor *message.Cursor,
	limit int,
) ([]message.Message, *message.Cursor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := r.byConv[conversationID.String()]
	collected := make([]message.Message, 0, len(all))
	for _, id := range all {
		collected = append(collected, r.byID[id])
	}
	sort.Slice(collected, func(i, j int) bool {
		ci, cj := collected[i].CreatedAt(), collected[j].CreatedAt()
		if !ci.Equal(cj) {
			return ci.After(cj)
		}
		return collected[i].ID().String() > collected[j].ID().String()
	})

	if cursor != nil {
		filtered := collected[:0]
		for _, m := range collected {
			if m.CreatedAt().Before(cursor.CreatedAt) ||
				(m.CreatedAt().Equal(cursor.CreatedAt) && m.ID().String() < cursor.ID.String()) {
				filtered = append(filtered, m)
			}
		}
		collected = filtered
	}
	if len(collected) > limit {
		collected = collected[:limit]
	}

	for i := range collected {
		collected[i] = collected[i].WithAttachments(r.atts[collected[i].ID().String()])
	}

	var next *message.Cursor
	if len(collected) == limit && limit > 0 {
		last := collected[len(collected)-1]
		next = &message.Cursor{CreatedAt: last.CreatedAt(), ID: last.ID()}
	}
	return collected, next, nil
}
