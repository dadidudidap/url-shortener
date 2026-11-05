# User accounts: design

Date: 2026-11-05
Status: awaiting approval

## Intent

Add real user accounts so links can belong to a person. Auth service owns
identity; the API stays stateless about who a user is.

## Why

`users` and `urls.user_id` already exist in the schema, and `auth/` exists but
only answers a hardcoded API-key check. Two partial auth stories cannot both
survive. Accounts give the link table an owner and make `GetURLsByUser` useful.

## Goals

- Register and log in with email + password.
- Password stored as bcrypt hash, never returned by any API.
- JWT issued on login, verified locally by the API on each request.
- Short links optionally owned by a user; `GET /api/links` lists the caller's.
- Auth service owns `users` and is the only writer of that table.

## Non-goals

- Email verification, password reset, refresh tokens, logout/revocation.
- Social login, MFA, roles/permissions beyond link ownership.
- Frontend login UI (backend first; UI is a follow-up).
- Deleting or editing links.

## Anti-goals

- API reading or writing `users`.
- Token stored in a way that a plain page reload cannot recover (unless we
  choose to persist it, see Decisions).
- Anonymous shortening being broken by this change.

## Approaches considered

1. **Auth service owns users (chosen).** gRPC `Register`/`Login`, JWT signed by
   auth, verified locally by the API. Keeps the module boundary already in the
   repo, and the API never needs a DB read for identity.
2. **API owns users.** Rejected: would duplicate/undo the auth service the
   project deliberately kept.
3. **Per-request gRPC token validation.** Rejected: adds a network hop on every
   authenticated request and makes auth uptime a hard dependency of the API.

## Decisions

- Token transport: **Bearer token in the JSON response**. Frontend keeps it in
  memory. No cookie, no CSRF surface.
- Verification: **local HMAC check inside the API**, no gRPC hop.
- JWT claims: `sub` (user id), `email`, `iat`, `exp`.
- Anonymous shortening: **stays allowed**, `user_id` NULL.
- Redirects, stats, and QR codes: **public**, unchanged.
- `GET /api/links`: requires a valid token.
- The API-key flow (`ValidateToken`, `AUTH_API_KEY`, `middleware.Auth` gRPC
  call) is **removed**. JWT replaces it. This is a breaking config change.
- Secret: `JWT_SECRET` env, required, both services. TTL: `JWT_TTL`, default
  `24h`.
- Dependency: `github.com/golang-jwt/jwt/v5` in both modules, plus
  `golang.org/x/crypto` for bcrypt.

## Parts

### proto

```proto
service AuthService {
  rpc Register (RegisterRequest) returns (UserResponse);
  rpc Login    (LoginRequest)    returns (LoginResponse);
}
message RegisterRequest { string email = 1; string password = 2; }
message LoginRequest    { string email = 1; string password = 2; }
message UserResponse    { int32 id = 1; string email = 2; }
message LoginResponse   { string token = 1; UserResponse user = 2; }
```

### services/auth

- `database.Init()` at startup, so the service gets a Postgres pool.
- `Register`: normalize email (trim, lowercase), require password length >= 8,
  bcrypt hash, `CreateUser`, map unique violation to `AlreadyExists`.
- `Login`: `GetUserByEmail`, `bcrypt.CompareHashAndPassword`, sign JWT.
- Duplicate detection uses `pgconn.PgError` code `23505`, not string matching.

### services/api

- `handlers/auth.go`: `RegisterHandler`, `LoginHandler` call gRPC, return JSON.
  No bcrypt, no DB.
- `GET /api/links` -> gRPC-free: token contains `sub`, handler calls
  `GetURLsByUser`.
- `middleware.Auth`: parses `Authorization: Bearer <token>`, verifies signature
  and expiry with `JWT_SECRET`, puts user id in context. Missing or invalid
  token -> 401.
- `middleware.OptionalAuth`: same parse, but a missing token is not an error.
  Used by `POST /api/shorten` so anonymous links still work.
- `shorten.go`: `user_id` from context when present; retry switches from
  `rows == 1` to `errors.Is(err, pgx.ErrNoRows)` because `CreateURL` is now
  `:one ... RETURNING`.
- `RegisterRoutes` keeps taking the gRPC client (register/login need it).

### shared

- `shared/database`: user queries already exist; add nothing unless the
  `CreateUser` duplicate path needs a `:one` variant.
- `sqlc.yaml` unchanged.

### config

`.env` and `.env.example`:

```
JWT_SECRET=change-me
JWT_TTL=24h
```

`AUTH_API_KEY` is removed. `docker-compose.yml`: both `api` and `auth` get
`JWT_SECRET`/`JWT_TTL`; `auth` additionally gets the Postgres vars, since it
now talks to the database.

### schema

No new columns. `password_hash VARCHAR(255)` holds bcrypt output (60 chars).
Email uniqueness relies on the existing constraint plus lowercasing in the
service.

## Risks

- Removing `AUTH_API_KEY` breaks any client already using `/api/protected`
  with an API key. Acceptable: stub only.
- `POST /api/shorten` with an invalid Bearer token: treated as anonymous or
  rejected? Decision: rejected with 401, so a typo never silently creates an
  unowned link.
- No revocation: a leaked token is valid until `exp`.

## Open for the builder

- Exact bcrypt cost (default cost is fine).
- Whether `GET /api/links` should paginate. Suggested: no, until a user has
  more than a few hundred links.
- Frontend wiring and token persistence, deferred.
