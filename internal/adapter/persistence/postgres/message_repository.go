package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/message"
	"github.com/htschvl/BrotherBand-Cygnus-/internal/domain/shared"
)

// MessageRepository implements both message.MessageReader and
// message.MessageWriter. The two ports are split at the domain layer
// for ISP, but a single struct on the adapter side keeps the
// connection pool and SQL string constants in one file.
type MessageRepository struct{ db DBTX }

// NewMessageRepository accepts any DBTX (pool in prod, tx in tests).
func NewMessageRepository(db DBTX) *MessageRepository {
	return &MessageRepository{db: db}
}

var (
	_ message.MessageReader = (*MessageRepository)(nil)
	_ message.MessageWriter = (*MessageRepository)(nil)
)

func (r *MessageRepository) Save(ctx context.Context, m message.Message) (message.Message, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, sender_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, conversation_id, sender_id, body, created_at, edited_at`,
		m.ConversationID().UUID(), m.SenderID().UUID(), m.Body().String())
	saved, err := scanMessage(row)
	if err != nil {
		return message.Message{}, fmt.Errorf("postgres: save message: %w", err)
	}
	return saved, nil
}

func (r *MessageRepository) FindByID(ctx context.Context, id shared.ID) (message.Message, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, conversation_id, sender_id, body, created_at, edited_at
		FROM messages WHERE id = $1`, id.UUID())
	got, err := scanMessage(row)
	if err != nil {
		if isNoRows(err) {
			return message.Message{}, message.ErrNotFound
		}
		return message.Message{}, fmt.Errorf("postgres: find message: %w", err)
	}
	atts, err := r.attachmentsFor(ctx, []uuid.UUID{got.ID().UUID()})
	if err != nil {
		return message.Message{}, err
	}
	return got.WithAttachments(atts[got.ID().String()]), nil
}

func (r *MessageRepository) FindByConversation(
	ctx context.Context,
	conversationID shared.ID,
	cursor *message.Cursor,
	limit int,
) ([]message.Message, *message.Cursor, error) {
	var (
		cursorAt  time.Time
		cursorID  uuid.UUID
		hasCursor bool
	)
	if cursor != nil {
		cursorAt = cursor.CreatedAt
		cursorID = cursor.ID.UUID()
		hasCursor = true
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, conversation_id, sender_id, body, created_at, edited_at
		FROM messages
		WHERE conversation_id = $1
		  AND ($4::bool = false OR (created_at, id) < ($2, $3))
		ORDER BY created_at DESC, id DESC
		LIMIT $5`,
		conversationID.UUID(), cursorAt, cursorID, hasCursor, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("postgres: list messages: %w", err)
	}
	defer rows.Close()

	out := []message.Message{}
	ids := []uuid.UUID{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, m)
		ids = append(ids, m.ID().UUID())
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	atts, err := r.attachmentsFor(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	for i := range out {
		out[i] = out[i].WithAttachments(atts[out[i].ID().String()])
	}

	var next *message.Cursor
	if len(out) == limit {
		last := out[len(out)-1]
		next = &message.Cursor{CreatedAt: last.CreatedAt(), ID: last.ID()}
	}
	return out, next, nil
}

func (r *MessageRepository) SaveAttachment(ctx context.Context, a message.Attachment) (message.Attachment, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO message_attachments (message_id, media_key, content_type, size_bytes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, message_id, media_key, content_type, size_bytes, created_at`,
		a.MessageID().UUID(), a.MediaKey(), a.ContentType(), a.SizeBytes())
	var (
		id, messageID uuid.UUID
		mediaKey      string
		contentType   string
		sizeBytes     int64
		createdAt     time.Time
	)
	if err := row.Scan(&id, &messageID, &mediaKey, &contentType, &sizeBytes, &createdAt); err != nil {
		return message.Attachment{}, fmt.Errorf("postgres: save attachment: %w", err)
	}
	return message.RehydrateAttachment(
		shared.MustParseID(id.String()),
		shared.MustParseID(messageID.String()),
		mediaKey, contentType, sizeBytes, createdAt,
	), nil
}

func (r *MessageRepository) attachmentsFor(ctx context.Context, messageIDs []uuid.UUID) (map[string][]message.Attachment, error) {
	out := map[string][]message.Attachment{}
	if len(messageIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, message_id, media_key, content_type, size_bytes, created_at
		FROM message_attachments
		WHERE message_id = ANY($1::uuid[])
		ORDER BY created_at ASC`, messageIDs)
	if err != nil {
		return nil, fmt.Errorf("postgres: load attachments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id, messageID uuid.UUID
			mediaKey      string
			contentType   string
			sizeBytes     int64
			createdAt     time.Time
		)
		if err := rows.Scan(&id, &messageID, &mediaKey, &contentType, &sizeBytes, &createdAt); err != nil {
			return nil, err
		}
		att := message.RehydrateAttachment(
			shared.MustParseID(id.String()),
			shared.MustParseID(messageID.String()),
			mediaKey, contentType, sizeBytes, createdAt,
		)
		key := messageID.String()
		out[key] = append(out[key], att)
	}
	return out, rows.Err()
}

func scanMessage(row pgxRow) (message.Message, error) {
	var (
		id, conversationID, senderID uuid.UUID
		body                         string
		createdAt                    time.Time
		editedAt                     *time.Time
	)
	if err := row.Scan(&id, &conversationID, &senderID, &body, &createdAt, &editedAt); err != nil {
		return message.Message{}, err
	}
	bodyVO, err := message.NewBody(body)
	if err != nil {
		return message.Message{}, err
	}
	return message.Rehydrate(
		shared.MustParseID(id.String()),
		shared.MustParseID(conversationID.String()),
		shared.MustParseID(senderID.String()),
		bodyVO, createdAt, editedAt, nil,
	), nil
}
