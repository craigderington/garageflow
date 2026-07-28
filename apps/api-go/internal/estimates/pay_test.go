package estimates_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/garageflow/api-go/internal/estimates"
	"github.com/garageflow/api-go/internal/middleware"
	"github.com/garageflow/api-go/internal/payments"
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

// seedPayableEstimate creates a shop with one approved estimate ready to pay.
func seedPayableEstimate(t *testing.T, pool *pgxpool.Pool, label string) (shopID, estimateID string) {
	t.Helper()
	ctx := context.Background()

	slug := fmt.Sprintf("pay-%s-%d", label, time.Now().UnixNano())
	if err := pool.QueryRow(ctx,
		`INSERT INTO shops (name, slug) VALUES ($1, $2) RETURNING id`, "Pay "+label, slug,
	).Scan(&shopID); err != nil {
		t.Fatalf("insert shop: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM shops WHERE id=$1`, shopID) })

	var customerID, roID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (shop_id, name) VALUES ($1, 'Pat') RETURNING id`, shopID,
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO repair_orders (shop_id, customer_id, description) VALUES ($1, $2, 'Brakes') RETURNING id`,
		shopID, customerID,
	).Scan(&roID); err != nil {
		t.Fatalf("insert repair order: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO estimates (shop_id, repair_order_id, total, status) VALUES ($1, $2, 200, 'approved') RETURNING id`,
		shopID, roID,
	).Scan(&estimateID); err != nil {
		t.Fatalf("insert estimate: %v", err)
	}
	return shopID, estimateID
}

// stripeSpy stands in for api.stripe.com and records whether it was ever
// contacted. Being called at all is the failure this test exists to catch.
func stripeSpy(t *testing.T) (*payments.Service, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"cs_test_123","url":"https://checkout.stripe.test/cs_test_123"}`))
	}))
	t.Cleanup(srv.Close)
	// A non-empty secret key is what puts payments.Service into live mode.
	return payments.NewService("sk_test_fake", "", srv.URL), &hits
}

func pay(t *testing.T, h *estimates.Handler, shopID, estimateID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/estimates/"+estimateID+"/pay", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", estimateID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(middleware.WithShopID(ctx, shopID))

	rec := httptest.NewRecorder()
	h.Pay(rec, req)
	return rec
}

// In production a prospect clicking "collect payment" in their demo shop must
// not create a real Checkout Session on the merchant's live Stripe account.
func TestPayNeverReachesStripeForADemoShop(t *testing.T) {
	pool := testPool(t)
	pay1, hits := stripeSpy(t)
	shopID, estimateID := seedPayableEstimate(t, pool, "demo")

	h := estimates.NewHandler(pool, nil, pay1, "http://localhost:3000",
		func(context.Context) bool { return true })

	rec := pay(t, h, shopID, estimateID)
	if rec.Code != http.StatusOK {
		t.Fatalf("Pay returned %d, want 200 — the prospect must still see the flow complete", rec.Code)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("Stripe was called %d times for a demo shop, want 0", n)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["mode"] == "stripe" {
		t.Error(`mode = "stripe" for a demo shop`)
	}

	// The prospect still sees the estimate settle, exactly as in dev mode.
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM estimates WHERE id=$1`, estimateID).Scan(&status); err != nil {
		t.Fatalf("query estimate: %v", err)
	}
	if status != "paid" {
		t.Errorf("status = %q, want paid — the demo must still complete the flow", status)
	}
}

// Positive control: a real shop with Stripe configured must still get a real
// Checkout Session. Without this, a guard that blocked everyone would pass.
func TestPayStillReachesStripeForARealShop(t *testing.T) {
	pool := testPool(t)
	pay1, hits := stripeSpy(t)
	shopID, estimateID := seedPayableEstimate(t, pool, "real")

	h := estimates.NewHandler(pool, nil, pay1, "http://localhost:3000",
		func(context.Context) bool { return false })

	rec := pay(t, h, shopID, estimateID)
	if rec.Code != http.StatusOK {
		t.Fatalf("Pay returned %d, want 200", rec.Code)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("Stripe was called %d times for a real shop, want 1", n)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["mode"] != "stripe" {
		t.Errorf("mode = %q, want stripe", body["mode"])
	}
	// Live mode settles on the webhook, not here.
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM estimates WHERE id=$1`, estimateID).Scan(&status); err != nil {
		t.Fatalf("query estimate: %v", err)
	}
	if status != "approved" {
		t.Errorf("status = %q, want it left approved until the webhook settles it", status)
	}
}

// A nil predicate is a wiring mistake; it must cost a merchant a card payment,
// never charge a prospect's.
func TestPayFailsClosedWithoutADemoPredicate(t *testing.T) {
	pool := testPool(t)
	pay1, hits := stripeSpy(t)
	shopID, estimateID := seedPayableEstimate(t, pool, "nilpred")

	h := estimates.NewHandler(pool, nil, pay1, "http://localhost:3000", nil)

	if rec := pay(t, h, shopID, estimateID); rec.Code != http.StatusOK {
		t.Fatalf("Pay returned %d, want 200", rec.Code)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("Stripe was called %d times with a nil predicate, want 0 — it must fail closed", n)
	}
}
