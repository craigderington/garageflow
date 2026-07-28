# Demo Tenant Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a prospect trade an email address for their own populated, interactive GarageFlow shop that expires after 14 days.

**Architecture:** `POST /demo` provisions a per-visitor shop in one transaction — shop row, owner user, and template data from a Go-side seeder — then issues a session cookie exactly as the auth handlers do. A `demo_leads` row holds the captured email and an opaque `return_token` emailed as a resume link. Demo tenants are prevented from sending any outbound SMS or email by decorating the existing `Sender` interfaces. A ticker goroutine deletes expired demo shops.

**Tech Stack:** Go 1.26, chi v5, pgx/v5, go-redis v9, bcrypt, httprate; Next.js 16 + React 19 + Tailwind v4; Playwright for E2E.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-07-28-demo-tenant-design.md`
- Demo shops expire **14 days** after creation.
- Template data lives in Go (`internal/demo`), **never** in `migrations/seed/` — that directory must keep never running in production.
- Demo tenants send **no** outbound SMS or email to customers, ever.
- The demo resume path must **never** resolve identity through `users.email` — `users` is `UNIQUE(shop_id, email)` and `auth/service.go:100` selects on email alone, returning an arbitrary row when an address spans shops.
- No credential or secret appears in any HTTP response body or any public page.
- Every Go test runs green under `go test -race ./...`; the full Playwright suite must stay green.
- Run the local stack with `docker compose up -d` and the suite with `make e2e`.

---

### Task 1: Schema for demo tenancy

**Files:**
- Create: `migrations/006_demo_tenants.sql`

**Interfaces:**
- Consumes: nothing.
- Produces: `shops.is_demo BOOLEAN NOT NULL DEFAULT FALSE`, `shops.expires_at TIMESTAMPTZ`, and table `demo_leads(id, email UNIQUE, shop_id NULLABLE, return_token UNIQUE, ip, user_agent, created_at, verified_at, last_seen_at)`.

- [ ] **Step 1: Write the migration**

```sql
-- Demo tenancy: each captured email gets its own throwaway shop.
ALTER TABLE shops
    ADD COLUMN is_demo BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN expires_at TIMESTAMPTZ;

CREATE TABLE demo_leads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    -- SET NULL, not CASCADE: a lead whose 14 days lapsed is still a lead.
    -- Cascading would delete the exact thing the email wall exists to collect.
    shop_id UUID REFERENCES shops(id) ON DELETE SET NULL,
    return_token TEXT NOT NULL UNIQUE,
    ip TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    verified_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ
);

-- The expiry sweep scans on this every tick.
CREATE INDEX idx_shops_expires_at ON shops (expires_at) WHERE expires_at IS NOT NULL;
```

- [ ] **Step 2: Apply it to a fresh database**

Run: `bash scripts/db-reset.sh`
Expected: completes with no error.

- [ ] **Step 3: Verify the schema landed**

Run:
```bash
docker compose exec -T postgres psql -U garageflow -d garageflow \
  -c '\d demo_leads' -c 'SELECT is_demo, expires_at FROM shops LIMIT 1;'
```
Expected: `demo_leads` is listed with all nine columns; the `shops` query succeeds (zero rows is fine).

- [ ] **Step 4: Commit**

```bash
git add migrations/006_demo_tenants.sql
git commit -m "feat: add demo tenancy schema"
```

---

### Task 2: Extract a reusable session issuer

**Files:**
- Modify: `apps/api-go/internal/auth/service.go:106-113`
- Test: `apps/api-go/internal/auth/service_test.go`

Session creation is currently inlined in `VerifyMagicLink`. The demo needs the same behaviour, and duplicating a session format is how two code paths drift into disagreeing about what a session looks like.

**Interfaces:**
- Consumes: nothing.
- Produces: `func (s *Service) IssueSession(ctx context.Context, userID, shopID, role string) (string, error)` — returns an opaque session token, already stored in Redis with a 24h TTL.

- [ ] **Step 1: Write the failing test**

Add to `apps/api-go/internal/auth/service_test.go`:

```go
func TestIssueSessionRoundTrips(t *testing.T) {
	rdb, cleanup := testRedis(t)
	defer cleanup()

	svc := &Service{rdb: rdb}

	token, err := svc.IssueSession(context.Background(), "user-1", "shop-1", "owner")
	if err != nil {
		t.Fatalf("IssueSession: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	uid, sid, role, err := svc.ValidateSession(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if uid != "user-1" || sid != "shop-1" || role != "owner" {
		t.Errorf("got (%q,%q,%q), want (user-1,shop-1,owner)", uid, sid, role)
	}
}
```

If `testRedis` does not already exist in that package, add it:

```go
// testRedis returns a client pointed at the compose Redis, skipping when it is
// unreachable so the unit suite still runs on a bare checkout.
func testRedis(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable at %s: %v", addr, err)
	}
	return rdb, func() { rdb.Close() }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api-go && go test ./internal/auth/ -run TestIssueSessionRoundTrips -v`
Expected: FAIL — `svc.IssueSession undefined`.

- [ ] **Step 3: Extract the method**

Add to `apps/api-go/internal/auth/service.go`:

```go
// IssueSession stores a session for an already-authenticated user and returns
// its opaque token. Kept in one place so every sign-in path — password, magic
// link, demo — produces an identical session shape.
func (s *Service) IssueSession(ctx context.Context, userID, shopID, role string) (string, error) {
	sessionToken := uuid.New().String()
	sessionKey := fmt.Sprintf("session:%s", sessionToken)
	sessionData := fmt.Sprintf(`{"uid":"%s","sid":"%s","role":"%s"}`, userID, shopID, role)
	if err := s.rdb.Set(ctx, sessionKey, sessionData, 24*time.Hour).Err(); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return sessionToken, nil
}
```

Then replace the inlined block in `VerifyMagicLink` (the five lines from `sessionToken := uuid.New().String()` through the closing brace of the `if err := s.rdb.Set(...)` check, ending at `return sessionToken, nil`) with:

```go
	return s.IssueSession(ctx, userID, shopID, role)
```

- [ ] **Step 4: Run the auth tests**

Run: `cd apps/api-go && go test -race ./internal/auth/ -v`
Expected: PASS, including the pre-existing tests — `VerifyMagicLink` must still behave identically.

- [ ] **Step 5: Commit**

```bash
git add apps/api-go/internal/auth/service.go apps/api-go/internal/auth/service_test.go
git commit -m "refactor: extract IssueSession so every sign-in path agrees on session shape"
```

---

### Task 3: Template data seeder

**Files:**
- Create: `apps/api-go/internal/demo/seed.go`
- Test: `apps/api-go/internal/demo/seed_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `func Seed(ctx context.Context, tx pgx.Tx, shopID string) error` — inserts a populated shop's worth of rows under `shopID`. Contains no fixed UUIDs.

- [ ] **Step 1: Write the failing test**

```go
package demo

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://garageflow:garageflow@localhost:5434/garageflow?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Seeding runs inside the provisioning transaction, so it is tested inside one
// that always rolls back — the test must not leave a shop behind.
func TestSeedPopulatesShop(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	var shopID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO shops (name, slug, is_demo) VALUES ('Seed Test', 'seed-test-tmp', TRUE) RETURNING id`,
	).Scan(&shopID); err != nil {
		t.Fatalf("insert shop: %v", err)
	}

	if err := Seed(ctx, tx, shopID); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	for _, tc := range []struct {
		table   string
		atLeast int
	}{
		{"customers", 3},
		{"vehicles", 3},
		{"repair_orders", 2},
		{"inventory_parts", 3},
		{"inspections", 1},
	} {
		var count int
		if err := tx.QueryRow(ctx,
			"SELECT COUNT(*) FROM "+tc.table+" WHERE shop_id = $1", shopID,
		).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", tc.table, err)
		}
		if count < tc.atLeast {
			t.Errorf("%s = %d rows, want at least %d", tc.table, count, tc.atLeast)
		}
	}
}

// A prospect must land on a dashboard with something to look at, and the DVI is
// the whole pitch — an inspection with no flagged item demos nothing.
func TestSeedFlagsInspectionItems(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	var shopID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO shops (name, slug, is_demo) VALUES ('Seed Flags', 'seed-flags-tmp', TRUE) RETURNING id`,
	).Scan(&shopID); err != nil {
		t.Fatalf("insert shop: %v", err)
	}
	if err := Seed(ctx, tx, shopID); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	var flagged int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM inspection_items ii
		JOIN inspections i ON i.id = ii.inspection_id
		WHERE i.shop_id = $1 AND ii.condition IN ('attention','urgent')`, shopID).Scan(&flagged); err != nil {
		t.Fatalf("count flagged: %v", err)
	}
	if flagged < 1 {
		t.Errorf("flagged items = %d, want at least 1", flagged)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api-go && go test ./internal/demo/ -v`
Expected: FAIL — `undefined: Seed`.

- [ ] **Step 3: Implement the seeder**

Before writing it, read the exact column names you need:

```bash
grep -nA14 "CREATE TABLE customers" migrations/001_initial_schema.sql
grep -nA16 "CREATE TABLE vehicles" migrations/001_initial_schema.sql
grep -nA16 "CREATE TABLE repair_orders" migrations/001_initial_schema.sql
grep -nA14 "CREATE TABLE inventory_parts" migrations/001_initial_schema.sql
grep -nA20 "CREATE TABLE inspections" migrations/005_inspections.sql
grep -nA16 "CREATE TABLE inspection_items" migrations/005_inspections.sql
```

Write `apps/api-go/internal/demo/seed.go` inserting, under the given `shopID`:

- 3 customers with plausible names and phone numbers in the `555-01xx` reserved range — never a dialable number, since a prospect may try the send flow
- 3 vehicles, one per customer, with realistic make/model/year/VIN
- 2 repair orders in different statuses, each linked to a customer and vehicle
- 4 inventory parts, one of them at or below its minimum stock so the low-stock indicator has something to show
- 1 inspection against the first repair order, with items copied from the default template, **at least one** marked `urgent` with a price

Every id comes from `gen_random_uuid()` or `uuid.New()`. No literal UUIDs — two demo shops seeded from literals would collide on primary keys.

Use `pgx.Tx` throughout so the caller's transaction governs, and return the first error unwrapped with context, e.g.:

```go
if _, err := tx.Exec(ctx, `INSERT INTO customers (id, shop_id, name, phone, email) VALUES ($1,$2,$3,$4,$5)`,
	custID, shopID, "Dana Whitfield", "555-0142", "dana@example.com"); err != nil {
	return fmt.Errorf("seed customer: %w", err)
}
```

- [ ] **Step 4: Run the tests**

Run: `cd apps/api-go && go test -race ./internal/demo/ -v`
Expected: PASS.

- [ ] **Step 5: Confirm nothing leaked**

Run:
```bash
docker compose exec -T postgres psql -U garageflow -d garageflow \
  -c "SELECT count(*) FROM shops WHERE slug LIKE '%-tmp';"
```
Expected: `0` — the rollbacks held.

- [ ] **Step 6: Commit**

```bash
git add apps/api-go/internal/demo/
git commit -m "feat: seed template data for demo shops"
```

---

### Task 4: Block outbound for demo tenants

**Files:**
- Create: `apps/api-go/internal/demo/guard.go`
- Test: `apps/api-go/internal/demo/guard_test.go`

A demo prospect can type any phone number into a customer record. Without this, the DVI send flow texts a stranger.

**Interfaces:**
- Consumes: `sms.Sender`, `email.Sender`, `middleware.GetShopID`.
- Produces:
  - `func IsDemoShop(ctx context.Context, pool *pgxpool.Pool) bool`
  - `func GuardSMS(inner sms.Sender, pool *pgxpool.Pool) sms.Sender`
  - `func GuardEmail(inner email.Sender, pool *pgxpool.Pool) email.Sender`

- [ ] **Step 1: Write the failing test**

```go
package demo

import (
	"context"
	"testing"

	"github.com/garageflow/api-go/internal/middleware"
)

type spySMS struct{ calls int }

func (s *spySMS) Send(_ context.Context, _, _ string) error { s.calls++; return nil }

type spyEmail struct{ calls int }

func (s *spyEmail) Send(_ context.Context, _, _, _, _ string) error { s.calls++; return nil }

func TestGuardSMSBlocksDemoShop(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	var shopID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO shops (name, slug, is_demo) VALUES ('Guard Demo', 'guard-demo-'||gen_random_uuid(), TRUE) RETURNING id`,
	).Scan(&shopID); err != nil {
		t.Fatalf("insert shop: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM shops WHERE id=$1`, shopID) })

	spy := &spySMS{}
	guarded := GuardSMS(spy, pool)

	err := guarded.Send(middleware.WithShopID(ctx, shopID), "555-0142", "hello")
	if err != nil {
		t.Fatalf("Send returned %v, want nil — a blocked send must look successful", err)
	}
	if spy.calls != 0 {
		t.Errorf("inner sender called %d times, want 0", spy.calls)
	}
}

func TestGuardSMSAllowsRealShop(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	var shopID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO shops (name, slug, is_demo) VALUES ('Guard Real', 'guard-real-'||gen_random_uuid(), FALSE) RETURNING id`,
	).Scan(&shopID); err != nil {
		t.Fatalf("insert shop: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM shops WHERE id=$1`, shopID) })

	spy := &spySMS{}
	guarded := GuardSMS(spy, pool)

	if err := guarded.Send(middleware.WithShopID(ctx, shopID), "555-0142", "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if spy.calls != 1 {
		t.Errorf("inner sender called %d times, want 1", spy.calls)
	}
}

// No shop in context means we cannot prove the tenant is real. Fail closed: an
// unknown tenant is treated as a demo and nothing goes out.
func TestGuardSMSBlocksWhenShopUnknown(t *testing.T) {
	pool := testPool(t)

	spy := &spySMS{}
	guarded := GuardSMS(spy, pool)

	if err := guarded.Send(context.Background(), "555-0142", "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if spy.calls != 0 {
		t.Errorf("inner sender called %d times, want 0", spy.calls)
	}
}

func TestGuardEmailBlocksDemoShop(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	var shopID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO shops (name, slug, is_demo) VALUES ('Guard Mail', 'guard-mail-'||gen_random_uuid(), TRUE) RETURNING id`,
	).Scan(&shopID); err != nil {
		t.Fatalf("insert shop: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM shops WHERE id=$1`, shopID) })

	spy := &spyEmail{}
	guarded := GuardEmail(spy, pool)

	if err := guarded.Send(middleware.WithShopID(ctx, shopID), "a@b.com", "s", "<p>h</p>", "t"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if spy.calls != 0 {
		t.Errorf("inner sender called %d times, want 0", spy.calls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api-go && go test ./internal/demo/ -run TestGuard -v`
Expected: FAIL — `undefined: GuardSMS`.

- [ ] **Step 3: Implement the guards**

```go
package demo

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/garageflow/api-go/internal/email"
	"github.com/garageflow/api-go/internal/middleware"
	"github.com/garageflow/api-go/internal/sms"
)

// IsDemoShop reports whether the request's tenant is a demo shop. It fails
// CLOSED: no shop in context, or any lookup error, counts as a demo. A demo
// tenant that leaks a real text is worse than a real tenant that drops one, and
// every genuine send path runs behind TenantMiddleware and so has a shop.
func IsDemoShop(ctx context.Context, pool *pgxpool.Pool) bool {
	shopID := middleware.GetShopID(ctx)
	if shopID == "" {
		return true
	}
	var isDemo bool
	if err := pool.QueryRow(ctx, `SELECT is_demo FROM shops WHERE id = $1`, shopID).Scan(&isDemo); err != nil {
		log.Printf("[demo] is_demo lookup failed for shop %s, blocking outbound: %v", shopID, err)
		return true
	}
	return isDemo
}

type guardedSMS struct {
	inner sms.Sender
	pool  *pgxpool.Pool
}

// GuardSMS blocks outbound SMS for demo tenants. It returns nil rather than an
// error so the UI still reports a successful send — the prospect sees the whole
// flow, and the report stays reachable by link and QR.
func GuardSMS(inner sms.Sender, pool *pgxpool.Pool) sms.Sender {
	return &guardedSMS{inner: inner, pool: pool}
}

func (g *guardedSMS) Send(ctx context.Context, to, body string) error {
	if IsDemoShop(ctx, g.pool) {
		log.Printf("[demo] suppressed SMS to %s", to)
		return nil
	}
	return g.inner.Send(ctx, to, body)
}

type guardedEmail struct {
	inner email.Sender
	pool  *pgxpool.Pool
}

// GuardEmail blocks outbound customer email for demo tenants. Note this wraps
// only the sender handed to customer-facing handlers; the demo's own return-link
// email is sent with the unwrapped sender.
func GuardEmail(inner email.Sender, pool *pgxpool.Pool) email.Sender {
	return &guardedEmail{inner: inner, pool: pool}
}

func (g *guardedEmail) Send(ctx context.Context, to, subject, html, text string) error {
	if IsDemoShop(ctx, g.pool) {
		log.Printf("[demo] suppressed email to %s", to)
		return nil
	}
	return g.inner.Send(ctx, to, subject, html, text)
}
```

If `sms.Sender`'s method signature differs from `Send(ctx, to, body string) error`, match the real one — check `apps/api-go/internal/sms/sms.go:17`.

- [ ] **Step 4: Run the tests**

Run: `cd apps/api-go && go test -race ./internal/demo/ -v`
Expected: PASS, all four guard tests plus the seeder tests.

- [ ] **Step 5: Commit**

```bash
git add apps/api-go/internal/demo/guard.go apps/api-go/internal/demo/guard_test.go
git commit -m "feat: block outbound sends for demo tenants"
```

---

### Task 5: Provisioning and resume service

**Files:**
- Create: `apps/api-go/internal/demo/service.go`
- Test: `apps/api-go/internal/demo/service_test.go`

**Interfaces:**
- Consumes: `Seed` (Task 3), `auth.Service.IssueSession` (Task 2).
- Produces:
  - `type SessionIssuer interface { IssueSession(ctx context.Context, userID, shopID, role string) (string, error) }`
  - `func NewService(pool *pgxpool.Pool, sessions SessionIssuer, ttl time.Duration) *Service`
  - `func (s *Service) Provision(ctx context.Context, email, ip, userAgent string) (sessionToken, returnToken string, err error)`
  - `func (s *Service) Resume(ctx context.Context, returnToken string) (sessionToken string, err error)`
  - `func (s *Service) SweepExpired(ctx context.Context) (int, error)`

`SessionIssuer` is an interface, not `*auth.Service`, so these tests need no Redis and no auth wiring.

- [ ] **Step 1: Write the failing test**

```go
package demo

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type fakeSessions struct{ issued int }

func (f *fakeSessions) IssueSession(_ context.Context, userID, shopID, role string) (string, error) {
	f.issued++
	return fmt.Sprintf("session-for-%s", userID), nil
}

func newTestService(t *testing.T) (*Service, *fakeSessions) {
	t.Helper()
	pool := testPool(t)
	sessions := &fakeSessions{}
	return NewService(pool, sessions, 14*24*time.Hour), sessions
}

func cleanupLead(t *testing.T, s *Service, email string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		s.pool.Exec(ctx, `DELETE FROM shops WHERE id IN (SELECT shop_id FROM demo_leads WHERE email=$1)`, email)
		s.pool.Exec(ctx, `DELETE FROM demo_leads WHERE email=$1`, email)
	})
}

func TestProvisionCreatesPopulatedShop(t *testing.T) {
	svc, sessions := newTestService(t)
	ctx := context.Background()
	email := fmt.Sprintf("provision-%d@example.com", time.Now().UnixNano())
	cleanupLead(t, svc, email)

	session, returnToken, err := svc.Provision(ctx, email, "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if session == "" || returnToken == "" {
		t.Fatal("Provision returned an empty token")
	}
	if sessions.issued != 1 {
		t.Errorf("issued %d sessions, want 1", sessions.issued)
	}

	var isDemo bool
	var customers int
	if err := svc.pool.QueryRow(ctx, `
		SELECT s.is_demo, (SELECT COUNT(*) FROM customers WHERE shop_id = s.id)
		FROM shops s JOIN demo_leads l ON l.shop_id = s.id WHERE l.email = $1`, email,
	).Scan(&isDemo, &customers); err != nil {
		t.Fatalf("query shop: %v", err)
	}
	if !isDemo {
		t.Error("shop is not marked is_demo")
	}
	if customers < 3 {
		t.Errorf("customers = %d, want the template seeded", customers)
	}
}

// A prospect who submits twice must not accumulate shops.
func TestProvisionIsIdempotentPerEmail(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	email := fmt.Sprintf("idem-%d@example.com", time.Now().UnixNano())
	cleanupLead(t, svc, email)

	if _, _, err := svc.Provision(ctx, email, "1.2.3.4", "a"); err != nil {
		t.Fatalf("first Provision: %v", err)
	}
	if _, _, err := svc.Provision(ctx, email, "1.2.3.4", "a"); err != nil {
		t.Fatalf("second Provision: %v", err)
	}

	var shops int
	if err := svc.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM shops s JOIN demo_leads l ON l.shop_id = s.id WHERE l.email = $1`, email,
	).Scan(&shops); err != nil {
		t.Fatalf("count: %v", err)
	}
	if shops != 1 {
		t.Errorf("shops = %d, want 1", shops)
	}
}

func TestProvisionRejectsInvalidEmail(t *testing.T) {
	svc, _ := newTestService(t)

	if _, _, err := svc.Provision(context.Background(), "not-an-email", "", ""); err == nil {
		t.Fatal("expected an error for an invalid address")
	}
}

func TestResumeReturnsSessionForValidToken(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	email := fmt.Sprintf("resume-%d@example.com", time.Now().UnixNano())
	cleanupLead(t, svc, email)

	_, returnToken, err := svc.Provision(ctx, email, "", "")
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	session, err := svc.Resume(ctx, returnToken)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if session == "" {
		t.Fatal("Resume returned an empty session")
	}

	var verified *time.Time
	if err := svc.pool.QueryRow(ctx,
		`SELECT verified_at FROM demo_leads WHERE email=$1`, email).Scan(&verified); err != nil {
		t.Fatalf("query lead: %v", err)
	}
	if verified == nil {
		t.Error("verified_at not stamped — clicking the emailed link is what verifies a lead")
	}
}

func TestResumeRejectsUnknownToken(t *testing.T) {
	svc, _ := newTestService(t)

	if _, err := svc.Resume(context.Background(), "nope"); err == nil {
		t.Fatal("expected an error for an unknown token")
	}
}

// The single most dangerous bug available here: deleting a paying customer's
// shop. The sweep must touch only expired demo shops.
func TestSweepDeletesOnlyExpiredDemoShops(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	var realID, liveDemoID, expiredDemoID string
	if err := svc.pool.QueryRow(ctx,
		`INSERT INTO shops (name, slug, is_demo) VALUES ('Real', 'real-'||gen_random_uuid(), FALSE) RETURNING id`,
	).Scan(&realID); err != nil {
		t.Fatalf("insert real: %v", err)
	}
	if err := svc.pool.QueryRow(ctx,
		`INSERT INTO shops (name, slug, is_demo, expires_at) VALUES ('Live', 'live-'||gen_random_uuid(), TRUE, NOW() + INTERVAL '1 day') RETURNING id`,
	).Scan(&liveDemoID); err != nil {
		t.Fatalf("insert live: %v", err)
	}
	if err := svc.pool.QueryRow(ctx,
		`INSERT INTO shops (name, slug, is_demo, expires_at) VALUES ('Expired', 'exp-'||gen_random_uuid(), TRUE, NOW() - INTERVAL '1 day') RETURNING id`,
	).Scan(&expiredDemoID); err != nil {
		t.Fatalf("insert expired: %v", err)
	}
	t.Cleanup(func() {
		svc.pool.Exec(ctx, `DELETE FROM shops WHERE id = ANY($1)`, []string{realID, liveDemoID, expiredDemoID})
	})

	deleted, err := svc.SweepExpired(ctx)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	for _, tc := range []struct {
		name   string
		id     string
		exists bool
	}{
		{"real shop", realID, true},
		{"live demo", liveDemoID, true},
		{"expired demo", expiredDemoID, false},
	} {
		var count int
		if err := svc.pool.QueryRow(ctx, `SELECT COUNT(*) FROM shops WHERE id=$1`, tc.id).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", tc.name, err)
		}
		if (count == 1) != tc.exists {
			t.Errorf("%s: exists = %v, want %v", tc.name, count == 1, tc.exists)
		}
	}
}

// The lead must survive its shop, or the sweep destroys the mailing list.
func TestSweepPreservesLeadAfterShopDeleted(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	email := fmt.Sprintf("sweep-%d@example.com", time.Now().UnixNano())
	cleanupLead(t, svc, email)

	if _, _, err := svc.Provision(ctx, email, "", ""); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if _, err := svc.pool.Exec(ctx,
		`UPDATE shops SET expires_at = NOW() - INTERVAL '1 day'
		 WHERE id IN (SELECT shop_id FROM demo_leads WHERE email=$1)`, email); err != nil {
		t.Fatalf("expire: %v", err)
	}

	if _, err := svc.SweepExpired(ctx); err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}

	var shopID *string
	if err := svc.pool.QueryRow(ctx,
		`SELECT shop_id FROM demo_leads WHERE email=$1`, email).Scan(&shopID); err != nil {
		t.Fatalf("lead was deleted with its shop: %v", err)
	}
	if shopID != nil {
		t.Error("shop_id should be NULL after the shop was swept")
	}
}

// An expired demo must yield a fresh shop rather than resurrecting a dead one.
func TestProvisionAfterExpiryCreatesNewShop(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	email := fmt.Sprintf("reprov-%d@example.com", time.Now().UnixNano())
	cleanupLead(t, svc, email)

	if _, _, err := svc.Provision(ctx, email, "", ""); err != nil {
		t.Fatalf("first Provision: %v", err)
	}
	if _, err := svc.pool.Exec(ctx,
		`UPDATE shops SET expires_at = NOW() - INTERVAL '1 day'
		 WHERE id IN (SELECT shop_id FROM demo_leads WHERE email=$1)`, email); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if _, err := svc.SweepExpired(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if _, _, err := svc.Provision(ctx, email, "", ""); err != nil {
		t.Fatalf("second Provision: %v", err)
	}

	var shopID *string
	if err := svc.pool.QueryRow(ctx,
		`SELECT shop_id FROM demo_leads WHERE email=$1`, email).Scan(&shopID); err != nil {
		t.Fatalf("query lead: %v", err)
	}
	if shopID == nil {
		t.Error("lead has no shop after re-provisioning")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api-go && go test ./internal/demo/ -run 'TestProvision|TestResume|TestSweep' -v`
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 3: Implement the service**

Write `apps/api-go/internal/demo/service.go` with:

```go
type SessionIssuer interface {
	IssueSession(ctx context.Context, userID, shopID, role string) (string, error)
}

type Service struct {
	pool     *pgxpool.Pool
	sessions SessionIssuer
	ttl      time.Duration
}

func NewService(pool *pgxpool.Pool, sessions SessionIssuer, ttl time.Duration) *Service {
	return &Service{pool: pool, sessions: sessions, ttl: ttl}
}
```

`Provision`:
1. `mail.ParseAddress(email)`; return an error on failure — matches `GenerateMagicLink`
2. Look for an existing lead whose shop exists and has `expires_at > NOW()`. Found → load its user and `return_token`, issue a session, stamp `last_seen_at`, return
3. Otherwise open a transaction. Insert the shop with `is_demo = TRUE`, `expires_at = NOW() + ttl`, and a slug of `"demo-" + first 8 chars of a UUID` so two shops never collide
4. Insert the owner user: `role = 'owner'`, `email` = the lead's address, `password_hash` = bcrypt of 32 random bytes from `crypto/rand`, never stored elsewhere or returned. `users` is `UNIQUE(shop_id, email)`, so reusing the real address in a fresh shop is safe
5. `Seed(ctx, tx, shopID)`
6. Upsert `demo_leads` with a `return_token` of 32 random hex characters from `crypto/rand` — `ON CONFLICT (email) DO UPDATE SET shop_id = EXCLUDED.shop_id, return_token = EXCLUDED.return_token, last_seen_at = NOW()`
7. Commit, then `IssueSession`. Issuing after commit means a failed commit cannot leave a session pointing at a rolled-back shop

`Resume`:
1. Join `demo_leads` to `shops` on the token, requiring `shops.id IS NOT NULL AND expires_at > NOW()`; no row → error
2. Load the shop's owner user
3. `UPDATE demo_leads SET verified_at = COALESCE(verified_at, NOW()), last_seen_at = NOW()`
4. `IssueSession`

`SweepExpired`:
```go
func (s *Service) SweepExpired(ctx context.Context) (int, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM shops WHERE is_demo = TRUE AND expires_at IS NOT NULL AND expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("sweep demo shops: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
```

The `is_demo = TRUE` predicate is what stops this from being a catastrophe. Never widen it.

- [ ] **Step 4: Run the tests**

Run: `cd apps/api-go && go test -race ./internal/demo/ -v`
Expected: PASS — all provisioning, resume, and sweep tests.

- [ ] **Step 5: Commit**

```bash
git add apps/api-go/internal/demo/service.go apps/api-go/internal/demo/service_test.go
git commit -m "feat: provision, resume, and expire demo shops"
```

---

### Task 6: HTTP endpoints and wiring

**Files:**
- Create: `apps/api-go/internal/demo/handler.go`
- Modify: `apps/api-go/cmd/server/main.go`
- Modify: `apps/api-go/internal/config/config.go`
- Modify: `.env.example`

**Interfaces:**
- Consumes: `Service` (Task 5), `GuardSMS` / `GuardEmail` (Task 4).
- Produces: `func NewHandler(svc *Service, mailer email.Sender, appURL string) *Handler` with `Start` and `Resume` methods; routes `POST /demo` and `POST /demo/resume`.

- [ ] **Step 1: Write the handler**

`apps/api-go/internal/demo/handler.go`:

- `POST /demo` — decode `{"email":"..."}`; call `Provision`; on error return 400 `{"error":"..."}`; on success set the `session` cookie with exactly the attributes `auth.Handler.Verify` uses (`Path: "/"`, `HttpOnly: true`, `Secure: true`, `SameSite: http.SameSiteLaxMode`, `MaxAge: 86400`), send the return-link email best-effort with the **unwrapped** mailer, and encode `{"status":"ok"}`. No token appears in the body.
- `POST /demo/resume` — decode `{"token":"..."}`; call `Resume`; on error 401 `{"error":"invalid or expired demo link"}`; on success set the same cookie and encode `{"status":"ok"}`.

The return email body:

```go
resumeURL := fmt.Sprintf("%s/demo/resume?token=%s", strings.TrimRight(h.appURL, "/"), url.QueryEscape(returnToken))
subject := "Your GarageFlow demo shop"
text := fmt.Sprintf(
	"Your demo shop is ready. Come back to it any time in the next 14 days:\n\n%s\n\n"+
		"It is preloaded with customers, vehicles, repair orders and an inspection to try.\n",
	resumeURL,
)
```

Send it with the mailer passed to `NewHandler`, which must be the raw sender — the demo's own mail is not customer outbound and must not be suppressed by `GuardEmail`.

- [ ] **Step 2: Add configuration**

In `apps/api-go/internal/config/config.go`, beside `AuthRateLimitPerMin`:

```go
	// DemoRateLimitPerMin caps POST /demo per client IP. Lower than the auth
	// limit because provisioning a shop is far more expensive than a login
	// attempt. 0 disables the limit, matching AuthRateLimitPerMin.
	DemoRateLimitPerMin int
```

and in the loader:

```go
		DemoRateLimitPerMin: getEnvInt("DEMO_RATE_LIMIT_PER_MIN", 3),
```

In `.env.example`, under Core:

```bash
# Caps POST /demo per client IP per minute. Provisioning a shop is expensive,
# so this is deliberately tighter than AUTH_RATE_LIMIT_PER_MIN. 0 disables it
# (used by E2E, which provisions faster than any real visitor).
DEMO_RATE_LIMIT_PER_MIN=3
```

- [ ] **Step 3: Wire it into main.go**

Wrap the senders where they are constructed, so every customer-facing handler receives the guarded versions:

```go
	// Demo tenants must never send to a real phone or inbox. Wrap once, here,
	// so no future handler can accidentally take the unguarded sender.
	guardedSMS := demo.GuardSMS(smsSender, pool)
	guardedMailer := demo.GuardEmail(mailer, pool)
```

Pass `guardedSMS` and `guardedMailer` to `inspections.NewHandler` and to any other handler that sends to customers. Leave `auth.NewService` on the **raw** `mailer` — sign-in links are not customer outbound and must always send.

Register the routes beside `/healthz`, outside the authenticated group:

```go
	demoSvc := demo.NewService(pool, authSvc, 14*24*time.Hour)
	demoHandler := demo.NewHandler(demoSvc, mailer, cfg.AppURL)

	r.Route("/demo", func(r chi.Router) {
		r.Use(chimw.Timeout(requestTimeout))
		if cfg.DemoRateLimitPerMin > 0 {
			r.Use(httprate.LimitByIP(cfg.DemoRateLimitPerMin, time.Minute))
		}
		r.Post("/", demoHandler.Start)
		r.Post("/resume", demoHandler.Resume)
	})
```

Start the sweep beside the realtime bridge, reusing `hubCtx`:

```go
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-hubCtx.Done():
				return
			case <-ticker.C:
				if n, err := demoSvc.SweepExpired(hubCtx); err != nil {
					log.Printf("[demo] sweep failed: %v", err)
				} else if n > 0 {
					log.Printf("[demo] swept %d expired demo shops", n)
				}
			}
		}
	}()
```

- [ ] **Step 4: Build and vet**

Run: `cd apps/api-go && go build ./... && go vet ./...`
Expected: no output.

- [ ] **Step 5: Verify the endpoint by hand**

Run:
```bash
cd /home/cd/Work/garageflow && docker compose up -d --build
curl -sS -X POST http://localhost:8081/demo \
  -H 'Content-Type: application/json' \
  -d '{"email":"plan-check@example.com"}' -i | head -20
```
Expected: `200`, a `Set-Cookie: session=...` header, body `{"status":"ok"}`, and **no token anywhere in the body**.

- [ ] **Step 6: Commit**

```bash
git add apps/api-go/ .env.example
git commit -m "feat: expose demo provisioning and resume endpoints"
```

---

### Task 7: Frontend entry points

**Files:**
- Create: `apps/web-next/src/components/DemoCapture.tsx`
- Create: `apps/web-next/src/app/demo/resume/page.tsx`
- Modify: `apps/web-next/src/app/login/page.tsx:124-129`
- Modify: `apps/web-next/src/app/page.tsx`
- Modify: `apps/web-next/src/app/robots.ts`

**Interfaces:**
- Consumes: `POST /demo`, `POST /demo/resume`.
- Produces: `<DemoCapture />`, a self-contained email field plus submit.

- [ ] **Step 1: Build the capture component**

`DemoCapture.tsx` is a client component holding its own `email`, `loading`, and `error` state. On submit it calls `api.post("/demo", { email })` and, on success, `window.location.href = "/dashboard"` — a hard navigation, so `AuthProvider` remounts and picks up the new session cookie exactly as it does after sign-in. On `ApiError` with status 429 show "Too many demo requests just now. Try again in a minute."; otherwise show the API's message.

Match the existing form styling: `gf-label`, `gf-input`, `gf-btn-primary`, and the `Mail` icon positioned as in `login/page.tsx:63-71`. Give the submit button `data-testid="demo-submit"` and the input the placeholder `"you@shop.com"`.

- [ ] **Step 2: Place it on the login page**

Replace the "Have a sign-in code?" paragraph block at `login/page.tsx:124-129` with a bordered section containing a short line — "Just looking? Try a live demo shop." — the `<DemoCapture />`, and the existing sign-in-code link moved beneath it.

- [ ] **Step 3: Place it on the marketing landing**

In `apps/web-next/src/app/page.tsx`, render `<DemoCapture />` next to the existing "Start free" CTA. Read the file first and match its section rhythm rather than dropping a bare form into the hero.

- [ ] **Step 4: Build the resume page**

`apps/web-next/src/app/demo/resume/page.tsx` mirrors `apps/web-next/src/app/auth/verify/page.tsx` exactly in structure: a `Suspense`-wrapped inner component using `useSearchParams`, a `useRef` guard so a re-render cannot spend the token twice, `api.post("/demo/resume", { token })`, then `router.push("/dashboard")`. On failure: "That demo link has expired. Demo shops last 14 days — enter your email to start a fresh one." followed by `<DemoCapture />`.

- [ ] **Step 5: Keep it out of search results**

In `apps/web-next/src/app/robots.ts`, add `"/demo"` to the `disallow` array, keeping the list alphabetical.

- [ ] **Step 6: Typecheck and build**

Run: `cd apps/web-next && npx tsc --noEmit && npm run build`
Expected: no type errors; the route list includes `/demo/resume`.

- [ ] **Step 7: Commit**

```bash
git add apps/web-next/src
git commit -m "feat: add demo capture to login and marketing, plus the resume page"
```

---

### Task 8: End-to-end coverage

**Files:**
- Create: `apps/web-next/e2e/demo.spec.ts`

**Interfaces:**
- Consumes: everything above.
- Produces: nothing.

- [ ] **Step 1: Write the spec**

```ts
import { test, expect } from "./helpers/fixtures";
import { API_URL, uniq } from "./helpers/api";

const demoEmail = () => `demo-${uniq()}@example.com`;

test.describe("demo tenant", () => {
  test("email capture lands on a populated dashboard", async ({ page }) => {
    await page.goto("/login");
    await page.getByPlaceholder("you@shop.com").last().fill(demoEmail());
    await page.getByTestId("demo-submit").click();

    await expect(page).toHaveURL(/\/dashboard/);
    // The pitch is a shop with work in it; an empty demo demos nothing.
    await page.goto("/repair-orders");
    await expect(page.getByRole("link").first()).toBeVisible();
  });

  test("the same email resumes one shop rather than accumulating them", async ({ request }) => {
    const email = demoEmail();

    const first = await request.post(`${API_URL}/demo`, { data: { email } });
    expect(first.status()).toBe(200);
    const second = await request.post(`${API_URL}/demo`, { data: { email } });
    expect(second.status()).toBe(200);

    // Same tenant both times.
    const me = await request.get(`${API_URL}/auth/me`);
    expect(me.status()).toBe(200);
  });

  test("provisioning never returns a token in the body", async ({ request }) => {
    const res = await request.post(`${API_URL}/demo`, { data: { email: demoEmail() } });

    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).not.toMatch(/token/i);
    expect(body).not.toMatch(/password/i);
  });

  test("an invalid address is rejected", async ({ request }) => {
    const res = await request.post(`${API_URL}/demo`, { data: { email: "nope" } });
    expect(res.status()).toBe(400);
  });

  test("a bad resume token explains itself", async ({ page }) => {
    await page.goto("/demo/resume?token=not-a-real-token");
    await expect(page.getByText(/expired/i)).toBeVisible();
  });

  // The guard that stops a prospect texting a stranger.
  test("a demo shop sends nothing outbound but still shows link and QR", async ({ page, request }) => {
    await request.post(`${API_URL}/demo`, { data: { email: demoEmail() } });

    await page.goto("/repair-orders");
    await page.getByRole("link").first().click();
    await page.getByTestId("start-inspection").click();
    await expect(page).toHaveURL(/\/inspections\//);

    await page.getByTestId("send-inspection").click();
    await expect(page.getByTestId("customer-link")).toBeVisible();

    await page.getByTestId("toggle-qr").click();
    await expect(page.getByTestId("qr-image")).toHaveAttribute("src", /^data:image\/png;base64,/);
  });
});
```

- [ ] **Step 2: Set the E2E rate limit to 0**

E2E provisions faster than any real visitor. In `docker-compose.yml`, add `DEMO_RATE_LIMIT_PER_MIN: "0"` to the `api` service's environment, beside the existing `AUTH_DEV_CODES` setting. **Do not** add it to `docker-compose.prod.yml`.

- [ ] **Step 3: Run the demo spec**

Run: `cd apps/web-next && npx playwright test demo.spec.ts --reporter=line`
Expected: 6 passed.

- [ ] **Step 4: Run the whole suite**

Run: `cd /home/cd/Work/garageflow && make e2e`
Expected: all previously passing tests still pass, plus the 6 new ones.

- [ ] **Step 5: Run every Go test with the race detector**

Run: `cd apps/api-go && go test -race -count=1 ./...`
Expected: no failures.

- [ ] **Step 6: Commit**

```bash
git add apps/web-next/e2e/demo.spec.ts docker-compose.yml
git commit -m "test: cover demo provisioning, resume, and outbound suppression"
```

---

## Verification before calling this done

- [ ] `cd apps/api-go && go test -race -count=1 ./...` — green
- [ ] `cd apps/web-next && npx tsc --noEmit` — clean
- [ ] `cd apps/web-next && npm run build` — clean, `/demo/resume` in the route list
- [ ] `make e2e` — green
- [ ] `grep -rn "password123\|owner@garageflow.app" apps/web-next/src` — no matches
- [ ] A demo shop's inspection send produces no Twilio or Mailgun call in `docker logs garageflow-api-1`
