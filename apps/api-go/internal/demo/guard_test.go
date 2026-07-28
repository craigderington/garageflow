package demo

import (
	"context"
	"testing"

	"github.com/garageflow/api-go/internal/middleware"
)

type spySMS struct{ calls int }

func (s *spySMS) Send(_ context.Context, _, _ string) error { s.calls++; return nil }
func (s *spySMS) Enabled() bool                             { return true }

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
