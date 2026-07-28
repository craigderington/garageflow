package demo

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// issued is accessed from multiple goroutines by
// TestProvisionIsRaceSafeForConcurrentCalls, so it's guarded rather than
// left a plain field — otherwise -race reports a false positive on the
// counter itself instead of (or in addition to) any real bug.
type fakeSessions struct {
	mu     sync.Mutex
	issued int
}

func (f *fakeSessions) IssueSession(_ context.Context, userID, shopID, role string) (string, error) {
	f.mu.Lock()
	f.issued++
	f.mu.Unlock()
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

// Two concurrent Provision calls for the same not-yet-provisioned email must
// not race into creating two shops: the loser should find and reuse the
// winner's shop rather than create its own and have the demo_leads upsert
// silently overwrite whichever committed last, orphaning the other shop and
// stranding its return_token.
func TestProvisionIsRaceSafeForConcurrentCalls(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	email := fmt.Sprintf("concurrent-%d@example.com", time.Now().UnixNano())
	cleanupLead(t, svc, email)

	const n = 8
	returnTokens := make([]string, n)
	errs := make([]error, n)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, tok, err := svc.Provision(ctx, email, "1.2.3.4", "concurrent-test")
			returnTokens[i] = tok
			errs[i] = err
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Provision[%d]: %v", i, err)
		}
	}

	var shops int
	if err := svc.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM shops s JOIN demo_leads l ON l.shop_id = s.id WHERE l.email = $1`, email,
	).Scan(&shops); err != nil {
		t.Fatalf("count shops: %v", err)
	}
	if shops != 1 {
		t.Errorf("shops = %d, want exactly 1 despite %d concurrent Provision calls", shops, n)
	}

	distinct := map[string]bool{}
	for i, tok := range returnTokens {
		if tok == "" {
			t.Fatalf("Provision[%d]: empty return token", i)
		}
		distinct[tok] = true
		if _, err := svc.Resume(ctx, tok); err != nil {
			t.Errorf("Resume(return token from call %d): %v — orphaned or rolled-back shop", i, err)
		}
	}
	if len(distinct) != 1 {
		t.Errorf("saw %d distinct return tokens across %d concurrent calls, want 1", len(distinct), n)
	}
}
