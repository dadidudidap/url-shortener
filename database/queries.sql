-- name: CreateURL :exec
INSERT INTO urls (short_code, long_url) VALUES ($1, $2);

-- name: GetURL :one
SELECT long_url FROM urls WHERE short_code = $1;

-- name: IncrementClicks :one
UPDATE urls SET clicks = clicks + 1 WHERE short_code = $1 RETURNING clicks;

-- name: GetURLStats :one
SELECT short_code, long_url, clicks, created_at FROM urls WHERE short_code = $1;