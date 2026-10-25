# URL Shortener

A small URL shortener written in Go. Short links redirect through Postgres, reads are cached in Redis, clicks are counted in the background, and the frontend supports link creation, click stats, and QR codes.

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
- RabbitMQ (Docker Compose file included)
- Docker Compose (optional, for the full stack)
- [sqlc](https://sqlc.dev) only if you change the schema or queries

## Setup

```bash
git clone https://github.com/dadidudidap/url-shortener.git
cd url-shortener

cp .env.example .env
```

Start RabbitMQ only:

```bash
docker compose up -d rabbitmq
```

The management UI is at <http://localhost:15672> (`app` / `app`). If port 5672 is already in use, stop the existing broker or point `RABBITMQ_URL` at it.

Edit `.env` with your Postgres user, password, database, host, and port. The app builds the connection URL from them. Then create the database and apply the schema:

```bash
createdb url_db
psql "postgres://user:password@localhost:5432/url_db?sslmode=disable" -f database/schema.sql
```

Run the app:

```bash
go run .
```

Open <http://localhost:8000>.

### Run everything in Docker

The Compose file starts the API, Postgres, Redis, and RabbitMQ together. Edit `.env` first, then:

```bash
docker compose up --build
```

Open <http://localhost:8000>. Postgres is published on host port 5433 and Redis on 6380 so they do not collide with local services. The schema is applied when the database volume is first created; after changing `database/schema.sql`, run `docker compose down -v` and start again.

## Configuration

| Variable | Required | Default | Notes |
| --- | --- | --- | --- |
| `PORT` | no | `8000` | Must be numeric |
| `DATABASE_URL` | no | built from the parts below | Full connection string; wins over the parts |
| `POSTGRES_USER` | yes, unless `DATABASE_URL` is set | none | Postgres user |
| `POSTGRES_PASSWORD` | yes, unless `DATABASE_URL` is set | none | Postgres password |
| `POSTGRES_DB` | yes, unless `DATABASE_URL` is set | none | Postgres database name |
| `DB_HOST` | yes, unless `DATABASE_URL` is set | none | Postgres host |
| `DB_PORT` | yes, unless `DATABASE_URL` is set | none | Postgres port |
| `REDIS_URL` | no | `localhost:6379` | Redis address as `host:port`, not a URL |
| `RABBITMQ_URL` | no | `amqp://guest:guest@localhost:5672/` | AMQP URL; `.env.example` uses the Compose `app` user |

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
- The click is published to the durable `clicks_queue`. A background consumer writes it to Postgres and acks the message. Publish failures are logged and do not block the redirect.
- If Redis is unavailable, the rate limiter fails open and redirects still use Postgres.

## Project layout

```
main.go                 startup, HTTP server, graceful shutdown
routes/routes.go        route wiring
handlers/               shorten, redirect, stats, QR, click consumer
middleware/             Redis rate limiting
src/                    index.html, styles.css, and app.js, served as static files
database/               Postgres, Redis, and RabbitMQ setup, schema, queries
database/db/            sqlc-generated code, do not edit by hand
sqlc.yaml               sqlc configuration
```

## Development

```bash
go test ./...
go vet ./...
go build -o url-shortener .
```

The tests cover URL normalization, the QR route, and the frontend route. The RabbitMQ integration test skips unless `RABBITMQ_URL` is set:

```bash
RABBITMQ_URL=amqp://app:app@localhost:5672/ go test ./database
```

After editing `database/schema.sql` or `database/queries.sql`, regenerate the data layer:

```bash
sqlc generate
```

## Notes

- Redirects wait at most 50ms for click publishing, so a stuck broker cannot hold the response. Publish failures are logged and redirects still succeed.
- The click consumer uses manual acknowledgements and processes one message at a time. Delivery is at-least-once: a crash after the Postgres update but before the ack can double count a click.
- Publisher confirms are not enabled, so a successful publish means the frame was written, not that the broker persisted it. Clicks are analytics, not billing data.
- The publisher and consumer reconnect every 5 seconds if RabbitMQ is unavailable.
- The Compose credentials are for local development only.
- The frontend is served from `src/` relative to the working directory, so run the app from the project root.
- Short links and QR codes use the request host, so run behind a proxy that preserves `Host` and sets `X-Forwarded-Proto`.
- There is no authentication. Anyone who can reach the API can create links.
