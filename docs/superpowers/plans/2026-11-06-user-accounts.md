# User Accounts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add email/password accounts owned by the auth service, with JWT bearer tokens verified locally by the API.

**Architecture:** The auth gRPC service owns the `users` table, hashes passwords with bcrypt, and signs HS256 JWTs. The API never touches `users`: it forwards register/login over gRPC, verifies tokens locally in middleware, and stores `user_id` on links when a token is present.

**Tech Stack:** Go 1.27, pgx/v5, sqlc, chi, gRPC, `github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`.

**Spec:** `docs/superpowers/specs/2026-11-05-user-accounts-design.md`

## Global Constraints

- `JWT_SECRET` is required by both services; the process must refuse to start without it.
- `JWT_TTL` defaults to `24h`, parsed with `time.ParseDuration`.
- Token transport is `Authorization: Bearer <token>` only. No cookies.
- Password rules: minimum 8 characters, bcrypt default cost.
- Email: trimmed and lowercased before every read or write.
- `AUTH_API_KEY` and the `ValidateToken` RPC are removed.
- Anonymous shortening stays allowed. Invalid Bearer token → 401, never silent anonymous.
- Public routes stay public: redirect, stats, QR, WebSocket.
- Every task leaves `go build ./...` and `go test ./...` green. Task order is chosen for that: the protobuf swap lands with the auth service that implements it, and the middleware change lands with the route wiring that consumes it.

## Review Focus

- Unknown email and wrong password must return the same 401 body, or the API leaks account existence. Pinned in Task 2.
- Expired token must be rejected by both `Auth` and `OptionalAuth`. Pinned in Task 3.
- Token signed with a different secret, or `alg: none`, must be rejected. Pinned in Tasks 1 and 3.
- Auth service unreachable during login must yield a 5xx, not a panic or an empty token. Pinned in Task 3.
- A user must never read another user's links. Pinned in Task 5.

---

### Task 1: Shared token package

**Files:**
- Create: `shared/token/token.go`
- Test: `shared/token/token_test.go`
- Modify: `go.mod` (add `github.com/golang-jwt/jwt/v5`)

**Interfaces:**
- Produces: `token.Claims{UserID int32; Email string}`, `token.Sign(secret string, claims Claims, ttl time.Duration) (string, error)`, `token.Verify(secret, raw string) (Claims, error)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestSignVerifyRoundTrip(t *testing.T)      // Sign then Verify returns the same UserID and Email
func TestVerifyRejectsWrongSecret(t *testing.T) // Verify with another secret returns an error
func TestVerifyRejectsExpired(t *testing.T)     // Sign with ttl = -time.Minute, Verify returns an error
func TestVerifyRejectsAlgNone(t *testing.T)     // hand-built {"alg":"none"} token returns an error
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./shared/token/`
Expected: FAIL, package `token` has no `Sign`/`Verify`.

- [ ] **Step 3: Implement with HS256**

`Sign` uses `jwt.NewWithClaims(jwt.SigningMethodHS256, ...)` with claims `sub` (`strconv.Itoa(claims.UserID)`), `email`, `iat`, `exp`. `Verify` passes `jwt.WithValidMethods([]string{"HS256"})` and rejects a missing or non-integer `sub`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./shared/token/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add shared/token go.mod go.sum
git commit -m "feat(auth): add shared jwt sign and verify helpers"
```

---

### Task 2: Proto swap and auth service logic

**Files:**
- Modify: `shared/proto/auth.proto`
- Regenerate: `shared/pb/auth.pb.go`, `shared/pb/auth_grpc.pb.go`
- Create: `services/auth/server.go`
- Modify: `services/auth/main.go`
- Test: `services/auth/server_test.go`
- Modify: `services/auth/go.mod`

**Interfaces:**
- Consumes: `token.Sign`, `db.Queries.CreateUser`, `db.Queries.GetUserByEmail`.
- Produces: `newServer(queries *db.Queries, secret string, ttl time.Duration) *server` with `Register`/`Login`; `pb.RegisterRequest`, `pb.LoginRequest`, `pb.UserResponse`, `pb.LoginResponse`, `pb.AuthServiceClient.Register`, `pb.AuthServiceClient.Login`.

- [ ] **Step 1: Replace the proto file**

```proto
syntax = "proto3";

option go_package = "github.com/dadidudidap/url-shortener/shared/pb;pb";

package auth;

service AuthService {
  rpc Register (RegisterRequest) returns (UserResponse);
  rpc Login    (LoginRequest)    returns (LoginResponse);
}

message RegisterRequest {
  string email    = 1;
  string password = 2;
}

message LoginRequest {
  string email    = 1;
  string password = 2;
}

message UserResponse {
  int32  id    = 1;
  string email = 2;
}

message LoginResponse {
  string       token = 1;
  UserResponse user  = 2;
}
```

- [ ] **Step 2: Install the generators matching the checked-in headers**

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
```

- [ ] **Step 3: Regenerate into `shared/pb`**

```bash
PATH="$PATH:$(go env GOPATH)/bin" protoc \
  --go_out=. --go_opt=module=github.com/dadidudidap/url-shortener \
  --go-grpc_out=. --go-grpc_opt=module=github.com/dadidudidap/url-shortener \
  shared/proto/auth.proto
```

Expected: both files rewritten; `TokenRequest`/`TokenResponse` gone.

- [ ] **Step 4: Write the failing tests**

```go
func TestNormalizeEmail(t *testing.T)         // " User@Example.COM " -> "user@example.com"
func TestValidateCredentials(t *testing.T)    // empty email or 5-char password -> error; valid pair -> nil
func TestIsDuplicateKey(t *testing.T)         // pgconn.PgError{Code:"23505"} -> true; other code -> false
```

- [ ] **Step 5: Run tests to verify they fail**

Run: `cd services/auth && go test ./...`
Expected: FAIL, helpers undefined.

- [ ] **Step 6: Implement `server.go`**

`Register` = normalize, validate, `bcrypt.GenerateFromPassword`, `queries.CreateUser`; `isDuplicateKey` → `codes.AlreadyExists` "email already registered"; other DB error → `codes.Internal`.

`Login` = normalize, `queries.GetUserByEmail`; `pgx.ErrNoRows` and bcrypt mismatch both return the identical `codes.Unauthenticated, "invalid email or password"`; success returns `Sign` output plus `&pb.UserResponse{Id: user.ID, Email: user.Email}`.

- [ ] **Step 7: Wire `main.go`**

`database.Init()`, read `JWT_SECRET` (`log.Fatal` when empty), parse `JWT_TTL` (default `24h`), `db.New(database.Pool)`, `pb.RegisterAuthServiceServer(grpcServer, newServer(...))`.

- [ ] **Step 8: Verify**

```bash
cd services/auth && go build ./... && go test ./...
```

Expected: both green.

- [ ] **Step 9: Commit**

```bash
git add shared/proto shared/pb services/auth
git commit -m "feat(auth): register and login users with bcrypt and jwt"
```

---

### Task 3: API auth surface (middleware, register, login)

**Files:**
- Modify: `services/api/middleware/auth.go`
- Create: `services/api/middleware/auth_test.go`
- Modify: `services/api/handlers/auth.go`
- Modify: `services/api/routes/routes.go`
- Modify: `services/api/main.go`
- Test: `services/api/routes/routes_test.go`

**Interfaces:**
- Consumes: `token.Verify`, `pb.AuthServiceClient`.
- Produces: `handlers.RegisterHandler(client pb.AuthServiceClient) http.HandlerFunc`, `handlers.LoginHandler(client pb.AuthServiceClient) http.HandlerFunc`, `handlers.UserHandler(w, r)`, `routes.RegisterRoutes(authClient pb.AuthServiceClient, jwtSecret string) *chi.Mux`.

- [ ] **Step 1: Write the middleware tests**

```go
func TestAuthRejectsMissingHeader(t *testing.T)     // 401
func TestAuthRejectsGarbageToken(t *testing.T)      // "Bearer nope" -> 401
func TestAuthAcceptsValidToken(t *testing.T)        // signed token -> next called, UserID returns id
func TestAuthRejectsExpiredToken(t *testing.T)      // ttl -time.Minute -> 401
func TestAuthRejectsForeignSecret(t *testing.T)     // signed with other secret -> 401
func TestOptionalAuthAllowsAnonymous(t *testing.T)  // no header -> next called, UserID ok=false
func TestOptionalAuthRejectsBadToken(t *testing.T)  // "Bearer nope" -> 401
```

- [ ] **Step 2: Write the failing handler tests**

Extend the existing `fakeAuth` with `Register`/`Login` returning fixed values, then:

```go
func TestRegisterEndpoint(t *testing.T)              // POST {"email","password"} -> 201, body has email, no password_hash
func TestRegisterEndpointRejectsBadJSON(t *testing.T) // -> 400
func TestLoginEndpoint(t *testing.T)                 // -> 200, body token == "test-token"
func TestLoginEndpointGRPCError(t *testing.T)        // fake returns error -> 502
```

- [ ] **Step 3: Implement middleware**

Header parsing helper: `strings.CutPrefix(h, "Bearer ")`, then `strings.TrimSpace`; empty → treated as missing. Store `int32` under `userIDKey`. `Auth` returns 401 when the header is missing or the token is invalid. `OptionalAuth` returns 401 only when a header is present and invalid. Remove the `pb` import; `UserID(ctx) (int32, bool)` replaces the string version.

- [ ] **Step 4: Run middleware tests**

Run: `go test ./services/api/middleware/`
Expected: PASS.

- [ ] **Step 5: Implement handlers and routes**

Inside `/api`:

```go
r.Post("/register", handlers.RegisterHandler(authClient))
r.Post("/login", handlers.LoginHandler(authClient))
r.With(middleware.Auth(jwtSecret)).Get("/protected", handlers.UserHandler)
```

Register maps `codes.AlreadyExists` → 409, `codes.InvalidArgument` → 400, other → 502. Login maps any gRPC error → 502. `UserHandler` prints the id from `middleware.UserID`; it replaces the old `AuthHandler` body in the same file.

- [ ] **Step 6: Read `JWT_SECRET` in `services/api/main.go`**

`log.Fatal` when empty, then `routes.RegisterRoutes(authClient, jwtSecret)`.

- [ ] **Step 7: Update existing route tests**

Every `RegisterRoutes(nil)` becomes `RegisterRoutes(nil, "test-secret")`.

- [ ] **Step 8: Run tests to verify they pass**

Run: `go build ./... && go test ./services/api/...`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add services/api/middleware services/api/handlers/auth.go services/api/routes services/api/main.go
git commit -m "feat(api): add jwt middleware and account endpoints"
```

---

### Task 4: Short links carry an owner

**Files:**
- Modify: `services/api/handlers/shorten.go`
- Modify: `services/api/handlers/shorten_test.go`
- Modify: `services/api/routes/routes.go`

**Interfaces:**
- Consumes: `middleware.UserID`, `middleware.OptionalAuth`, `db.CreateURLParams{ShortCode, LongUrl, UserID pgtype.Int4}`.
- Produces: `createUniqueCode(insert func(code string) (db.Url, error)) (string, error)` — retries while `errors.Is(err, pgx.ErrNoRows)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestCreateUniqueCodeRetriesOnConflict(t *testing.T) // first two calls return pgx.ErrNoRows, third returns a row
func TestCreateUniqueCodeStopsOnOtherError(t *testing.T) // returns the error after one call
func TestCreateUniqueCodeGivesUp(t *testing.T)           // always ErrNoRows -> errCodeExhausted after maxCodeAttempts
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./services/api/handlers/`
Expected: FAIL, signature mismatch.

- [ ] **Step 3: Implement**

`ShortenHandler` reads `id, ok := middleware.UserID(r.Context())`, sets `UserID: pgtype.Int4{Int32: id, Valid: ok}`, and passes a closure over `queries.CreateURL`. Route becomes:

```go
r.With(middleware.RateLimit, middleware.OptionalAuth(jwtSecret)).Post("/shorten", handlers.ShortenHandler)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./services/api/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/handlers/shorten.go services/api/handlers/shorten_test.go services/api/routes/routes.go
git commit -m "feat(api): attach user id to new links, retry on code conflict"
```

---

### Task 5: List the caller's links

**Files:**
- Create: `services/api/handlers/links.go`
- Modify: `services/api/routes/routes.go`

**Interfaces:**
- Consumes: `middleware.UserID`, `db.Queries.GetURLsByUser`.
- Produces: `handlers.ListLinksHandler(w http.ResponseWriter, r *http.Request)`.

- [ ] **Step 1: Implement**

`GET /api/links`, guarded by `middleware.Auth`. Reads the id from context (500 if absent — the guard is broken), calls `GetURLsByUser(ctx, id)`, returns `{"links":[...]}` with `short_code`, `long_url`, `clicks`, `created_at`. The id always comes from context; there is no client-supplied user id.

- [ ] **Step 2: Verify build and existing tests**

Run: `go build ./... && go test ./services/api/...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add services/api/handlers/links.go services/api/routes/routes.go
git commit -m "feat(api): list links owned by the caller"
```

---

### Task 6: Configuration and docs

**Files:**
- Modify: `.env`, `.env.example`, `docker-compose.yml`, `README.md`

- [ ] **Step 1: Environment**

Add to `.env` and `.env.example`:

```dotenv
JWT_SECRET=change-me
JWT_TTL=24h
```

Remove `AUTH_API_KEY`.

- [ ] **Step 2: Compose**

Give `api` and `auth` `JWT_SECRET` and `JWT_TTL`. Give `auth` the Postgres vars (`DB_HOST=postgres`, `DB_PORT=5432`, plus the `POSTGRES_*` values already flowing through `env_file`) and `depends_on: postgres: condition: service_healthy`.

- [ ] **Step 3: README**

Config table: drop `AUTH_API_KEY`, add `JWT_SECRET` and `JWT_TTL`. Endpoint table: add `POST /api/register`, `POST /api/login`, `GET /api/links`. Replace the `/api/protected` example's `X-API-Key` with `Authorization: Bearer <token>`.

- [ ] **Step 4: Verify**

Run: `go build ./... && go vet ./... && go test ./... && docker compose config -q`
Expected: all silent; tests PASS.

- [ ] **Step 5: Commit**

```bash
git add .env.example docker-compose.yml README.md
git commit -m "chore: add jwt config and document account endpoints"
```

---

### Task 7: End-to-end check

**Files:** none.

- [ ] **Step 1: Start the stack**

```bash
docker compose up -d --build
```

- [ ] **Step 2: Exercise the flows**

```bash
curl -s -X POST localhost:8000/api/register -d '{"email":"a@b.com","password":"secret123"}'
TOKEN=$(curl -s -X POST localhost:8000/api/login -d '{"email":"a@b.com","password":"secret123"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
curl -s -X POST localhost:8000/api/shorten -H "Authorization: Bearer $TOKEN" -d '{"long_url":"example.com/owned"}'
curl -s localhost:8000/api/links -H "Authorization: Bearer $TOKEN"
curl -s -o /dev/null -w "%{http_code}\n" localhost:8000/api/links
curl -s -o /dev/null -w "%{http_code}\n" -X POST localhost:8000/api/shorten -H "Authorization: Bearer nope" -d '{"long_url":"example.com"}'
```

Expected: 201, a token, a `short_url`, the link listed under `/api/links`, `401` with no token, `401` with a bad token.

- [ ] **Step 3: Tear down**

```bash
docker compose down
```

- [ ] **Step 4: Commit any fixes discovered**

```bash
git commit -am "fix: <what the end-to-end run exposed>"
```
