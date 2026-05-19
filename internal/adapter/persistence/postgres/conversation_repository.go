package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/message"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// ConversationRepository implements message.ConversationRepository.
type ConversationRepository struct{ db DBTX }

// NewConversationRepository accepts any DBTX (pool in prod, tx in tests).
func NewConversationRepository(db DBTX) *ConversationRepository {
	return &ConversationRepository{db: db}
}

var _ message.ConversationRepository = (*ConversationRepository)(nil)

// Create inserts a fresh conversation and the participant rows in a
// single round trip. The use case is responsible for finding an
// existing direct conversation first; this method assumes none
// exists.
func (r *ConversationRepository) Create(ctx context.Context, c message.Conversation, participants []shared.ID) (message.Conversation, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO conversations DEFAULT VALUES
		RETURNING id, created_at`)
	var (
		id        uuid.UUID
		createdAt time.Time
	)
	if err := row.Scan(&id, &createdAt); err != nil {
		return message.Conversation{}, fmt.Errorf("postgres: create conversation: %w", err)
	}
	for _, p := range participants {
		_, err := r.db.Exec(ctx, `
			INSERT INTO conversation_participants (conversation_id, user_id)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			id, p.UUID())
		if err != nil {
			return message.Conversation{}, fmt.Errorf("postgres: add participant: %w", err)
		}
	}
	return message.RehydrateConversation(shared.MustParseID(id.String()), createdAt), nil
}

// FindDirectBetween locates the conversation that has exactly the
// two given users as participants. Returns (zero, false, nil) if no
// such conversation exists.
func (r *ConversationRepository) FindDirectBetween(ctx context.Context, a, b shared.ID) (message.Conversation, bool, error) {
	row := r.db.QueryRow(ctx, `
		SELECT c.id, c.created_at
		FROM conversations c
		JOIN conversation_participants cp1 ON cp1.conversation_id = c.id AND cp1.user_id = $1
		JOIN conversation_participants cp2 ON cp2.conversation_id = c.id AND cp2.user_id = $2
		WHERE (
		  SELECT COUNT(*) FROM conversation_participants WHERE conversation_id = c.id
		) = 2
		LIMIT 1`,
		a.UUID(), b.UUID())
	var (
		id        uuid.UUID
		createdAt time.Time
	)
	if err := row.Scan(&id, &createdAt); err != nil {
		if isNoRows(err) {
			return message.Conversation{}, false, nil
		}
		return message.Conversation{}, false, fmt.Errorf("postgres: find direct conversation: %w", err)
	}
	return message.RehydrateConversation(shared.MustParseID(id.String()), createdAt), true, nil
}

func (r *ConversationRepository) IsParticipant(ctx context.Context, conversationID, userID shared.ID) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM conversation_participants
		  WHERE conversation_id = $1 AND user_id = $2
		)`, conversationID.UUID(), userID.UUID()).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("postgres: is participant: %w", err)
	}
	return ok, nil
}

func (r *ConversationRepository) UpdateLastRead(ctx context.Context, conversationID, userID shared.ID, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE conversation_participants
		SET last_read_at = $3
		WHERE conversation_id = $1 AND user_id = $2`,
		conversationID.UUID(), userID.UUID(), at)
	if err != nil {
		return fmt.Errorf("postgres: update last read: %w", err)
	}
	return nil
}
