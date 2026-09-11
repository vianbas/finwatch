# JWT Authentication & RBAC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add short-lived JWT access-token authentication and two-role RBAC (`operator`, `admin`) to the FinWatch API and wire the frontend login flow and route guard to it.

**Architecture:** A new `internal/auth` domain package (claims, bcrypt password hashing, HS256 issuer/verifier, login service, HTTP middleware) backed by a new `users` table and a Postgres store, mirroring the existing `internal/alerts` module shape (domain package + `store` subpackage + `httpapi` subpackage). The platform `httpserver` router gains a public/protected route split via a generic middleware hook, with zero new dependency from `httpserver` onto feature packages. The frontend gets a React context holding the access token in memory only, a route guard, and a wired `LoginPage`.

**Tech Stack:** Go, `github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`, pgx/sqlc, chi. React + TypeScript, TanStack Query, React Router.

## Global Constraints

- Access tokens only; no refresh tokens, OAuth/SAML, or password reset (out of scope).
- Access token TTL: 15 minutes. Signing algorithm: HS256.
- JWT claims: `sub`, `email`, `role`, `iat`, `exp`.
- Roles: `operator`, `admin` only — do not introduce `supervisor`.
- Frontend stores the access token in memory only — never `localStorage` or `sessionStorage`.
- WebSocket auth is issue #5's scope, not this one.
- No changes to alert workflow behavior.
- Money in integer minor units, timestamps RFC 3339 UTC at API boundaries (existing project rule).
- Errors use the canonical envelope `{"error":{"code","message"}}` matching `contracts/openapi.yaml`.
- All synthetic/example data only; demo passwords are clearly-labelled example dev credentials in `.env.example`, never logged in plaintext.
- `make verify` (gofmt, vet, go test, go build, eslint, tsc, vitest, vite build) must pass before considering any task done.

---

## File Structure

**Backend — new files:**

```
apps/api/migrations/0004_users.up.sql
apps/api/migrations/0004_users.down.sql
apps/api/queries/users.sql
apps/api/internal/auth/claims.go          # Role, Claims
apps/api/internal/auth/password.go        # bcrypt hash/verify
apps/api/internal/auth/password_test.go
apps/api/internal/auth/jwt.go             # Issuer, Verifier (HS256)
apps/api/internal/auth/jwt_test.go
apps/api/internal/auth/service.go         # UserStore port, Service.Login
apps/api/internal/auth/service_test.go
apps/api/internal/auth/middleware.go      # RequireAuth, RequireRole, ClaimsFromContext
apps/api/internal/auth/middleware_test.go
apps/api/internal/auth/store/store.go     # Postgres UserStore + seeding helper
apps/api/internal/auth/httpapi/handler.go # POST /login, GET /me
apps/api/internal/auth/httpapi/handler_test.go
```

**Backend — modified files:**

```
apps/api/go.mod / go.sum                                   # add golang-jwt/jwt/v5, x/crypto
apps/api/sqlc.yaml                                          # unchanged (queries dir already wired)
apps/api/internal/config/config.go                          # JWTSigningSecret, JWTAccessTokenTTL
apps/api/internal/config/config_test.go                     # new cases
apps/api/internal/platform/httpserver/router.go             # PublicModules, RequireAuth hook, RegistrarFunc
apps/api/internal/platform/httpserver/router_auth_test.go   # new: public/protected split behavior
apps/api/cmd/api/main.go                                     # wire auth, seed-users subcommand
apps/api/internal/platform/postgres/db/models.go            # sqlc-generated: User
apps/api/internal/platform/postgres/db/queries.sql.go        # sqlc-generated: GetUserByEmail, InsertUserIfAbsent
.env.example                                                  # JWT_SIGNING_SECRET, JWT_ACCESS_TOKEN_TTL, demo passwords
docker-compose.yml                                            # pass through new env vars to api service
contracts/openapi.yaml                                        # bearerAuth scheme, /login, /me, 401/403 responses
```

**Frontend — new files:**

```
apps/web/src/app/AuthContext.tsx
apps/web/src/app/AuthContext.test.tsx
apps/web/src/app/ProtectedRoute.tsx
apps/web/src/app/ProtectedRoute.test.tsx
apps/web/src/app/useApiFetch.ts
apps/web/src/app/useApiFetch.test.tsx
```

**Frontend — modified files:**

```
apps/web/src/app/App.tsx          # wrap with AuthProvider
apps/web/src/app/routes.tsx       # wrap authenticated routes with ProtectedRoute
apps/web/src/app/routes.test.tsx  # update for guard behavior
apps/web/src/pages/LoginPage.tsx  # wire to useAuth().login
apps/web/src/pages/LoginPage.test.tsx  # new
```

---

## Task 1: Add JWT and bcrypt dependencies

**Files:**
- Modify: `apps/api/go.mod`, `apps/api/go.sum`

**Interfaces:**
- Produces: `github.com/golang-jwt/jwt/v5` and `golang.org/x/crypto/bcrypt` importable in `internal/auth`.

- [ ] **Step 1: Add the dependencies**

Run:
```bash
cd apps/api && go get github.com/golang-jwt/jwt/v5@v5.2.1 && go get golang.org/x/crypto@v0.31.0
```

- [ ] **Step 2: Verify the module builds**

Run: `cd apps/api && go build ./...`
Expected: exits 0 (no source uses the new packages yet, so this only validates `go.mod`/`go.sum`).

- [ ] **Step 3: Commit**

```bash
cd apps/api && git add go.mod go.sum
git commit -m "build(api): add golang-jwt/jwt and x/crypto dependencies"
```

---

## Task 2: Users migration and sqlc query file

**Files:**
- Create: `apps/api/migrations/0004_users.up.sql`
- Create: `apps/api/migrations/0004_users.down.sql`
- Create: `apps/api/queries/users.sql`

**Interfaces:**
- Produces: `users` table (`id`, `email`, `password_hash`, `role`, `created_at`); sqlc queries `GetUserByEmail`, `InsertUserIfAbsent` consumed by Task 5 (`internal/auth/store`).

- [ ] **Step 1: Write the up migration**

Create `apps/api/migrations/0004_users.up.sql`:
```sql
-- 0004_users: user accounts for JWT authentication and RBAC.
--
-- Conventions match earlier migrations: UUID ids via gen_random_uuid,
-- TIMESTAMPTZ in UTC, small constrained value sets via CHECK. Roles are
-- intentionally limited to operator and admin for this issue.

CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    role          TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT users_role_valid CHECK (role IN ('operator', 'admin'))
);
```

- [ ] **Step 2: Write the down migration**

Create `apps/api/migrations/0004_users.down.sql`:
```sql
DROP TABLE users;
```

- [ ] **Step 3: Write the sqlc query file**

Create `apps/api/queries/users.sql`:
```sql
-- name: GetUserByEmail :one
SELECT id, email, password_hash, role, created_at
FROM users
WHERE email = $1;

-- name: InsertUserIfAbsent :one
-- Idempotent seeding: returns no row if the email already exists, so the
-- caller (the seed-users command) can skip logging a duplicate creation.
INSERT INTO users (email, password_hash, role)
VALUES ($1, $2, $3)
ON CONFLICT (email) DO NOTHING
RETURNING id, email, password_hash, role, created_at;
```

- [ ] **Step 4: Generate sqlc code**

sqlc is not preinstalled; install the same version pinned in CI (`.github/workflows/ci.yml`):
```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
cd apps/api && sqlc generate
```
Expected: `internal/platform/postgres/db/models.go` gains a `User` struct; `internal/platform/postgres/db/queries.sql.go` gains `GetUserByEmail` and `InsertUserIfAbsent` methods on `*Queries`.

- [ ] **Step 5: Verify generated code matches sources and the module builds**

Run:
```bash
cd apps/api && sqlc diff && go build ./...
```
Expected: `sqlc diff` prints nothing (clean) and the build succeeds.

- [ ] **Step 6: Commit**

```bash
git add apps/api/migrations/0004_users.up.sql apps/api/migrations/0004_users.down.sql \
        apps/api/queries/users.sql apps/api/internal/platform/postgres/db/models.go \
        apps/api/internal/platform/postgres/db/queries.sql.go
git commit -m "feat(api): add users table and sqlc queries for auth"
```

---

## Task 3: Config — JWT signing secret and access token TTL

**Files:**
- Modify: `apps/api/internal/config/config.go`
- Modify: `apps/api/internal/config/config_test.go`

**Interfaces:**
- Consumes: existing `Load(getenv func(string) string) (*Config, error)` pattern, `firstNonEmpty`, `durationEnv` helpers already in `config.go`.
- Produces: `Config.JWTSigningSecret string`, `Config.JWTAccessTokenTTL time.Duration`, consumed by Task 8 (`cmd/api/main.go` wiring).

- [ ] **Step 1: Write the failing tests**

Add to `apps/api/internal/config/config_test.go`, replacing `validEnv()` and adding cases:
```go
func validEnv() map[string]string {
	return map[string]string{
		"APP_ENV":           "development",
		"LOG_LEVEL":         "info",
		"DATABASE_URL":      "postgres://user:pass@localhost:5432/finwatch?sslmode=disable",
		"JWT_SIGNING_SECRET": "test-signing-secret-must-be-at-least-32-bytes",
	}
}
```
Add a new test function:
```go
func TestLoad_JWTDefaultsAndOverrides(t *testing.T) {
	cfg, err := Load(envFunc(validEnv()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.JWTAccessTokenTTL != 15*time.Minute {
		t.Errorf("JWTAccessTokenTTL = %v, want 15m", cfg.JWTAccessTokenTTL)
	}

	env := validEnv()
	env["JWT_ACCESS_TOKEN_TTL"] = "5m"
	cfg, err = Load(envFunc(env))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.JWTAccessTokenTTL != 5*time.Minute {
		t.Errorf("JWTAccessTokenTTL = %v, want 5m", cfg.JWTAccessTokenTTL)
	}
}
```
Extend the `TestLoad_ValidationErrors` table in the same file with:
```go
		{"missing jwt signing secret", func(m map[string]string) { delete(m, "JWT_SIGNING_SECRET") }},
		{"short jwt signing secret", func(m map[string]string) { m["JWT_SIGNING_SECRET"] = "too-short" }},
		{"non-duration jwt ttl", func(m map[string]string) { m["JWT_ACCESS_TOKEN_TTL"] = "soon" }},
		{"zero jwt ttl", func(m map[string]string) { m["JWT_ACCESS_TOKEN_TTL"] = "0s" }},
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && go test ./internal/config/... -run TestLoad -v`
Expected: compile error (`Config.JWTSigningSecret` undefined) or failing assertions.

- [ ] **Step 3: Implement the config fields**

In `apps/api/internal/config/config.go`, add to the `Config` struct (after `CORSAllowedOrigins`):
```go
	// JWTSigningSecret is the HS256 key used to sign and verify access tokens.
	JWTSigningSecret string
	// JWTAccessTokenTTL bounds how long an issued access token remains valid.
	JWTAccessTokenTTL time.Duration
```
In `Load`, after the `CORSAllowedOrigins` line, add:
```go
	cfg.JWTSigningSecret = getenv("JWT_SIGNING_SECRET")
	if cfg.JWTAccessTokenTTL, err = durationEnv(getenv, "JWT_ACCESS_TOKEN_TTL", 15*time.Minute); err != nil {
		return nil, err
	}
```
In `Validate`, after the `DatabaseURL` prefix check, add:
```go
	if len(c.JWTSigningSecret) < 32 {
		return fmt.Errorf("config: JWT_SIGNING_SECRET must be at least 32 characters")
	}
```
Add `"JWT_ACCESS_TOKEN_TTL": c.JWTAccessTokenTTL,` to the existing positive-duration validation map.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/api && go test ./internal/config/... -v`
Expected: PASS, all subtests green.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/config/config.go apps/api/internal/config/config_test.go
git commit -m "feat(api): add JWT signing secret and access token TTL to config"
```

---

## Task 4: Auth domain — claims, password hashing, JWT issue/verify

**Files:**
- Create: `apps/api/internal/auth/claims.go`
- Create: `apps/api/internal/auth/password.go`
- Test: `apps/api/internal/auth/password_test.go`
- Create: `apps/api/internal/auth/jwt.go`
- Test: `apps/api/internal/auth/jwt_test.go`

**Interfaces:**
- Produces: `auth.Role` (`RoleOperator`, `RoleAdmin`), `auth.Claims{UserID, Email, Role, IssuedAt, ExpiresAt}`, `auth.HashPassword(plain string) (string, error)`, `auth.VerifyPassword(hash, plain string) error`, `auth.NewIssuer(secret []byte, ttl time.Duration) *Issuer`, `(*Issuer).Issue(userID, email string, role Role) (string, error)`, `auth.NewVerifier(secret []byte) *Verifier`, `(*Verifier).Verify(token string) (Claims, error)`, `auth.ErrInvalidToken`, `auth.ErrExpiredToken`. Consumed by Task 5 (`Service`), Task 6 (`middleware.go`), Task 8 (`httpapi`).

- [ ] **Step 1: Write claims.go (no test needed — plain data type)**

Create `apps/api/internal/auth/claims.go`:
```go
// Package auth implements JWT-based authentication and role-based access
// control: password hashing, token issuance/verification, the login service,
// and HTTP middleware that enforces them.
package auth

import "time"

// Role is a RBAC role. Only operator and admin exist in this issue.
type Role string

const (
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

// Claims is the decoded, verified content of an access token.
type Claims struct {
	UserID    string
	Email     string
	Role      Role
	IssuedAt  time.Time
	ExpiresAt time.Time
}
```

- [ ] **Step 2: Write the failing password test**

Create `apps/api/internal/auth/password_test.go`:
```go
package auth

import "testing"

func TestHashPassword_VerifyRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct-password" {
		t.Fatalf("hash must not equal the plaintext")
	}
	if err := VerifyPassword(hash, "correct-password"); err != nil {
		t.Errorf("VerifyPassword with correct password: %v", err)
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := VerifyPassword(hash, "wrong-password"); err == nil {
		t.Errorf("VerifyPassword with wrong password: want error, got nil")
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd apps/api && go test ./internal/auth/... -run Password -v`
Expected: FAIL — `HashPassword`/`VerifyPassword` undefined.

- [ ] **Step 4: Implement password.go**

Create `apps/api/internal/auth/password.go`:
```go
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword bcrypt-hashes a plaintext password for storage.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword reports whether plain matches the given bcrypt hash.
func VerifyPassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `cd apps/api && go test ./internal/auth/... -run Password -v`
Expected: PASS.

- [ ] **Step 6: Write the failing JWT test**

Create `apps/api/internal/auth/jwt_test.go`:
```go
package auth

import (
	"errors"
	"testing"
	"time"
)

const testSecret = "test-signing-secret-must-be-at-least-32-bytes"

func TestIssueAndVerify_RoundTrip(t *testing.T) {
	issuer := NewIssuer([]byte(testSecret), 15*time.Minute)
	verifier := NewVerifier([]byte(testSecret))

	token, err := issuer.Issue("user-1", "operator@example.com", RoleOperator)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	claims, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", claims.UserID)
	}
	if claims.Email != "operator@example.com" {
		t.Errorf("Email = %q, want operator@example.com", claims.Email)
	}
	if claims.Role != RoleOperator {
		t.Errorf("Role = %q, want operator", claims.Role)
	}
	if !claims.ExpiresAt.After(claims.IssuedAt) {
		t.Errorf("ExpiresAt %v must be after IssuedAt %v", claims.ExpiresAt, claims.IssuedAt)
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	issuer := NewIssuer([]byte(testSecret), -1*time.Minute) // already expired
	verifier := NewVerifier([]byte(testSecret))

	token, err := issuer.Issue("user-1", "operator@example.com", RoleOperator)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := verifier.Verify(token); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("got %v, want ErrExpiredToken", err)
	}
}

func TestVerify_WrongSignature(t *testing.T) {
	issuer := NewIssuer([]byte(testSecret), 15*time.Minute)
	verifier := NewVerifier([]byte("a-completely-different-32-byte-secret!"))

	token, err := issuer.Issue("user-1", "operator@example.com", RoleOperator)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := verifier.Verify(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("got %v, want ErrInvalidToken", err)
	}
}

func TestVerify_MalformedToken(t *testing.T) {
	verifier := NewVerifier([]byte(testSecret))
	if _, err := verifier.Verify("not-a-jwt"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("got %v, want ErrInvalidToken", err)
	}
}
```

- [ ] **Step 7: Run to verify it fails**

Run: `cd apps/api && go test ./internal/auth/... -run 'Issue|Verify' -v`
Expected: FAIL — `NewIssuer`/`NewVerifier` undefined.

- [ ] **Step 8: Implement jwt.go**

Create `apps/api/internal/auth/jwt.go`:
```go
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned for malformed tokens or signature mismatches.
var ErrInvalidToken = errors.New("auth: invalid token")

// ErrExpiredToken is returned when the token's exp claim is in the past.
var ErrExpiredToken = errors.New("auth: token expired")

// tokenClaims is the on-the-wire JWT claim set: sub/iat/exp via
// RegisteredClaims, plus email and role.
type tokenClaims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

// Issuer signs short-lived HS256 access tokens.
type Issuer struct {
	secret []byte
	ttl    time.Duration
}

// NewIssuer constructs an Issuer. secret is the HS256 signing key; ttl is the
// access token lifetime (15 minutes per the architecture decision).
func NewIssuer(secret []byte, ttl time.Duration) *Issuer {
	return &Issuer{secret: secret, ttl: ttl}
}

// Issue signs a new access token for the given user.
func (i *Issuer) Issue(userID, email string, role Role) (string, error) {
	now := time.Now().UTC()
	claims := tokenClaims{
		Email: email,
		Role:  string(role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(i.secret)
}

// Verifier validates HS256 access tokens and extracts their claims.
type Verifier struct {
	secret []byte
}

// NewVerifier constructs a Verifier using the same secret the Issuer signed with.
func NewVerifier(secret []byte) *Verifier {
	return &Verifier{secret: secret}
}

// Verify parses and validates tokenString, returning ErrExpiredToken or
// ErrInvalidToken on failure.
func (v *Verifier) Verify(tokenString string) (Claims, error) {
	var claims tokenClaims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(*jwt.Token) (interface{}, error) {
		return v.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	if errors.Is(err, jwt.ErrTokenExpired) {
		return Claims{}, ErrExpiredToken
	}
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	return Claims{
		UserID:    claims.Subject,
		Email:     claims.Email,
		Role:      Role(claims.Role),
		IssuedAt:  claims.IssuedAt.Time,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}
```

- [ ] **Step 9: Run to verify it passes**

Run: `cd apps/api && go test ./internal/auth/... -v`
Expected: PASS, all of `password_test.go` and `jwt_test.go`.

- [ ] **Step 10: Commit**

```bash
git add apps/api/internal/auth/claims.go apps/api/internal/auth/password.go \
        apps/api/internal/auth/password_test.go apps/api/internal/auth/jwt.go \
        apps/api/internal/auth/jwt_test.go
git commit -m "feat(api): add JWT claims, bcrypt hashing, and HS256 issue/verify"
```

---

## Task 5: Login service

**Files:**
- Create: `apps/api/internal/auth/service.go`
- Test: `apps/api/internal/auth/service_test.go`

**Interfaces:**
- Consumes: `auth.Role`, `auth.Claims` (Task 4), `*Issuer` and its `Issue` method (Task 4).
- Produces: `auth.User{ID, Email, PasswordHash, Role}`, `auth.UserStore` interface (`GetUserByEmail(ctx, email) (User, error)`), `auth.ErrUserNotFound`, `auth.ErrInvalidCredentials`, `auth.NewService(store UserStore, issuer *Issuer) *Service`, `(*Service).Login(ctx, email, password string) (string, User, error)`. Consumed by Task 7 (`store` implements `UserStore`) and Task 8 (`httpapi` calls `Login`).

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/auth/service_test.go`:
```go
package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
)

type fakeUserStore struct {
	usersByEmail map[string]auth.User
}

func (f *fakeUserStore) GetUserByEmail(_ context.Context, email string) (auth.User, error) {
	u, ok := f.usersByEmail[email]
	if !ok {
		return auth.User{}, auth.ErrUserNotFound
	}
	return u, nil
}

func newTestService(t *testing.T, users ...auth.User) *auth.Service {
	t.Helper()
	byEmail := make(map[string]auth.User, len(users))
	for _, u := range users {
		byEmail[u.Email] = u
	}
	issuer := auth.NewIssuer([]byte("test-signing-secret-must-be-at-least-32-bytes"), 15*time.Minute)
	return auth.NewService(&fakeUserStore{usersByEmail: byEmail}, issuer)
}

func userWithPassword(t *testing.T, email, password string, role auth.Role) auth.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return auth.User{ID: "user-1", Email: email, PasswordHash: hash, Role: role}
}

func TestService_Login_ValidCredentials(t *testing.T) {
	user := userWithPassword(t, "operator@example.com", "correct-password", auth.RoleOperator)
	svc := newTestService(t, user)

	token, got, err := svc.Login(context.Background(), "operator@example.com", "correct-password")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" {
		t.Errorf("want non-empty token")
	}
	if got.Email != user.Email || got.Role != user.Role {
		t.Errorf("got user %+v, want email/role to match %+v", got, user)
	}
}

func TestService_Login_WrongPassword(t *testing.T) {
	user := userWithPassword(t, "operator@example.com", "correct-password", auth.RoleOperator)
	svc := newTestService(t, user)

	_, _, err := svc.Login(context.Background(), "operator@example.com", "wrong-password")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("got %v, want ErrInvalidCredentials", err)
	}
}

func TestService_Login_UnknownEmail(t *testing.T) {
	svc := newTestService(t)

	_, _, err := svc.Login(context.Background(), "nobody@example.com", "whatever")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("got %v, want ErrInvalidCredentials", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && go test ./internal/auth/... -run TestService -v`
Expected: FAIL — `auth.User`, `auth.UserStore`, `auth.NewService` undefined.

- [ ] **Step 3: Implement service.go**

Create `apps/api/internal/auth/service.go`:
```go
package auth

import (
	"context"
	"errors"
	"fmt"
)

// ErrUserNotFound is returned by UserStore when no user has the given email.
var ErrUserNotFound = errors.New("auth: user not found")

// ErrInvalidCredentials is returned by Service.Login for any failure that
// should not reveal whether the email or the password was wrong.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// User is a persisted account used for login.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         Role
}

// UserStore is the persistence port for user accounts.
type UserStore interface {
	GetUserByEmail(ctx context.Context, email string) (User, error)
}

// Service implements the login use case: verify credentials, issue a token.
type Service struct {
	store  UserStore
	issuer *Issuer
}

// NewService constructs a Service.
func NewService(store UserStore, issuer *Issuer) *Service {
	return &Service{store: store, issuer: issuer}
}

// Login verifies email/password and, on success, returns a signed access
// token and the authenticated user. Unknown email and wrong password both
// return ErrInvalidCredentials so the caller cannot distinguish them.
func (s *Service) Login(ctx context.Context, email, password string) (string, User, error) {
	user, err := s.store.GetUserByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		return "", User{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", User{}, fmt.Errorf("auth: get user by email: %w", err)
	}

	if err := VerifyPassword(user.PasswordHash, password); err != nil {
		return "", User{}, ErrInvalidCredentials
	}

	token, err := s.issuer.Issue(user.ID, user.Email, user.Role)
	if err != nil {
		return "", User{}, fmt.Errorf("auth: issue token: %w", err)
	}
	return token, user, nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/api && go test ./internal/auth/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/auth/service.go apps/api/internal/auth/service_test.go
git commit -m "feat(api): add login service with bcrypt verification"
```

---

## Task 6: Auth HTTP middleware (RequireAuth, RequireRole)

**Files:**
- Create: `apps/api/internal/auth/middleware.go`
- Test: `apps/api/internal/auth/middleware_test.go`

**Interfaces:**
- Consumes: `*Verifier` and `Verify` (Task 4), `Claims`, `Role` (Task 4), `web.WriteError` (`internal/platform/web`, existing).
- Produces: `auth.RequireAuth(verifier *Verifier) func(http.Handler) http.Handler`, `auth.RequireRole(role Role) func(http.Handler) http.Handler`, `auth.ClaimsFromContext(ctx) (Claims, bool)`. Consumed by Task 9 (router wiring) and Task 8 (`httpapi.me` reads claims).

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/auth/middleware_test.go`:
```go
package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
)

const mwTestSecret = "test-signing-secret-must-be-at-least-32-bytes"

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if ok {
			w.Header().Set("X-Role", string(claims.Role))
		}
		w.WriteHeader(http.StatusOK)
	})
}

func issueToken(t *testing.T, ttl time.Duration, role auth.Role) string {
	t.Helper()
	issuer := auth.NewIssuer([]byte(mwTestSecret), ttl)
	token, err := issuer.Issue("user-1", "operator@example.com", role)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return token
}

func TestRequireAuth_MissingToken(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_MalformedToken(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(okHandler())
	token := issueToken(t, -1*time.Minute, auth.RoleOperator)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_ValidToken(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(okHandler())
	token := issueToken(t, 15*time.Minute, auth.RoleOperator)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Role"); got != "operator" {
		t.Errorf("X-Role = %q, want operator", got)
	}
}

func TestRequireRole_OperatorDeniedFromAdminRoute(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(auth.RequireRole(auth.RoleAdmin)(okHandler()))
	token := issueToken(t, 15*time.Minute, auth.RoleOperator)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestRequireRole_AdminAllowed(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(auth.RequireRole(auth.RoleAdmin)(okHandler()))
	token := issueToken(t, 15*time.Minute, auth.RoleAdmin)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && go test ./internal/auth/... -run 'TestRequireAuth|TestRequireRole' -v`
Expected: FAIL — `auth.RequireAuth`, `auth.RequireRole`, `auth.ClaimsFromContext` undefined.

- [ ] **Step 3: Implement middleware.go**

Create `apps/api/internal/auth/middleware.go`:
```go
package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/vianbas/finwatch/apps/api/internal/platform/web"
)

type contextKey string

const claimsContextKey contextKey = "auth_claims"

// RequireAuth rejects requests without a valid, unexpired bearer token with
// 401, and otherwise stores the decoded Claims on the request context.
func RequireAuth(verifier *Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				web.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
				return
			}
			claims, err := verifier.Verify(token)
			if err != nil {
				web.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole rejects requests whose authenticated Claims do not carry the
// given role with 403. It must run behind RequireAuth.
func RequireRole(role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok || claims.Role != role {
				web.WriteError(w, http.StatusForbidden, "FORBIDDEN", "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClaimsFromContext returns the Claims stored by RequireAuth, if any.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(Claims)
	return claims, ok
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", false
	}
	return token, true
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/api && go test ./internal/auth/... -v`
Expected: PASS, all tests in the package.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/auth/middleware.go apps/api/internal/auth/middleware_test.go
git commit -m "feat(api): add RequireAuth and RequireRole HTTP middleware"
```

---

## Task 7: Postgres user store

**Files:**
- Create: `apps/api/internal/auth/store/store.go`

**Interfaces:**
- Consumes: `db.New(pool).GetUserByEmail` / `InsertUserIfAbsent` (sqlc-generated, Task 2), `pgconv.UUIDString` (existing `internal/platform/postgres/pgconv`), `auth.User`, `auth.Role`, `auth.ErrUserNotFound` (Task 5).
- Produces: `store.New(pool *pgxpool.Pool) *Store` implementing `auth.UserStore`; `(*Store).InsertUserIfAbsent(ctx, email, passwordHash string, role auth.Role) (auth.User, bool, error)` consumed by Task 10 (`seed-users` command).

This task has no unit test of its own — it is a thin Postgres adapter exercised indirectly through `make verify`'s build/vet step and, if a database is available locally, manually via the `seed-users` command added in Task 10. This matches the existing `internal/alerts/store` package, which is also untested in isolation (its behavior is covered through the domain `Service` tests with a fake `Repository`).

- [ ] **Step 1: Implement store.go**

Create `apps/api/internal/auth/store/store.go`:
```go
// Package store is the PostgreSQL implementation of auth.UserStore.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
	"github.com/vianbas/finwatch/apps/api/internal/platform/postgres/db"
	"github.com/vianbas/finwatch/apps/api/internal/platform/postgres/pgconv"
)

// Store persists users in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// New constructs a Store.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// GetUserByEmail implements auth.UserStore.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (auth.User, error) {
	row, err := db.New(s.pool).GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("store: get user by email: %w", err)
	}
	return toDomain(row), nil
}

// InsertUserIfAbsent creates a user with an already-hashed password. created
// is false (with a nil error) if the email already exists, so the
// seed-users command can skip logging a duplicate.
func (s *Store) InsertUserIfAbsent(ctx context.Context, email, passwordHash string, role auth.Role) (auth.User, bool, error) {
	row, err := db.New(s.pool).InsertUserIfAbsent(ctx, db.InsertUserIfAbsentParams{
		Email:        email,
		PasswordHash: passwordHash,
		Role:         string(role),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, false, nil
	}
	if err != nil {
		return auth.User{}, false, fmt.Errorf("store: insert user: %w", err)
	}
	return toDomain(row), true, nil
}

func toDomain(r db.User) auth.User {
	return auth.User{
		ID:           pgconv.UUIDString(r.ID),
		Email:        r.Email,
		PasswordHash: r.PasswordHash,
		Role:         auth.Role(r.Role),
	}
}
```

- [ ] **Step 2: Verify it builds and vets cleanly**

Run: `cd apps/api && go build ./... && go vet ./...`
Expected: exits 0. (Field names `db.InsertUserIfAbsentParams{Email, PasswordHash, Role}` must match what sqlc generated in Task 2 — if `go build` reports a field mismatch, open `internal/platform/postgres/db/queries.sql.go` and adjust this file's field names to match exactly.)

- [ ] **Step 3: Commit**

```bash
git add apps/api/internal/auth/store/store.go
git commit -m "feat(api): add Postgres-backed user store"
```

---

## Task 8: Auth HTTP handlers (POST /login, GET /me)

**Files:**
- Create: `apps/api/internal/auth/httpapi/handler.go`
- Test: `apps/api/internal/auth/httpapi/handler_test.go`

**Interfaces:**
- Consumes: `auth.NewService`, `*auth.Service.Login` (Task 5), `auth.ClaimsFromContext` (Task 6), `auth.ErrInvalidCredentials` (Task 5), `web.WriteJSON`/`web.WriteError` (existing), `chi.Router`.
- Produces: `httpapi.NewHandler(svc *auth.Service, log *slog.Logger) *Handler`; `(*Handler).RegisterPublicRoutes(r chi.Router)` mounting `POST /login`; `(*Handler).RegisterProtectedRoutes(r chi.Router)` mounting `GET /me`. Consumed by Task 10 (`cmd/api/main.go` wiring) via Task 9's `RegistrarFunc` adapter.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/auth/httpapi/handler_test.go`:
```go
package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
	"github.com/vianbas/finwatch/apps/api/internal/auth/httpapi"
)

const handlerTestSecret = "test-signing-secret-must-be-at-least-32-bytes"

type fakeUserStore struct {
	usersByEmail map[string]auth.User
}

func (f *fakeUserStore) GetUserByEmail(_ context.Context, email string) (auth.User, error) {
	u, ok := f.usersByEmail[email]
	if !ok {
		return auth.User{}, auth.ErrUserNotFound
	}
	return u, nil
}

func newRouter(t *testing.T, users ...auth.User) http.Handler {
	t.Helper()
	byEmail := make(map[string]auth.User, len(users))
	for _, u := range users {
		byEmail[u.Email] = u
	}
	issuer := auth.NewIssuer([]byte(handlerTestSecret), 15*time.Minute)
	verifier := auth.NewVerifier([]byte(handlerTestSecret))
	svc := auth.NewService(&fakeUserStore{usersByEmail: byEmail}, issuer)
	h := httpapi.NewHandler(svc, slog.Default())

	r := chi.NewRouter()
	h.RegisterPublicRoutes(r)
	r.Group(func(pr chi.Router) {
		pr.Use(auth.RequireAuth(verifier))
		h.RegisterProtectedRoutes(pr)
	})
	return r
}

func userWithPassword(t *testing.T, email, password string, role auth.Role) auth.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return auth.User{ID: "user-1", Email: email, PasswordHash: hash, Role: role}
}

func decode(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode body: %v\nbody: %s", err, body)
	}
	return m
}

func TestLogin_ValidCredentials(t *testing.T) {
	user := userWithPassword(t, "operator@example.com", "correct-password", auth.RoleOperator)
	router := newRouter(t, user)

	body, _ := json.Marshal(map[string]string{"email": "operator@example.com", "password": "correct-password"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	resp := decode(t, rec.Body.Bytes())
	if resp["accessToken"] == "" || resp["accessToken"] == nil {
		t.Errorf("want non-empty accessToken, got %v", resp["accessToken"])
	}
	user_, ok := resp["user"].(map[string]any)
	if !ok || user_["email"] != "operator@example.com" || user_["role"] != "operator" {
		t.Errorf("user field = %v, want email/role operator@example.com/operator", resp["user"])
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	user := userWithPassword(t, "operator@example.com", "correct-password", auth.RoleOperator)
	router := newRouter(t, user)

	body, _ := json.Marshal(map[string]string{"email": "operator@example.com", "password": "wrong-password"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	router := newRouter(t)

	body, _ := json.Marshal(map[string]string{"email": "", "password": ""})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestMe_ValidToken_ReturnsClaims(t *testing.T) {
	user := userWithPassword(t, "operator@example.com", "correct-password", auth.RoleOperator)
	router := newRouter(t, user)

	loginBody, _ := json.Marshal(map[string]string{"email": "operator@example.com", "password": "correct-password"})
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody)))
	token := decode(t, loginRec.Body.Bytes())["accessToken"].(string)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	resp := decode(t, rec.Body.Bytes())
	if resp["email"] != "operator@example.com" || resp["role"] != "operator" {
		t.Errorf("got %v, want email/role operator@example.com/operator", resp)
	}
}

func TestMe_NoToken_Returns401(t *testing.T) {
	router := newRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && go test ./internal/auth/httpapi/... -v`
Expected: FAIL — package `httpapi` and `NewHandler` undefined.

- [ ] **Step 3: Implement handler.go**

Create `apps/api/internal/auth/httpapi/handler.go`:
```go
// Package httpapi exposes the auth module over HTTP: login and the current
// user. It validates input at the boundary and holds no business logic.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
	"github.com/vianbas/finwatch/apps/api/internal/platform/web"
)

// Handler serves the auth HTTP endpoints.
type Handler struct {
	svc *auth.Service
	log *slog.Logger
}

// NewHandler constructs a Handler.
func NewHandler(svc *auth.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// RegisterPublicRoutes mounts routes that do not require authentication.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/login", h.login)
}

// RegisterProtectedRoutes mounts routes that require a valid bearer token.
// The caller is responsible for applying auth.RequireAuth to this router.
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Get("/me", h.me)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type loginResponse struct {
	AccessToken string  `json:"accessToken"`
	User        userDTO `json:"user"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		web.WriteError(w, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		web.WriteError(w, http.StatusBadRequest, "INVALID_BODY", "email and password are required")
		return
	}

	token, user, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			web.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email or password is incorrect")
			return
		}
		h.log.ErrorContext(r.Context(), "login failed", slog.String("error", err.Error()))
		web.WriteError(w, http.StatusInternalServerError, "INTERNAL", "unexpected error")
		return
	}
	web.WriteJSON(w, http.StatusOK, loginResponse{AccessToken: token, User: toUserDTO(user)})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		web.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
		return
	}
	web.WriteJSON(w, http.StatusOK, userDTO{ID: claims.UserID, Email: claims.Email, Role: string(claims.Role)})
}

func toUserDTO(u auth.User) userDTO {
	return userDTO{ID: u.ID, Email: u.Email, Role: string(u.Role)}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/api && go test ./internal/auth/httpapi/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/auth/httpapi/handler.go apps/api/internal/auth/httpapi/handler_test.go
git commit -m "feat(api): add POST /login and GET /me handlers"
```

---

## Task 9: Router — public/protected route split

**Files:**
- Modify: `apps/api/internal/platform/httpserver/router.go`
- Create: `apps/api/internal/platform/httpserver/router_auth_test.go`

**Interfaces:**
- Consumes: nothing new from feature packages — `RouterDeps.RequireAuth` is a plain `func(http.Handler) http.Handler`, keeping `httpserver` free of any dependency on `internal/auth`.
- Produces: `RouterDeps.PublicModules []RouteRegistrar`, `RouterDeps.RequireAuth func(http.Handler) http.Handler`, `httpserver.RegistrarFunc` (adapter type). Consumed by Task 10 (`cmd/api/main.go` passes `auth.RequireAuth(verifier)` and wraps handler methods in `RegistrarFunc`).

This task changes router behavior without breaking the existing `TestRouter_NotFound` test in `health_test.go` (`RouterDeps{Logger: testLogger(), Health: h}` still compiles — the new fields default to nil/empty and are no-ops).

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/platform/httpserver/router_auth_test.go`:
```go
package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func registrar(register func(chi.Router)) RouteRegistrar {
	return RegistrarFunc(register)
}

func TestRouter_PublicModuleRunsWithoutAuthMiddleware(t *testing.T) {
	h := NewHealthHandler(fakePinger{})
	router := NewRouter(RouterDeps{
		Logger: testLogger(),
		Health: h,
		PublicModules: []RouteRegistrar{
			registrar(func(r chi.Router) {
				r.Get("/public", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
			}),
		},
		RequireAuth: func(http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized) // would reject everything if applied
			})
		},
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/public", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (public route must bypass RequireAuth)", rec.Code)
	}
}

func TestRouter_ProtectedModuleRunsBehindAuthMiddleware(t *testing.T) {
	h := NewHealthHandler(fakePinger{})
	router := NewRouter(RouterDeps{
		Logger: testLogger(),
		Health: h,
		Modules: []RouteRegistrar{
			registrar(func(r chi.Router) {
				r.Get("/protected", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
			}),
		},
		RequireAuth: func(http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			})
		},
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/protected", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (protected route must run behind RequireAuth)", rec.Code)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && go test ./internal/platform/httpserver/... -run TestRouter -v`
Expected: FAIL — `RouterDeps.PublicModules`, `RouterDeps.RequireAuth`, `RegistrarFunc` undefined.

- [ ] **Step 3: Implement the router changes**

In `apps/api/internal/platform/httpserver/router.go`, add after the `RouteRegistrar` interface:
```go
// RegistrarFunc adapts a plain function to RouteRegistrar, the same pattern
// as http.HandlerFunc.
type RegistrarFunc func(r chi.Router)

// RegisterRoutes implements RouteRegistrar.
func (f RegistrarFunc) RegisterRoutes(r chi.Router) { f(r) }
```
Replace the `RouterDeps` struct with:
```go
// RouterDeps are the dependencies required to build the HTTP router.
type RouterDeps struct {
	Logger *slog.Logger
	Health *HealthHandler
	// PublicModules mount routes that do not require authentication (e.g. login).
	PublicModules []RouteRegistrar
	// Modules mount routes that require a valid bearer token when RequireAuth
	// is set.
	Modules []RouteRegistrar
	// RequireAuth, if non-nil, wraps Modules' routes. It is a plain middleware
	// function so this package has no dependency on the auth feature package.
	RequireAuth func(http.Handler) http.Handler
}
```
Replace the route-mounting body of `NewRouter` (the two comment blocks plus the `for _, m := range deps.Modules` loop) with:
```go
	// Operational endpoints. These are intentionally unauthenticated.
	r.Get("/health/live", deps.Health.Live)
	r.Get("/health/ready", deps.Health.Ready)

	// Public feature routes (e.g. login) mount without auth middleware.
	for _, m := range deps.PublicModules {
		if m != nil {
			m.RegisterRoutes(r)
		}
	}

	// Protected feature routes run behind RequireAuth when configured.
	r.Group(func(pr chi.Router) {
		if deps.RequireAuth != nil {
			pr.Use(deps.RequireAuth)
		}
		for _, m := range deps.Modules {
			if m != nil {
				m.RegisterRoutes(pr)
			}
		}
	})
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/api && go test ./internal/platform/httpserver/... -v`
Expected: PASS, including the pre-existing `TestRouter_NotFound` and all `TestHealth_*` tests (no regression).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/platform/httpserver/router.go apps/api/internal/platform/httpserver/router_auth_test.go
git commit -m "feat(api): split router into public and auth-protected route groups"
```

---

## Task 10: Wire auth into cmd/api, add seed-users command

**Files:**
- Modify: `apps/api/cmd/api/main.go`

**Interfaces:**
- Consumes: `config.Config.JWTSigningSecret`/`JWTAccessTokenTTL` (Task 3), `auth.NewIssuer`/`NewVerifier`/`NewService`/`RequireAuth`/`HashPassword`/`RoleOperator`/`RoleAdmin` (Tasks 4-6), `authstore.New`/`InsertUserIfAbsent` (Task 7), `authhttp.NewHandler` (Task 8), `httpserver.RegistrarFunc` (Task 9).
- Produces: the wired `/login`, `/me` routes and protected `/transactions`, `/alerts*` routes in the running server; a `seed-users` CLI subcommand.

- [ ] **Step 1: Add imports**

In `apps/api/cmd/api/main.go`, add to the import block:
```go
	"github.com/vianbas/finwatch/apps/api/internal/auth"
	authstore "github.com/vianbas/finwatch/apps/api/internal/auth/store"
	authhttp "github.com/vianbas/finwatch/apps/api/internal/auth/httpapi"
```

- [ ] **Step 2: Add the seed-users subcommand dispatch**

In `main()`, after the existing `seed` subcommand block, add:
```go
	// `api seed-users` creates the demo operator/admin accounts and exits.
	if len(os.Args) > 1 && os.Args[1] == "seed-users" {
		if err := runSeedUsers(); err != nil {
			os.Exit(1)
		}
		return
	}
```

- [ ] **Step 3: Implement runSeedUsers**

Add a new function near `runSeed`:
```go
// runSeedUsers creates the demo operator and admin accounts used for local
// development and manual testing. It is idempotent: existing emails are left
// untouched. Passwords are never logged.
func runSeedUsers() error {
	cfg, err := config.Load(os.Getenv)
	logger := newLogger(cfg, err)
	if err != nil {
		logger.Error("invalid configuration", slog.String("error", err.Error()))
		return err
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to initialise database pool", slog.String("error", err.Error()))
		return err
	}
	defer pool.Close()

	demoUsers := []struct {
		email    string
		password string
		role     auth.Role
	}{
		{email: "operator@example.com", password: demoPassword("DEMO_OPERATOR_PASSWORD", "operator_dev_password"), role: auth.RoleOperator},
		{email: "admin@example.com", password: demoPassword("DEMO_ADMIN_PASSWORD", "admin_dev_password"), role: auth.RoleAdmin},
	}

	repo := authstore.New(pool)
	for _, u := range demoUsers {
		hash, err := auth.HashPassword(u.password)
		if err != nil {
			logger.Error("failed to hash demo password", slog.String("error", err.Error()))
			return err
		}
		_, created, err := repo.InsertUserIfAbsent(ctx, u.email, hash, u.role)
		if err != nil {
			logger.Error("failed to seed demo user", slog.String("email", u.email), slog.String("error", err.Error()))
			return err
		}
		logger.Info("seed user", slog.String("email", u.email), slog.Bool("created", created))
	}
	return nil
}

func demoPassword(envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: Wire auth into the running server**

In `run()`, after `svcs := buildServices(pool, logger)`, add:
```go
	issuer := auth.NewIssuer([]byte(cfg.JWTSigningSecret), cfg.JWTAccessTokenTTL)
	verifier := auth.NewVerifier([]byte(cfg.JWTSigningSecret))
	authSvc := auth.NewService(authstore.New(pool), issuer)
	authHandler := authhttp.NewHandler(authSvc, logger)
```
Replace the `httpserver.NewRouter` call with:
```go
	router := httpserver.NewRouter(httpserver.RouterDeps{
		Logger: logger,
		Health: httpserver.NewHealthHandler(pool),
		PublicModules: []httpserver.RouteRegistrar{
			httpserver.RegistrarFunc(authHandler.RegisterPublicRoutes),
		},
		Modules: []httpserver.RouteRegistrar{
			httpserver.RegistrarFunc(authHandler.RegisterProtectedRoutes),
			txhttp.NewHandler(svcs.transactions, logger),
			alerthttp.NewHandler(svcs.alerts, logger),
		},
		RequireAuth: auth.RequireAuth(verifier),
	})
```

- [ ] **Step 5: Verify the binary builds and existing tests still pass**

Run: `cd apps/api && go build ./... && go vet ./... && go test ./...`
Expected: build succeeds; all tests pass (including the auth package tests from Tasks 4-9).

- [ ] **Step 6: Commit**

```bash
git add apps/api/cmd/api/main.go
git commit -m "feat(api): wire JWT auth into the HTTP server and add seed-users command"
```

---

## Task 11: OpenAPI contract — bearerAuth, /login, /me, 401/403

**Files:**
- Modify: `contracts/openapi.yaml`

**Interfaces:**
- Produces: the committed contract that Task 8's handlers must match (`POST /login`, `GET /me`, error codes `INVALID_BODY`, `INVALID_CREDENTIALS`, `UNAUTHORIZED`, `FORBIDDEN`).

- [ ] **Step 1: Add the `auth` tag**

In `contracts/openapi.yaml`, add to the `tags` list (after `health`):
```yaml
  - name: auth
    description: Login and the current authenticated user.
```

- [ ] **Step 2: Add the security scheme and a global default**

Add a top-level `security` key right after the `servers` block:
```yaml
security:
  - bearerAuth: []
```
In `components`, before `schemas:`, add:
```yaml
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
      description: Short-lived (15 minute) HS256 access token returned by POST /login.
```

- [ ] **Step 3: Mark health and login as public**

Add `security: []` to the `get:` operation under `/health/live` and under `/health/ready` (each gets its own line directly under its `operationId`/`summary` block, e.g. right after `summary: Liveness probe`):
```yaml
      security: []
```
(Apply the same one-line addition to the `/health/ready` `get:` operation.)

- [ ] **Step 4: Add the /login and /me paths**

Insert after the `/health/ready` block and before `/transactions`:
```yaml
  /login:
    post:
      tags: [auth]
      operationId: login
      summary: Authenticate with email and password
      security: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/LoginRequest"
      responses:
        "200":
          description: Authenticated; returns a short-lived access token.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/LoginResponse"
        "400":
          description: Missing or malformed email/password.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
        "401":
          description: Email or password is incorrect.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
              example:
                error:
                  code: INVALID_CREDENTIALS
                  message: email or password is incorrect
  /me:
    get:
      tags: [auth]
      operationId: getCurrentUser
      summary: Get the authenticated user
      responses:
        "200":
          description: The authenticated user.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
        "401":
          $ref: "#/components/responses/Unauthorized"
```

- [ ] **Step 5: Add 401/403 responses to protected feature endpoints**

Add a `"401": { $ref: "#/components/responses/Unauthorized" }` entry to the `responses:` map of each of these existing operations: `GET /transactions`, `GET /alerts`, `GET /alerts/{id}`, `POST /alerts/{id}/acknowledge`, `POST /alerts/{id}/resolve`. For example, `/transactions`'s `get.responses` becomes:
```yaml
      responses:
        "200":
          description: A page of transactions.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/TransactionPage"
        "400":
          description: Invalid query parameters.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
              example:
                error:
                  code: INVALID_QUERY
                  message: limit must be between 1 and 200
        "401":
          $ref: "#/components/responses/Unauthorized"
```
(Apply the same single added `"401"` entry to the other four operations listed above, leaving their existing `200`/`400`/`404`/`409` responses untouched.)

- [ ] **Step 6: Add the new schemas**

In `components.schemas`, add (alongside the existing `Error` schema):
```yaml
    LoginRequest:
      type: object
      required: [email, password]
      properties:
        email:
          type: string
          format: email
        password:
          type: string
          format: password
    User:
      type: object
      required: [id, email, role]
      properties:
        id:
          type: string
          format: uuid
        email:
          type: string
          format: email
        role:
          type: string
          enum: [operator, admin]
    LoginResponse:
      type: object
      required: [accessToken, user]
      properties:
        accessToken:
          type: string
          description: Short-lived (15 minute) HS256 JWT access token.
        user:
          $ref: "#/components/schemas/User"
```

- [ ] **Step 7: Add the reusable 401/403 responses**

In `components.responses`, add alongside the existing `NotFound`:
```yaml
    Unauthorized:
      description: Missing, malformed, or expired bearer token.
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/Error"
          example:
            error:
              code: UNAUTHORIZED
              message: missing bearer token
    Forbidden:
      description: The authenticated user's role does not permit this action.
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/Error"
          example:
            error:
              code: FORBIDDEN
              message: insufficient role
```

- [ ] **Step 8: Validate the contract**

Run (use whichever OpenAPI validator the project's contract-lint CI step uses; check `.github/workflows/` for the exact tool/command first):
```bash
grep -rn "openapi" /Users/viko/Documents/dev/code/finwatch/.github/workflows/*.yml
```
Then run that same validation command locally against `contracts/openapi.yaml` and confirm it reports no errors.

- [ ] **Step 9: Commit**

```bash
git add contracts/openapi.yaml
git commit -m "docs(contracts): add bearer auth, /login, /me, and 401/403 responses"
```

---

## Task 12: Environment and Compose wiring

**Files:**
- Modify: `.env.example`
- Modify: `docker-compose.yml`

**Interfaces:**
- Produces: `JWT_SIGNING_SECRET`, `JWT_ACCESS_TOKEN_TTL`, `DEMO_OPERATOR_PASSWORD`, `DEMO_ADMIN_PASSWORD` available to local `docker compose up` and to anyone copying `.env.example` to `.env`.

- [ ] **Step 1: Add example values to .env.example**

In `.env.example`, add to the `# --- API ---------------------------------------------------------------` section (after `CORS_ALLOWED_ORIGINS`):
```
# JWT signing secret: a SAFE example dev value only, at least 32 characters.
# Never reuse this value outside local development.
JWT_SIGNING_SECRET=dev_only_example_secret_change_me_32+chars
JWT_ACCESS_TOKEN_TTL=15m

# Demo account passwords for `go run ./cmd/api seed-users` (operator@example.com,
# admin@example.com). SAFE example dev values only.
DEMO_OPERATOR_PASSWORD=operator_dev_password
DEMO_ADMIN_PASSWORD=admin_dev_password
```

- [ ] **Step 2: Pass the new variables through to the api service in docker-compose.yml**

In `docker-compose.yml`, in the `api` service's `environment:` block, add after `CORS_ALLOWED_ORIGINS`:
```yaml
      JWT_SIGNING_SECRET: ${JWT_SIGNING_SECRET:-dev_only_example_secret_change_me_32+chars}
      JWT_ACCESS_TOKEN_TTL: ${JWT_ACCESS_TOKEN_TTL:-15m}
```

- [ ] **Step 3: Validate Compose config**

Run: `docker compose config -q && echo ok`
Expected: prints `ok` with no errors.

- [ ] **Step 4: Commit**

```bash
git add .env.example docker-compose.yml
git commit -m "chore: add JWT env vars and demo user passwords to local dev config"
```

---

## Task 13: AuthContext — in-memory token store

**Files:**
- Create: `apps/web/src/app/AuthContext.tsx`
- Test: `apps/web/src/app/AuthContext.test.tsx`

**Interfaces:**
- Consumes: `env.apiUrl` from `apps/web/src/lib/env.ts` (existing).
- Produces: `AuthUser{id, email, role}`, `AuthProvider` component, `useAuth()` hook returning `{accessToken: string | null, user: AuthUser | null, login(email, password): Promise<void>, logout(): void}`. Consumed by Task 14 (`ProtectedRoute`), Task 15 (`useApiFetch`), Task 16 (`LoginPage`), Task 17 (`App.tsx`).

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/app/AuthContext.test.tsx`:
```tsx
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuthProvider, useAuth } from "@/app/AuthContext";

function Probe() {
  const { accessToken, user, login, logout } = useAuth();
  return (
    <div>
      <span data-testid="token">{accessToken ?? "none"}</span>
      <span data-testid="role">{user?.role ?? "none"}</span>
      <button onClick={() => login("operator@example.com", "correct-password")}>login</button>
      <button onClick={logout}>logout</button>
    </div>
  );
}

describe("AuthContext", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("stores the access token and user after a successful login", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({
        accessToken: "token-123",
        user: { id: "u1", email: "operator@example.com", role: "operator" },
      }),
    });

    render(<AuthProvider><Probe /></AuthProvider>);
    await userEvent.click(screen.getByText("login"));

    await waitFor(() => expect(screen.getByTestId("token")).toHaveTextContent("token-123"));
    expect(screen.getByTestId("role")).toHaveTextContent("operator");
  });

  it("throws and leaves state empty when login fails", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      json: async () => ({ error: { code: "INVALID_CREDENTIALS", message: "email or password is incorrect" } }),
    });

    render(<AuthProvider><Probe /></AuthProvider>);
    await userEvent.click(screen.getByText("login"));

    await waitFor(() => expect(screen.getByTestId("token")).toHaveTextContent("none"));
  });

  it("clears the token and user on logout", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({
        accessToken: "token-123",
        user: { id: "u1", email: "operator@example.com", role: "operator" },
      }),
    });

    render(<AuthProvider><Probe /></AuthProvider>);
    await userEvent.click(screen.getByText("login"));
    await waitFor(() => expect(screen.getByTestId("token")).toHaveTextContent("token-123"));

    await userEvent.click(screen.getByText("logout"));
    expect(screen.getByTestId("token")).toHaveTextContent("none");
    expect(screen.getByTestId("role")).toHaveTextContent("none");
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run src/app/AuthContext.test.tsx`
Expected: FAIL — module `@/app/AuthContext` not found. (If `@testing-library/user-event` is not yet a dependency, the test fails to resolve it; add it in this step: `npm install -D @testing-library/user-event@^14`.)

- [ ] **Step 3: Implement AuthContext.tsx**

Create `apps/web/src/app/AuthContext.tsx`:
```tsx
import { createContext, useCallback, useContext, useState, type ReactNode } from "react";
import { env } from "@/lib/env";

export interface AuthUser {
  id: string;
  email: string;
  role: "operator" | "admin";
}

interface AuthContextValue {
  /** Held in memory only — never written to localStorage or sessionStorage. */
  accessToken: string | null;
  user: AuthUser | null;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

/** AuthProvider holds the access token and current user in memory only. */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [user, setUser] = useState<AuthUser | null>(null);

  const login = useCallback(async (email: string, password: string) => {
    const res = await fetch(`${env.apiUrl}/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => null);
      throw new Error(body?.error?.message ?? "Login failed");
    }
    const data = await res.json();
    setAccessToken(data.accessToken);
    setUser(data.user);
  }, []);

  const logout = useCallback(() => {
    setAccessToken(null);
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider value={{ accessToken, user, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

/** useAuth reads the auth state; must be used within an AuthProvider. */
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && npx vitest run src/app/AuthContext.test.tsx`
Expected: PASS, 3 tests.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/app/AuthContext.tsx apps/web/src/app/AuthContext.test.tsx apps/web/package.json apps/web/package-lock.json
git commit -m "feat(web): add in-memory AuthContext with login/logout"
```

---

## Task 14: ProtectedRoute guard

**Files:**
- Create: `apps/web/src/app/ProtectedRoute.tsx`
- Test: `apps/web/src/app/ProtectedRoute.test.tsx`

**Interfaces:**
- Consumes: `useAuth()` (Task 13).
- Produces: `ProtectedRoute` component (a route-element wrapper rendering `<Outlet/>` when authenticated, `<Navigate to="/login"/>` otherwise). Consumed by Task 17 (`routes.tsx`).

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/app/ProtectedRoute.test.tsx`:
```tsx
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { AuthProvider } from "@/app/AuthContext";
import { ProtectedRoute } from "@/app/ProtectedRoute";

function renderProtected(initialPath: string) {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/login" element={<div>login page</div>} />
          <Route element={<ProtectedRoute />}>
            <Route path="/dashboard" element={<div>dashboard page</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  );
}

describe("ProtectedRoute", () => {
  it("redirects to /login when there is no access token", () => {
    renderProtected("/dashboard");
    expect(screen.getByText("login page")).toBeInTheDocument();
    expect(screen.queryByText("dashboard page")).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run src/app/ProtectedRoute.test.tsx`
Expected: FAIL — module `@/app/ProtectedRoute` not found.

- [ ] **Step 3: Implement ProtectedRoute.tsx**

Create `apps/web/src/app/ProtectedRoute.tsx`:
```tsx
import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "@/app/AuthContext";

/**
 * ProtectedRoute renders its nested routes only when an access token is
 * present; otherwise it redirects to /login. Reactivity comes from
 * useAuth() — clearing the token (e.g. on a 401) re-renders this guard.
 */
export function ProtectedRoute() {
  const { accessToken } = useAuth();
  if (!accessToken) {
    return <Navigate to="/login" replace />;
  }
  return <Outlet />;
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && npx vitest run src/app/ProtectedRoute.test.tsx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/app/ProtectedRoute.tsx apps/web/src/app/ProtectedRoute.test.tsx
git commit -m "feat(web): add ProtectedRoute guard that redirects to /login"
```

---

## Task 15: useApiFetch — attach token, clear on 401

**Files:**
- Create: `apps/web/src/app/useApiFetch.ts`
- Test: `apps/web/src/app/useApiFetch.test.tsx`

**Interfaces:**
- Consumes: `useAuth()` (Task 13), `env.apiUrl`.
- Produces: `useApiFetch()` hook returning `(path: string, init?: RequestInit) => Promise<Response>` that attaches `Authorization: Bearer <token>` when present and calls `logout()` on a 401 response. This is the mechanism that makes `ProtectedRoute` (Task 14) redirect to `/login` after a 401: clearing the token via `logout()` triggers a re-render of the guard. Available for future protected-data-fetching pages; not consumed by any page in this issue beyond its own test.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/app/useApiFetch.test.tsx`:
```tsx
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { AuthProvider, useAuth } from "@/app/AuthContext";
import { useApiFetch } from "@/app/useApiFetch";
import type { ReactNode } from "react";

function wrapper({ children }: { children: ReactNode }) {
  return <AuthProvider>{children}</AuthProvider>;
}

describe("useApiFetch", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("attaches the bearer token when one is present", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({ ok: true, status: 200, json: async () => ({}) });
    const { result } = renderHook(
      () => ({ auth: useAuth(), apiFetch: useApiFetch() }),
      { wrapper },
    );

    (fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ accessToken: "token-123", user: { id: "u1", email: "a@example.com", role: "operator" } }),
    });
    await act(async () => {
      await result.current.auth.login("a@example.com", "correct-password");
    });

    await act(async () => {
      await result.current.apiFetch("/transactions");
    });

    const lastCall = (fetch as ReturnType<typeof vi.fn>).mock.calls.at(-1);
    expect(lastCall?.[1]?.headers).toMatchObject({ Authorization: "Bearer token-123" });
  });

  it("clears the token when a request returns 401", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ accessToken: "token-123", user: { id: "u1", email: "a@example.com", role: "operator" } }),
    });
    const { result } = renderHook(
      () => ({ auth: useAuth(), apiFetch: useApiFetch() }),
      { wrapper },
    );
    await act(async () => {
      await result.current.auth.login("a@example.com", "correct-password");
    });
    expect(result.current.auth.accessToken).toBe("token-123");

    (fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ ok: false, status: 401, json: async () => ({}) });
    await act(async () => {
      await result.current.apiFetch("/transactions");
    });

    expect(result.current.auth.accessToken).toBeNull();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run src/app/useApiFetch.test.tsx`
Expected: FAIL — module `@/app/useApiFetch` not found.

- [ ] **Step 3: Implement useApiFetch.ts**

Create `apps/web/src/app/useApiFetch.ts`:
```ts
import { useCallback } from "react";
import { useAuth } from "@/app/AuthContext";
import { env } from "@/lib/env";

/**
 * useApiFetch wraps fetch with the current access token and, on a 401
 * response, logs out — which clears the in-memory token and lets
 * ProtectedRoute redirect to /login on the next render.
 */
export function useApiFetch() {
  const { accessToken, logout } = useAuth();

  return useCallback(
    async (path: string, init: RequestInit = {}) => {
      const headers = {
        ...init.headers,
        ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
      };
      const res = await fetch(`${env.apiUrl}${path}`, { ...init, headers });
      if (res.status === 401) {
        logout();
      }
      return res;
    },
    [accessToken, logout],
  );
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && npx vitest run src/app/useApiFetch.test.tsx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/app/useApiFetch.ts apps/web/src/app/useApiFetch.test.tsx
git commit -m "feat(web): add useApiFetch that clears the token on 401"
```

---

## Task 16: Wire LoginPage to POST /login

**Files:**
- Modify: `apps/web/src/pages/LoginPage.tsx`
- Create: `apps/web/src/pages/LoginPage.test.tsx`

**Interfaces:**
- Consumes: `useAuth()` (Task 13), `useNavigate` (react-router-dom, existing dependency).

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/pages/LoginPage.test.tsx`:
```tsx
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { AuthProvider } from "@/app/AuthContext";
import { LoginPage } from "@/pages/LoginPage";

function renderLoginPage() {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={["/login"]}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/dashboard" element={<div>dashboard page</div>} />
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  );
}

describe("LoginPage", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("navigates to /dashboard on successful submit", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({
        accessToken: "token-123",
        user: { id: "u1", email: "operator@example.com", role: "operator" },
      }),
    });

    renderLoginPage();
    await userEvent.type(screen.getByLabelText(/email/i), "operator@example.com");
    await userEvent.type(screen.getByLabelText(/password/i), "correct-password");
    await userEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => expect(screen.getByText("dashboard page")).toBeInTheDocument());
  });

  it("shows an error message on invalid credentials", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: false,
      json: async () => ({ error: { code: "INVALID_CREDENTIALS", message: "email or password is incorrect" } }),
    });

    renderLoginPage();
    await userEvent.type(screen.getByLabelText(/email/i), "operator@example.com");
    await userEvent.type(screen.getByLabelText(/password/i), "wrong-password");
    await userEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => expect(screen.getByText(/email or password is incorrect/i)).toBeInTheDocument());
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run src/pages/LoginPage.test.tsx`
Expected: FAIL — the form fields are `disabled` and submit does nothing, so neither assertion is met.

- [ ] **Step 3: Implement the wired LoginPage**

Replace the contents of `apps/web/src/pages/LoginPage.tsx`:
```tsx
import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/app/AuthContext";
import { Button } from "@/components/ui/button";

/** LoginPage authenticates against POST /login and stores the token in memory. */
export function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login(email, password);
      navigate("/dashboard", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-6 p-6">
      <div className="space-y-1 text-center">
        <h1 className="text-2xl font-semibold tracking-tight">Sign in</h1>
        <p className="text-sm text-muted-foreground">FinWatch operator access.</p>
      </div>
      <form className="space-y-4" onSubmit={handleSubmit} aria-label="Sign in">
        <div className="space-y-1">
          <label htmlFor="email" className="text-sm font-medium">
            Email
          </label>
          <input
            id="email"
            type="email"
            autoComplete="username"
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            placeholder="operator@example.com"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            required
          />
        </div>
        <div className="space-y-1">
          <label htmlFor="password" className="text-sm font-medium">
            Password
          </label>
          <input
            id="password"
            type="password"
            autoComplete="current-password"
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            required
          />
        </div>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <Button type="submit" className="w-full" disabled={submitting}>
          {submitting ? "Signing in…" : "Sign in"}
        </Button>
      </form>
    </main>
  );
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && npx vitest run src/pages/LoginPage.test.tsx`
Expected: PASS, 2 tests.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/pages/LoginPage.tsx apps/web/src/pages/LoginPage.test.tsx
git commit -m "feat(web): wire LoginPage to POST /login"
```

---

## Task 17: Wrap the app in AuthProvider and gate authenticated routes

**Files:**
- Modify: `apps/web/src/app/App.tsx`
- Modify: `apps/web/src/app/routes.tsx`
- Modify: `apps/web/src/app/routes.test.tsx`

**Interfaces:**
- Consumes: `AuthProvider` (Task 13), `ProtectedRoute` (Task 14).

- [ ] **Step 1: Update the existing routes test to authenticate before asserting protected pages render**

Replace the contents of `apps/web/src/app/routes.test.tsx`:
```tsx
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AuthProvider, useAuth } from "@/app/AuthContext";
import { AppRoutes } from "@/app/routes";

function renderAt(path: string) {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={[path]}>
        <AppRoutes />
      </MemoryRouter>
    </AuthProvider>,
  );
}

describe("AppRoutes", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("redirects unauthenticated requests for the dashboard to /login", () => {
    renderAt("/dashboard");
    expect(screen.getByRole("heading", { name: /sign in/i })).toBeInTheDocument();
  });

  it("renders the dashboard with primary navigation once authenticated", async () => {
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({
        accessToken: "token-123",
        user: { id: "u1", email: "operator@example.com", role: "operator" },
      }),
    });

    function Authed() {
      const { login } = useAuth();
      return (
        <button
          onClick={() => {
            void login("operator@example.com", "correct-password");
          }}
        >
          do-login
        </button>
      );
    }

    render(
      <AuthProvider>
        <Authed />
        <MemoryRouter initialEntries={["/dashboard"]}>
          <AppRoutes />
        </MemoryRouter>
      </AuthProvider>,
    );

    screen.getByText("do-login").click();
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: /dashboard/i })).toBeInTheDocument(),
    );
    expect(screen.getByRole("navigation", { name: /primary/i })).toBeInTheDocument();
  });

  it("renders a not-found page for unknown routes", () => {
    renderAt("/nope");
    expect(screen.getByText("404")).toBeInTheDocument();
  });

  it("renders the login page outside the app shell", () => {
    renderAt("/login");
    expect(screen.getByRole("heading", { name: /sign in/i })).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npx vitest run src/app/routes.test.tsx`
Expected: FAIL — `/dashboard` currently renders the dashboard unconditionally (no guard yet), so the first new assertion (redirect to sign in) fails.

- [ ] **Step 3: Add ProtectedRoute to the route table**

In `apps/web/src/app/routes.tsx`, add the import and wrap the `AppShell` route:
```tsx
import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "@/app/AppShell";
import { ProtectedRoute } from "@/app/ProtectedRoute";
import { DashboardPage } from "@/pages/DashboardPage";
import { AlertsPage } from "@/pages/AlertsPage";
import { OpsPage } from "@/pages/OpsPage";
import { LoginPage } from "@/pages/LoginPage";
import { NotFoundPage } from "@/pages/NotFoundPage";

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<AppShell />}>
          <Route index element={<Navigate to="/dashboard" replace />} />
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/alerts" element={<AlertsPage />} />
          <Route path="/ops" element={<OpsPage />} />
        </Route>
      </Route>
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  );
}
```

- [ ] **Step 4: Wrap App.tsx in AuthProvider**

In `apps/web/src/app/App.tsx`:
```tsx
import { BrowserRouter } from "react-router-dom";
import { QueryClientProvider } from "@tanstack/react-query";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { queryClient } from "@/lib/queryClient";
import { AuthProvider } from "@/app/AuthContext";
import { AppRoutes } from "@/app/routes";

/** App composes global providers, the error boundary, and the router. */
export function App() {
  return (
    <ErrorBoundary>
      <AuthProvider>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <AppRoutes />
          </BrowserRouter>
        </QueryClientProvider>
      </AuthProvider>
    </ErrorBoundary>
  );
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `cd apps/web && npx vitest run src/app/routes.test.tsx`
Expected: PASS, all 4 tests.

- [ ] **Step 6: Run the full frontend test suite and type-check**

Run: `cd apps/web && npm run test && npm run typecheck && npm run lint`
Expected: all green — this exercises every test added in Tasks 13-17 together plus the previously existing suite.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/app/App.tsx apps/web/src/app/routes.tsx apps/web/src/app/routes.test.tsx
git commit -m "feat(web): gate authenticated routes behind ProtectedRoute"
```

---

## Test Strategy Summary

| Layer | File | Cases |
|---|---|---|
| Password hashing | `internal/auth/password_test.go` | round-trip, wrong password |
| JWT issue/verify | `internal/auth/jwt_test.go` | round-trip, expired, wrong signature, malformed |
| Login service | `internal/auth/service_test.go` | valid login, wrong password, unknown email |
| Middleware | `internal/auth/middleware_test.go` | missing/malformed/expired/valid token, operator denied from admin route (403), admin allowed |
| Config | `internal/config/config_test.go` | missing secret, short secret, non-duration TTL, zero TTL, default 15m, override |
| HTTP handlers | `internal/auth/httpapi/handler_test.go` | login success/wrong-password/missing-fields, /me with valid token, /me without token |
| Router | `internal/platform/httpserver/router_auth_test.go` | public module bypasses RequireAuth, protected module runs behind it |
| Frontend context | `app/AuthContext.test.tsx` | login stores token/user, failed login leaves state empty, logout clears state |
| Frontend guard | `app/ProtectedRoute.test.tsx` | unauthenticated redirect to /login |
| Frontend fetch wrapper | `app/useApiFetch.test.tsx` | attaches bearer token, clears token on 401 |
| Frontend login page | `pages/LoginPage.test.tsx` | successful submit navigates to /dashboard, invalid login shows error |
| Frontend routing | `app/routes.test.tsx` | unauthenticated redirect, authenticated dashboard render, 404, public login page |

No test against a live Postgres database is included: `internal/auth/store` is a thin adapter exercised through `go build`/`go vet` and, optionally, manual verification with `make dev` + `go run ./cmd/api seed-users` + a real `curl` login (see Verification Commands). This mirrors the existing `internal/alerts/store` package, which has no dedicated test file either.

## Security Risks and Mitigations

- **Signing secret strength.** `JWT_SIGNING_SECRET` must be at least 32 characters (enforced in `Config.Validate`); the example value in `.env.example` is clearly labelled as dev-only and must never be reused in any real deployment.
- **Credential enumeration.** `Service.Login` returns the same `ErrInvalidCredentials` for both "unknown email" and "wrong password" so the API cannot be used to enumerate registered emails.
- **Password storage.** Passwords are bcrypt-hashed (`bcrypt.DefaultCost`) before storage; plaintext passwords are never logged (`runSeedUsers` logs only email and a `created` boolean).
- **Token lifetime.** A 15-minute access-token TTL bounds the exposure window of a leaked token; there is no refresh token in this issue, so a logged-in session simply expires after 15 minutes (acceptable per the explicit non-goals — refresh is future work).
- **Token storage on the frontend.** The access token lives only in a React context's in-memory state; it is never written to `localStorage` or `sessionStorage`, so it does not survive a page reload and is not readable by a separately-injected script that only has DOM/storage access (XSS via direct memory access is a different, harder threat that this design does not claim to fully close).
- **Algorithm confusion.** `Verifier.Verify` pins `jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name})`, rejecting tokens signed with `none` or an asymmetric algorithm.
- **Role escalation.** Role is only ever read from a verified JWT's claims (`ClaimsFromContext`), never from client-supplied request bodies or query parameters.

## Known Limitations / Non-Goals (carried into follow-up issues)

- No refresh tokens — a 401 after 15 minutes requires logging in again. (Explicitly out of scope.)
- No OAuth/SAML, password reset, or production user-management UI. (Explicitly out of scope.)
- `RequireRole` is implemented and unit-tested but not yet mounted on any concrete business route, since no admin-only endpoint exists in the current API surface (adding one is alert/transaction-workflow scope, which this issue does not touch).
- WebSocket authentication is issue #5's scope.
- `sessionStorage` is also not used in this issue, per the explicit instruction — token loss on page refresh is accepted behavior for now.

## Verification Commands

Run after every task, and definitely before the final commit:
```bash
cd apps/api && gofmt -l . && go vet ./... && go test -p 1 ./... && go build ./...
cd apps/web && npm run lint && npm run typecheck && npm run test && npm run build
```
Or, from the repo root:
```bash
make verify
```

Manual end-to-end check (requires Docker):
```bash
make dev
go run ./apps/api/cmd/api seed-users   # or: docker compose exec api /app/api seed-users
curl -s -X POST http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"operator@example.com","password":"operator_dev_password"}'
# Expect: {"accessToken":"...","user":{"id":"...","email":"operator@example.com","role":"operator"}}
curl -s http://localhost:8080/transactions
# Expect: 401 Unauthorized envelope (no token)
curl -s http://localhost:8080/transactions -H "Authorization: Bearer <accessToken from above>"
# Expect: 200 with a transaction page
```

## Step-by-Step Checklist

- [ ] Task 1: Add JWT and bcrypt dependencies
- [ ] Task 2: Users migration and sqlc query file
- [ ] Task 3: Config — JWT signing secret and access token TTL
- [ ] Task 4: Auth domain — claims, password hashing, JWT issue/verify
- [ ] Task 5: Login service
- [ ] Task 6: Auth HTTP middleware (RequireAuth, RequireRole)
- [ ] Task 7: Postgres user store
- [ ] Task 8: Auth HTTP handlers (POST /login, GET /me)
- [ ] Task 9: Router — public/protected route split
- [ ] Task 10: Wire auth into cmd/api, add seed-users command
- [ ] Task 11: OpenAPI contract — bearerAuth, /login, /me, 401/403
- [ ] Task 12: Environment and Compose wiring
- [ ] Task 13: AuthContext — in-memory token store
- [ ] Task 14: ProtectedRoute guard
- [ ] Task 15: useApiFetch — attach token, clear on 401
- [ ] Task 16: Wire LoginPage to POST /login
- [ ] Task 17: Wrap the app in AuthProvider and gate authenticated routes
- [ ] Final: run `make verify` end-to-end; manually verify login/me/protected-route flow with `make dev`
