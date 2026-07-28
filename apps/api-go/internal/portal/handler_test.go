package portal_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/garageflow/api-go/internal/middleware"
	"github.com/garageflow/api-go/internal/portal"
)

// testPool mirrors internal/demo's helper: skip (don't fail) when no database
// is reachable, so this runs against the same dockerised Postgres as the rest
// of the suite without breaking a bare checkout.
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

type tenant struct {
	shopID     string
	customerID string
	roID       string
	estimateID string
}

// seedTenant creates a shop with one customer, one repair order and one sent
// estimate, and registers cleanup.
func seedTenant(t *testing.T, pool *pgxpool.Pool, label string) tenant {
	t.Helper()
	ctx := context.Background()
	var tn tenant

	slug := fmt.Sprintf("portal-%s-%d", label, time.Now().UnixNano())
	if err := pool.QueryRow(ctx,
		`INSERT INTO shops (name, slug) VALUES ($1, $2) RETURNING id`, "Portal "+label, slug,
	).Scan(&tn.shopID); err != nil {
		t.Fatalf("insert shop: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM shops WHERE id=$1`, tn.shopID) })

	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (shop_id, name) VALUES ($1, $2) RETURNING id`, tn.shopID, "Cust "+label,
	).Scan(&tn.customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	// vehicle_id and created_by must be non-NULL: GetServiceHistory scans them
	// into plain strings and silently skips any row that fails to scan, which
	// would make a passing assertion meaningless.
	var vehicleID, userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO vehicles (shop_id, customer_id, make, model, year) VALUES ($1, $2, 'Ford', 'F150', 2019) RETURNING id`,
		tn.shopID, tn.customerID,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (shop_id, email, name, role) VALUES ($1, $2, 'Portal Owner', 'owner') RETURNING id`,
		tn.shopID, slug+"@example.test",
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO repair_orders (shop_id, customer_id, vehicle_id, created_by, description)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		tn.shopID, tn.customerID, vehicleID, userID, "Work for "+label,
	).Scan(&tn.roID); err != nil {
		t.Fatalf("insert repair order: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO estimates (shop_id, repair_order_id, total, status) VALUES ($1, $2, 100, 'sent') RETURNING id`,
		tn.shopID, tn.roID,
	).Scan(&tn.estimateID); err != nil {
		t.Fatalf("insert estimate: %v", err)
	}
	return tn
}

// call runs one portal request as the given shop, with chi URL params bound.
func call(h http.HandlerFunc, method, shopID string, params map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/", nil)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(middleware.WithShopID(ctx, shopID))

	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// Until POST /demo existed, a session here required a hand-provisioned
// account. Now anyone with an email address gets one, so an unscoped query on
// a guessable path UUID is a live cross-tenant read.
func TestGetEstimatesIsScopedToTheSessionShop(t *testing.T) {
	pool := testPool(t)
	h := portal.NewHandler(pool)

	victim := seedTenant(t, pool, "victim")
	attacker := seedTenant(t, pool, "attacker")

	// Positive control: the owning shop still sees its own estimate.
	rec := call(h.GetEstimates, http.MethodGet, victim.shopID, map[string]string{"customer_id": victim.customerID})
	var own []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&own); err != nil {
		t.Fatalf("decode own estimates: %v", err)
	}
	if len(own) != 1 {
		t.Fatalf("owning shop saw %d estimates, want 1 — the scoping broke the legitimate read", len(own))
	}

	// The attack: attacker's session, victim's customer UUID.
	rec = call(h.GetEstimates, http.MethodGet, attacker.shopID, map[string]string{"customer_id": victim.customerID})
	var leaked []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&leaked); err != nil {
		t.Fatalf("decode cross-tenant estimates: %v", err)
	}
	if len(leaked) != 0 {
		t.Errorf("another shop's session read %d of the victim's estimates, want 0", len(leaked))
	}
}

func TestGetServiceHistoryIsScopedToTheSessionShop(t *testing.T) {
	pool := testPool(t)
	h := portal.NewHandler(pool)

	victim := seedTenant(t, pool, "histvictim")
	attacker := seedTenant(t, pool, "histattacker")

	rec := call(h.GetServiceHistory, http.MethodGet, victim.shopID, map[string]string{"customer_id": victim.customerID})
	var own []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&own); err != nil {
		t.Fatalf("decode own history: %v", err)
	}
	if len(own) != 1 {
		t.Fatalf("owning shop saw %d repair orders, want 1", len(own))
	}

	rec = call(h.GetServiceHistory, http.MethodGet, attacker.shopID, map[string]string{"customer_id": victim.customerID})
	var leaked []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&leaked); err != nil {
		t.Fatalf("decode cross-tenant history: %v", err)
	}
	if len(leaked) != 0 {
		t.Errorf("another shop's session read %d of the victim's repair orders, want 0", len(leaked))
	}
}

// A cross-tenant write is worse than a cross-tenant read: it approves work on
// someone else's estimate.
func TestApproveEstimateIsScopedToTheSessionShop(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	h := portal.NewHandler(pool)

	victim := seedTenant(t, pool, "apprvictim")
	attacker := seedTenant(t, pool, "apprattacker")

	rec := call(h.ApproveEstimate, http.MethodPost, attacker.shopID, map[string]string{"id": victim.estimateID})
	if rec.Code != http.StatusNotFound {
		t.Errorf("cross-tenant approve returned %d, want 404", rec.Code)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM estimates WHERE id=$1`, victim.estimateID).Scan(&status); err != nil {
		t.Fatalf("query estimate: %v", err)
	}
	if status != "sent" {
		t.Errorf("victim's estimate status = %q, want it untouched at \"sent\"", status)
	}

	// Positive control: the owning shop can still approve.
	rec = call(h.ApproveEstimate, http.MethodPost, victim.shopID, map[string]string{"id": victim.estimateID})
	if rec.Code != http.StatusOK {
		t.Fatalf("owning shop's approve returned %d, want 200", rec.Code)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM estimates WHERE id=$1`, victim.estimateID).Scan(&status); err != nil {
		t.Fatalf("query estimate: %v", err)
	}
	if status != "approved" {
		t.Errorf("status = %q after the owner approved, want approved", status)
	}
	var roStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM repair_orders WHERE id=$1`, victim.roID).Scan(&roStatus); err != nil {
		t.Fatalf("query repair order: %v", err)
	}
	if roStatus != "approved" {
		t.Errorf("repair order status = %q, want approved", roStatus)
	}
}
