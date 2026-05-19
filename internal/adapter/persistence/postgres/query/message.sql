-- name: InsertMessage :one
INSERT INTO messages (conversation_id, sender_id, body)
VALUES ($1, $2, $3)
RETURNING id, conversation_id, sender_id, body, metadata, created_at, edited_at;

-- name: GetMessageByID :one
SELECT id, conversation_id, sender_id, body, metadata, created_at, edited_at
FROM messages
WHERE id = $1;

-- name: ListMessagesPage :many
SELECT id, conversation_id, sender_id, body, metadata, created_at, edited_at
FROM messages
WHERE conversation_id = $1
  AND (
    sqlc.narg('cursor_created_at')::timestamptz IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamptz, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: InsertMessageAttachment :one
INSERT INTO message_attachments (message_id, media_key, content_type, size_bytes)
VALUES ($1, $2, $3, $4)
RETURNING id, message_id, media_key, content_type, size_bytes, created_at;

-- name: ListAttachmentsForMessages :many
SELECT id, message_id, media_key, content_type, size_bytes, created_at
FROM message_attachments
WHERE message_id = ANY(@message_ids::uuid[])
ORDER BY created_at ASC;
