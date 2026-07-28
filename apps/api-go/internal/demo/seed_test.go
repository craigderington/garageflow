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
