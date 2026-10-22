# URL Shortener

A small URL shortener written in Go. Short links redirect through Postgres, reads are cached in Redis, clicks are counted in the background, and the frontend (including QR codes) is embedded in the binary.

## Features

- Create a short link from a full URL or a bare domain
- Redirect with a `302` and count each visit
- Click stats per short code
- PNG QR code for every short link
- Redis cache in front of Postgres
- Rate limit of 5 shorten requests per minute per IP
- Single-page frontend with light and dark themes, no build step
- Graceful shutdown on `SIGINT` / `SIGTERM`

## Requirements

- Go 1.27 or newer
- PostgreSQL
- Redis
- [sqlc](https://sqlc.dev) only if you change the schema or queries

## Setup

```bash
git clone https://github.com/dadidudidap/url-shortener.git
cd url-shortener

cp .env.example .env
```

Edit `.env` so `DATABASE_URL` points at your Postgres instance. Then create the database and apply the schema:

```bash
createdb url_db
psql "postgres://user:password@localhost:5432/url_db?sslmode=disable" -f database/schema.sql
```

Run the app:

```bash
go run .
```

Open <http://localhost:8000>.

## Configuration

| Variable | Required | Default | Notes |
| --- | --- | --- | --- |
| `PORT` | no | `8000` | Must be numeric |
| `DATABASE_URL` | yes | none | Postgres connection string used by `pgxpool` |
| `REDIS_URL` | no | `localhost:6379` | Redis address as `host:port`, not a URL |

The app reads `.env` from the working directory when it exists. On a server, set the variables in the environment instead.

## API

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/shorten` | Create a short link. Rate limited. |
| `GET` | `/api/stats/{code}` | Click stats for a short code |
| `GET` | `/api/qr/{code}` | 256x256 PNG QR code |
| `GET` | `/{code}` | Redirect to the long URL |

Create a link. The scheme is optional and defaults to `https`:

```bash
curl -X POST http://localhost:8000/api/shorten \
  -H 'Content-Type: application/json' \
  -d '{"long_url":"example.com/a/long/path"}'
```

```json
{"short_url":"http://localhost:8000/abc123"}
```

Fetch stats:

```bash
curl http://localhost:8000/api/stats/abc123
```

```json
{"clicks":0,"created_at":"2026-10-22T14:10:06.784468Z","long_url":"https://example.com/a/long/path","short_code":"abc123"}
```

Download the QR code:

```bash
curl -o qr.png http://localhost:8000/api/qr/abc123
```

Follow a redirect:

```bash
curl -i http://localhost:8000/abc123
```

Only `http` and `https` URLs are accepted, and the normalized URL is limited to 2048 characters. A missing scheme becomes `https://`. When the limit is exceeded on `/api/shorten`, the API returns `429` and asks the caller to wait a minute.

## How it works

- `POST /api/shorten` validates the URL and stores a generated 6-character code in Postgres.
- `GET /{code}` checks Redis first, falls back to Postgres on a miss, then caches the result for 24 hours.
- The click is pushed onto a buffered channel and incremented by a background worker, so the redirect does not wait on analytics.
- If Redis is unavailable, the rate limiter fails open and redirects still use Postgres.

## Project layout

```
main.go                 startup, HTTP server, graceful shutdown
routes/routes.go        route wiring
handlers/               shorten, redirect, stats, QR, click worker
middleware/             Redis rate limiter
web/                    embedded frontend
database/               Postgres and Redis setup, schema, queries
database/db/            sqlc-generated code, do not edit by hand
sqlc.yaml               sqlc configuration
```

## Development

```bash
go test ./...
go vet ./...
go build -o url-shortener .
```

The tests cover URL normalization, the QR route, and the embedded page handler.

After editing `database/schema.sql` or `database/queries.sql`, regenerate the data layer:

```bash
sqlc generate
```

## Notes

- Click counts are queued in memory (buffer of 1000). A crash can lose clicks that have not been written yet. If the buffer is full, the click is dropped and logged rather than slowing the redirect.
- Short links and QR codes use the request host, so run behind a proxy that preserves `Host` and sets `X-Forwarded-Proto`.
- There is no authentication. Anyone who can reach the API can create links.
