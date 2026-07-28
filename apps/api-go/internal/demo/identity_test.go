package demo

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/garageflow/api-go/internal/auth"
)

// realUserPassword is the plaintext seedRealUser sets on the real owner.
const realUserPassword = "correct-horse-battery"

// seedRealUser creates a genuine (non-demo) shop with one owner who can log in
// with realUserPassword, and returns the shop id, user id and email.
func seedRealUser(t *testing.T, svc *Service) (shopID, userID, email string) {
	t.Helper()
	ctx := context.Background()

	email = fmt.Sprintf("victim-%d@realshop.test", time.Now().UnixNano())
	if err := svc.pool.QueryRow(ctx,
		`INSERT INTO shops (name, slug, is_demo) VALUES ('Real Shop', 'real-'||gen_random_uuid(), FALSE) RETURNING id`,
	).Scan(&shopID); err != nil {
		t.Fatalf("insert real shop: %v", err)
	}
	t.Cleanup(func() { svc.pool.Exec(context.Background(), `DELETE FROM shops WHERE id=$1`, shopID) })

	// MinCost: this test asserts which row is found, not bcrypt's work factor.
	raw, err := bcrypt.GenerateFromPassword([]byte(realUserPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	hash := string(raw)
	if err := svc.pool.QueryRow(ctx,
		`INSERT INTO users (shop_id, email, name, role, password_hash) VALUES ($1, $2, 'Real Owner', 'owner', $3) RETURNING id`,
		shopID, email, hash,
	).Scan(&userID); err != nil {
		t.Fatalf("insert real user: %v", err)
	}
	return shopID, userID, email
}

// The attack this guards against: an unauthenticated POST /demo carrying a
// real customer's address. If the demo owner row were stored under that
// address, `SELECT ... FROM users WHERE email = $1` (no ORDER BY, no LIMIT,
// and users is only UNIQUE(shop_id, email)) could return the demo row, so the
// victim's password would be checked against the demo's random bcrypt hash and
// their session would be scoped to the attacker's shop.
func TestProvisionDoesNotHijackAnExistingUsersIdentity(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	realShopID, realUserID, email := seedRealUser(t, svc)
	cleanupLead(t, svc, email)

	if _, _, err := svc.Provision(ctx, email, "1.2.3.4", "attacker"); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	// Exactly one users row may carry the victim's address, and it must be
	// theirs.
	var rows int
	if err := svc.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, email).Scan(&rows); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if rows != 1 {
		t.Fatalf("%d users rows carry the prospect's address, want 1 — POST /demo planted a row under someone else's identity", rows)
	}

	// Password login still resolves to the victim's own shop.
	authSvc := auth.NewService(svc.pool, nil, "test-secret", nil, "http://localhost")
	gotUser, gotShop, gotRole, err := authSvc.VerifyPassword(ctx, email, realUserPassword)
	if err != nil {
		t.Fatalf("VerifyPassword for the real user: %v — the demo row shadowed their credentials", err)
	}
	if gotUser != realUserID || gotShop != realShopID {
		t.Errorf("password login resolved to (user=%s, shop=%s), want (%s, %s)", gotUser, gotShop, realUserID, realShopID)
	}
	if gotRole != "owner" {
		t.Errorf("role = %q, want owner", gotRole)
	}

	// And the demo shop's own owner is a synthetic address, not the prospect's.
	var demoOwnerEmail string
	if err := svc.pool.QueryRow(ctx, `
		SELECT u.email FROM users u
		JOIN demo_leads l ON l.shop_id = u.shop_id
		WHERE l.email = $1 AND u.role = 'owner'`, email,
	).Scan(&demoOwnerEmail); err != nil {
		t.Fatalf("query demo owner: %v", err)
	}
	if demoOwnerEmail == email {
		t.Error("the demo owner user was stored under the prospect's real address")
	}
}

// The magic-link half of the same attack: verify resolves the address to a
// user row, so a planted demo row could hand the victim a session on the
// attacker's shop. Note that which of two equal-email rows Postgres returns is
// not deterministic — this test reproduces the end-to-end scenario, but the
// load-bearing proof is the "exactly one users row" assertion in
// TestProvisionDoesNotHijackAnExistingUsersIdentity above.
func TestMagicLinkForAnExistingUserIgnoresDemoShops(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		t.Skipf("redis unavailable at %s: %v", addr, err)
	}
	t.Cleanup(func() { rdb.Close() })

	realShopID, realUserID, email := seedRealUser(t, svc)
	cleanupLead(t, svc, email)

	if _, _, err := svc.Provision(ctx, email, "1.2.3.4", "attacker"); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	authSvc := auth.NewService(svc.pool, rdb, "test-secret", nil, "http://localhost")
	code, err := authSvc.GenerateMagicLink(ctx, email)
	if err != nil {
		t.Fatalf("GenerateMagicLink: %v", err)
	}
	session, err := authSvc.VerifyMagicLink(ctx, code)
	if err != nil {
		t.Fatalf("VerifyMagicLink: %v", err)
	}
	gotUser, gotShop, _, err := authSvc.ValidateSession(ctx, session)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if gotUser != realUserID || gotShop != realShopID {
		t.Errorf("magic link resolved to (user=%s, shop=%s), want the real user's own shop (%s, %s)", gotUser, gotShop, realUserID, realShopID)
	}
}

// mail.ParseAddress accepts a display name; only the address part may be
// persisted, or the same prospect becomes two leads.
func TestProvisionStoresTheParsedAddressOnly(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	plain := fmt.Sprintf("parsed-%d@example.com", time.Now().UnixNano())
	cleanupLead(t, svc, plain)

	if _, _, err := svc.Provision(ctx, "Prospect Name <"+plain+">", "", ""); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	var stored string
	if err := svc.pool.QueryRow(ctx,
		`SELECT email FROM demo_leads WHERE email = $1`, plain).Scan(&stored); err != nil {
		t.Fatalf("lead was not stored under the parsed address: %v", err)
	}
	if stored != plain {
		t.Errorf("stored %q, want %q", stored, plain)
	}
}
