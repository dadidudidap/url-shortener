# URL Shortener

A small URL shortener written in Go. Short links redirect through Postgres, reads are cached in Redis, clicks are counted by a separate background worker, and the frontend supports accounts, link creation, owner-only stats with live updates, QR codes, custom codes, and expiry.

## Features

- Create a short link from a full URL or a bare domain
- Guest links expire after 3 days; signed-in links also start with a 3-day expiry
- Owners can edit a link to a custom short code, an exact expiry, or lifetime, and can delete it
- Expired or unknown short codes redirect to the home page
- Expired guest links are deleted on visit and by a separate sweeper; expired user links stay in `Your links` and can be re-activated through edit
- Redirect with a `302` and count each visit
- Owner-only click stats, with live updates over WebSocket while the panel is open
- Live counts reach every API replica through a Redis pub/sub channel
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

The repository is a Go workspace (`go.work`). The root module holds the API, the worker, and the shared packages; the auth service is a separate module in `services/auth/`.

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

The management UI is at <http://localhost:15672> (`app` / `app`). If port 5672 is already in use, stop the existing broker or change `RABBITMQ_HOST`/`RABBITMQ_PORT`.

Edit `.env` with your Postgres user, password, database, host, and port. The app builds the connection URL from them. Then create the database and apply the schema:

```bash
createdb url_db
psql "postgres://user:password@localhost:5432/url_db?sslmode=disable" -f shared/database/schema.sql
```

Run the API, the click worker, and the sweeper in separate terminals:

```bash
go run ./services/api
```

```bash
go run ./services/worker
```

```bash
go run ./services/sweeper
```

Open <http://localhost:8000>. The worker consumes the click queue. Without it, clicks stay queued and stats stop moving; redirects and the stats endpoint still work. The auth service is a separate module; see [services/auth](services/auth). The sweeper deletes expired guest links at startup and every 30 days.

### Run everything in Docker

The Compose file starts the API, auth service, click worker, sweeper, Postgres, Redis, and RabbitMQ together. Edit `.env` first, then:

```bash
docker compose up --build
```

Open <http://localhost:8000>. Postgres is published on host port 5433 and Redis on 6380 so they do not collide with local services. The schema is applied when the database volume is first created. After changing `shared/database/schema.sql`, apply it again without losing data:

```bash
docker compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" < shared/database/schema.sql
```

Or run `docker compose down -v` and start again, which deletes every link.

## Configuration

| Variable | Required | Default | Notes |
| --- | --- | --- | --- |
| `PORT` | no | `8000` | Must be numeric |
| `POSTGRES_USER` | yes | none | Postgres user |
| `POSTGRES_PASSWORD` | yes | none | Postgres password |
| `POSTGRES_DB` | yes | none | Postgres database name |
| `DB_HOST` | yes | none | Postgres host |
| `DB_PORT` | yes | none | Postgres port. Compose publishes `5433` |
| `REDIS_HOST` | no | `localhost` | Redis host |
| `REDIS_PORT` | no | `6379` | Redis port. Compose publishes `6380` |
| `RABBITMQ_USER` | no | `guest` | RabbitMQ user; `.env.example` uses the Compose `app` user |
| `RABBITMQ_PASS` | no | `guest` | RabbitMQ password |
| `RABBITMQ_HOST` | no | `localhost` | RabbitMQ host |
| `RABBITMQ_PORT` | no | `5672` | RabbitMQ port |
| `AUTH_GRPC_HOST` | no | `localhost`, `0.0.0.0` in the auth service | Auth gRPC host |
| `AUTH_GRPC_PORT` | no | `50051` | Auth gRPC port |
| `JWT_SECRET` | yes | none | HMAC secret shared by the API and the auth service |
| `JWT_TTL` | no | `24h` | Token lifetime, any `time.ParseDuration` value |
| `SWEEP_INTERVAL` | no | `720h` | How often the sweeper runs, any `time.ParseDuration` value |
| `SWEEP_BATCH_SIZE` | no | `1000` | Expired guest rows deleted per batch |

All connection strings are built from these parts. There are no URL variables.

The app reads `.env` from the working directory when it exists. On a server, set the variables in the environment instead. The API, the auth service, the worker, and the sweeper read the same file; the worker needs Postgres, Redis, and RabbitMQ, the sweeper needs Postgres, and the auth service needs Postgres plus `JWT_SECRET`.

## API

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/register` | Create an account with email and password |
| `POST` | `/api/login` | Exchange credentials for a bearer token |
| `GET` | `/api/links` | Links owned by the caller. Requires `Authorization: Bearer <token>` |
| `PUT` | `/api/links/{code}` | Rename, re-expire, or make a link lifetime. Owner only |
| `DELETE` | `/api/links/{code}` | Permanently delete a link. Owner only |
| `POST` | `/api/shorten` | Create a short link. Rate limited. Optional bearer token sets the owner. |
| `GET` | `/api/stats/{code}` | Owner-only click stats. Requires `Authorization: Bearer <token>` |
| `GET` | `/api/qr/{code}` | 256x256 PNG QR code. Public |
| `GET` | `/api/ws/{code}` | Owner-only live click count over WebSocket |
| `GET` | `/{code}` | Redirect to the long URL, or to `/` when missing or expired |

Guests can create and use links, but they cannot open stats or the stats socket. A guest code that expires is deleted on its next visit; a signed-in code that expires is kept and marked expired in `Your links`.

Create a link. The scheme is optional and defaults to `https`:

```bash
curl -X POST http://localhost:8000/api/shorten \
  -H 'Content-Type: application/json' \
  -d '{"long_url":"example.com/a/long/path"}'
```

```json
{"expires_at":"2026-11-10T12:05:30.149735685Z","short_url":"http://localhost:8000/abc123"}
```

Fetch owner stats:

```bash
curl http://localhost:8000/api/stats/abc123 \
  -H 'Authorization: Bearer <token>'
```

```json
{"clicks":0,"created_at":"2026-10-22T14:10:06.784468Z","expired":false,"expires_at":"2026-11-10T12:05:30.149735685Z","long_url":"https://example.com/a/long/path","short_code":"abc123"}
```

Edit a link. `short_code` accepts 3-32 characters from letters, numbers, `_`, and `-`, and is case-sensitive. `api` in any case is reserved. An empty `expires_at` with `lifetime` false means 3 days from now; `lifetime` true ignores the date, even a past one. A rename takes effect immediately and the old code redirects to `/`.

```bash
curl -X PUT http://localhost:8000/api/links/abc123 \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{"short_code":"my-link","expires_at":"2026-12-01T12:00:00Z","lifetime":false}'
```

Delete a link:

```bash
curl -X DELETE http://localhost:8000/api/links/my-link \
  -H 'Authorization: Bearer <token>'
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

### Live clicks

`/api/ws/{code}` sends one JSON message per counted click:

```json
{"short_code":"abc123","clicks":7}
```

It carries updates only. Fetch `/api/stats/{code}` once for the current value, then keep the socket open. Browsers cannot set an `Authorization` header on a WebSocket, so the token travels in the `bearer` subprotocol:

```js
const socket = new WebSocket("ws://localhost:8000/api/ws/abc123", ["bearer", token]);
socket.onmessage = (event) => console.log(JSON.parse(event.data));
```

The server verifies the token and link ownership before the upgrade. The upgrade is accepted only when the `Origin` host matches the request `Host`. The server pings every 45 seconds and drops a viewer that stops answering, and a slow write is abandoned after 5 seconds so a dead tab cannot delay click counting. The bundled frontend opens the socket only while a code's stats are on screen, reconnects after 2 seconds if it drops, and closes it when the tab is hidden.

## How it works

- `POST /api/shorten` validates the URL, stores a generated 6-character code and a 3-day expiry in Postgres, and sets the owner when a valid token is sent.
- `GET /{code}` checks Redis first, falls back to Postgres on a miss, and rejects missing or expired codes with a redirect to `/`. Cache values store the long URL, owner, and expiry; the TTL is capped at 24 hours and at the time left before expiry.
- An expired guest code is deleted on visit. Expired user codes stay in the database and are shown as expired.
- The separate sweeper deletes expired guest codes in batches at startup and every 30 days. It never deletes user-owned rows.
- The click is published to the durable `clicks_queue`. The separate worker writes it to Postgres and acks the message. Publish failures are logged and do not block the redirect.
- The worker increments the count in one statement, then publishes `code:clicks` on the `ws_broadcasts` Redis channel. Each API replica subscribes to that channel and fans the value out to viewers of that short code over `/api/ws/{code}`.
- If Redis is unavailable, the rate limiter fails open and redirects still use Postgres.

## Project layout

```
services/api/           HTTP API: main.go, handlers, routes, middleware, WebSocket hub
services/api/main.go    startup, HTTP server, graceful shutdown
services/auth/          gRPC auth service, separate module (`go run ./services/auth`)
services/worker/        click consumer, run with `go run ./services/worker`
services/sweeper/       expired guest-link cleanup, run with `go run ./services/sweeper`
shared/database/        Postgres, Redis, and RabbitMQ setup, schema, queries
shared/database/db/     sqlc-generated code, do not edit by hand
shared/pb/              generated gRPC contracts (from shared/proto/auth.proto)
shared/proto/           .proto sources
src/                    index.html, styles.css, and app.js, served as static files
go.work                 workspace tying the root module and the auth module together
sqlc.yaml               sqlc configuration
```

## Development

```bash
go test ./...
(cd services/auth && go test ./...)
go vet ./...
go build -o url-shortener ./services/api
```

The API, worker, and sweeper are packages in the root module; the auth service is its own module. Build any of them from the repository root: `go build ./services/api`, `go build ./services/worker`, `go build ./services/sweeper`, `go build ./services/auth`.

The tests cover URL normalization, expiry and custom-code rules, owner-only stats, link edit/delete, QR routes, the frontend route, the sweeper, and the WebSocket hub. The hub tests open real sockets through `httptest`, so run them with the race detector after touching `services/api/ws/`:

```bash
go test -race ./services/api/ws
```

The RabbitMQ integration test skips unless `RABBITMQ_TEST_URL` is set:

```bash
RABBITMQ_TEST_URL=amqp://app:app@localhost:5672/ go test ./shared/database
```

After editing `shared/database/schema.sql` or `shared/database/queries.sql`, regenerate the data layer:

```bash
sqlc generate
```

## Notes

- Redirects wait at most 50ms for click publishing, so a stuck broker cannot hold the response. Publish failures are logged and redirects still succeed.
- The click consumer uses manual acknowledgements and processes one message at a time. Delivery is at-least-once: a crash after the Postgres update but before the ack can double count a click.
- Publisher confirms are not enabled, so a successful publish means the frame was written, not that the broker persisted it. Clicks are analytics, not billing data.
- The publisher and consumer reconnect every 5 seconds if RabbitMQ is unavailable. The worker exits at startup when Redis is unreachable.
- A failed live update is logged and the click is still acked: the stored count stays correct, only the WebSocket push is missed.
- The Compose credentials are for local development only.
- The frontend is served from `src/` relative to the working directory, so run the app from the project root.
- WebSocket upgrades accept only the same origin as the request host, so a proxy that rewrites `Host` also breaks live stats. The hub is per-process memory, but every API replica listens on the `ws_broadcasts` Redis channel, so a viewer gets updates whichever replica counted the click.
- Short links and QR codes use the request host, so run behind a proxy that preserves `Host` and sets `X-Forwarded-Proto`.
- Guests can create and follow links. Accounts unlock ownership, stats, edit/delete, and live clicks; stats and the live socket are owner-only. QR codes stay public.
- The frontend checks the JWT `exp` on load and clears an expired or malformed session. A `401` on a protected request also clears it and asks the user to log in again.
