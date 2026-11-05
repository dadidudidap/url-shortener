-- name: CreateURL :one
INSERT INTO urls (short_code, long_url, user_id) 
VALUES ($1, $2, $3) 
ON CONFLICT (short_code) DO NOTHING
RETURNING *;

-- name: GetURL :one
SELECT long_url FROM urls WHERE short_code = $1;

-- name: GetURLsByUser :many
SELECT short_code, long_url, clicks, created_at 
FROM urls 
WHERE user_id = $1 
ORDER BY created_at DESC;

-- name: IncrementClicks :one
UPDATE urls SET clicks = clicks + 1 WHERE short_code = $1 RETURNING clicks;

-- name: GetURLStats :one
SELECT short_code, long_url, clicks, created_at FROM urls WHERE short_code = $1;

-- name: CreateUser :one
INSERT INTO users (email, password_hash) 
VALUES ($1, $2) 
RETURNING id, email, created_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, created_at 
FROM users 
WHERE email = $1;