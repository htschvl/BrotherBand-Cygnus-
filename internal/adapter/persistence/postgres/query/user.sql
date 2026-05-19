-- name: CreateUser :one
INSERT INTO users (username, password_hash, birthdate, secret, status, favorites)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, username, password_hash, birthdate, secret, status, favorites, avatar_key, registered_at;

-- name: GetUserByID :one
SELECT id, username, password_hash, birthdate, secret, status, favorites, avatar_key, registered_at
FROM users
WHERE id = $1;

-- name: GetUserByUsername :one
SELECT id, username, password_hash, birthdate, secret, status, favorites, avatar_key, registered_at
FROM users
WHERE username = $1;

-- name: UpdateUserStatus :exec
UPDATE users SET status = $2 WHERE id = $1;

-- name: UpdateUserAvatar :exec
UPDATE users SET avatar_key = $2 WHERE id = $1;
