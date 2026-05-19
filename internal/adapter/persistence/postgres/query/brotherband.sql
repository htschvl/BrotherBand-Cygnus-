-- name: CreateBrotherbandRequest :one
INSERT INTO brotherband_requests (requester_id, recipient_id)
VALUES ($1, $2)
RETURNING id, requester_id, recipient_id, created_at;

-- name: GetBrotherbandRequestByID :one
SELECT id, requester_id, recipient_id, created_at
FROM brotherband_requests
WHERE id = $1;

-- name: DeleteBrotherbandRequest :exec
DELETE FROM brotherband_requests WHERE id = $1;

-- name: ListBrotherbandRequestsReceived :many
SELECT r.id, r.requester_id, r.recipient_id, r.created_at,
       u.username AS requester_username
FROM brotherband_requests r
JOIN users u ON u.id = r.requester_id
WHERE r.recipient_id = $1
ORDER BY r.created_at DESC;

-- name: ListBrotherbandRequestsSent :many
SELECT r.id, r.requester_id, r.recipient_id, r.created_at,
       u.username AS recipient_username
FROM brotherband_requests r
JOIN users u ON u.id = r.recipient_id
WHERE r.requester_id = $1
ORDER BY r.created_at DESC;

-- name: CreateBrotherhood :exec
INSERT INTO brotherhoods (user_low_id, user_high_id)
VALUES (LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid));

-- name: DeleteBrotherhood :exec
DELETE FROM brotherhoods
WHERE user_low_id = LEAST($1::uuid, $2::uuid)
  AND user_high_id = GREATEST($1::uuid, $2::uuid);

-- name: BrotherhoodExists :one
SELECT EXISTS (
  SELECT 1 FROM brotherhoods
  WHERE user_low_id = LEAST($1::uuid, $2::uuid)
    AND user_high_id = GREATEST($1::uuid, $2::uuid)
) AS exists;

-- name: ListBrothers :many
SELECT u.id, u.username, u.status, u.favorites, u.avatar_key, u.registered_at,
       b.became_brothers_at
FROM brotherhoods b
JOIN users u ON u.id = CASE
    WHEN b.user_low_id = $1 THEN b.user_high_id
    ELSE b.user_low_id
END
WHERE b.user_low_id = $1 OR b.user_high_id = $1
ORDER BY b.became_brothers_at DESC;
