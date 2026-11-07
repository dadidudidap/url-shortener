-- name: CreateURL :one
INSERT INTO urls (short_code, long_url, user_id, expires_at) 
VALUES ($1, $2, $3, $4) 
ON CONFLICT (short_code) DO NOTHING
RETURNING *;

-- name: GetURL :one
SELECT long_url FROM urls WHERE short_code = $1;

-- name: GetURLForRedirect :one
SELECT short_code, long_url, user_id, expires_at
FROM urls
WHERE short_code = $1;

-- name: GetURLForOwner :one
SELECT short_code, long_url, clicks, user_id, created_at, expires_at
FROM urls
WHERE short_code = $1 AND user_id = $2;

-- name: GetURLsByUser :many
SELECT short_code, long_url, clicks, created_at, expires_at
FROM urls 
WHERE user_id = $1 
ORDER BY created_at DESC;

-- name: IncrementClicks :one
UPDATE urls SET clicks = clicks + 1 WHERE short_code = $1 RETURNING clicks;

-- name: GetURLStats :one
SELECT short_code, long_url, clicks, created_at FROM urls WHERE short_code = $1;

-- name: UpdateURL :one
UPDATE urls SET short_code = $1, expires_at = $2
WHERE short_code = $3 AND user_id = $4
RETURNING short_code, long_url, clicks, user_id, created_at, expires_at;

-- name: DeleteURL :one
DELETE FROM urls WHERE short_code = $1 AND user_id = $2 RETURNING short_code;

-- name: DeleteExpiredGuestURL :one
DELETE FROM urls WHERE short_code = $1 AND user_id IS NULL RETURNING short_code;

-- name: DeleteExpiredGuestURLs :many
DELETE FROM urls
WHERE short_code IN (
    SELECT short_code FROM urls
    WHERE user_id IS NULL AND expires_at IS NOT NULL AND expires_at <= now()
    LIMIT $1
)
RETURNING short_code;

-- name: CreateUser :one
INSERT INTO users (email, password_hash) 
VALUES ($1, $2) 
RETURNING id, email, created_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, created_at 
FROM users 
WHERE email = $1;