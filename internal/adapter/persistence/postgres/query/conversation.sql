-- name: CreateConversation :one
INSERT INTO conversations DEFAULT VALUES
RETURNING id, created_at;

-- name: AddConversationParticipant :exec
INSERT INTO conversation_participants (conversation_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: FindDirectConversationBetween :one
SELECT cp1.conversation_id
FROM conversation_participants cp1
JOIN conversation_participants cp2
  ON cp1.conversation_id = cp2.conversation_id
WHERE cp1.user_id = $1
  AND cp2.user_id = $2
  AND (
    SELECT COUNT(*) FROM conversation_participants
    WHERE conversation_id = cp1.conversation_id
  ) = 2
LIMIT 1;

-- name: IsConversationParticipant :one
SELECT EXISTS (
  SELECT 1 FROM conversation_participants
  WHERE conversation_id = $1 AND user_id = $2
) AS is_participant;

-- name: UpdateLastReadAt :exec
UPDATE conversation_participants
SET last_read_at = $3
WHERE conversation_id = $1 AND user_id = $2;
