package inspections_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/garageflow/api-go/internal/demo"
	"github.com/garageflow/api-go/internal/inspections"
	"github.com/garageflow/api-go/internal/middleware"
)

// testPool mirrors internal/demo's test helper: skip (don't fail) when no
// database is reachable, so this still runs against the same dockerised
// Postgres the rest of the suite uses.
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

// spySMS and spyEmail record a call by signaling a channel rather than just
// incrementing a counter, so the test can deterministically wait for the
// delivery goroutine spawned by Send instead of sleeping and polling.
type spySMS struct{ called chan string }

func (s *spySMS) Enabled() bool { return true }
func (s *spySMS) Send(_ context.Context, to, _ string) error {
	s.called <- to
	return nil
}

type spyEmail struct{ called chan string }

func (s *spyEmail) Send(_ context.Context, to, _, _, _ string) error {
	s.called <- to
	return nil
}

// seedSendableInspection inserts a shop (real or demo), a customer with a
// phone and email, a repair order, and an in_progress inspection ready to
// send. It returns the inspection ID and registers cleanup.
func seedSendableInspection(t *testing.T, pool *pgxpool.Pool, isDemo bool) (shopID, inspID string) {
	t.Helper()
	ctx := context.Background()

	slug := fmt.Sprintf("delivery-test-%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx,
		`INSERT INTO shops (name, slug, is_demo) VALUES ('Delivery Test', $1, $2) RETURNING id`,
		slug, isDemo,
	).Scan(&shopID); err != nil {
		t.Fatalf("insert shop: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM shops WHERE id=$1`, shopID) })

	var custID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (shop_id, name, phone, email) VALUES ($1, 'Pat Customer', '555-0100', 'pat@example.com') RETURNING id`,
		shopID,
	).Scan(&custID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	var roID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO repair_orders (shop_id, customer_id) VALUES ($1, $2) RETURNING id`,
		shopID, custID,
	).Scan(&roID); err != nil {
		t.Fatalf("insert repair order: %v", err)
	}

	if err := pool.QueryRow(ctx,
		`INSERT INTO inspections (shop_id, repair_order_id, status) VALUES ($1, $2, 'in_progress') RETURNING id`,
		shopID, roID,
	).Scan(&inspID); err != nil {
		t.Fatalf("insert inspection: %v", err)
	}

	return shopID, inspID
}

// sendAndAwaitDelivery drives POST /inspections/{id}/send through a handler
// wired exactly like main.go wires inspHandler (guarded SMS + email), with
// shopID injected into the request context the way AuthMiddleware/
// TenantMiddleware do for a real request. It waits up to 2s for the
// detached delivery goroutine to either call the spy or not.
func sendAndAwaitDelivery(t *testing.T, pool *pgxpool.Pool, shopID, inspID string) (smsCalled, emailCalled bool) {
	t.Helper()

	sms := &spySMS{called: make(chan string, 1)}
	mail := &spyEmail{called: make(chan string, 1)}
	guardedSMS := demo.GuardSMS(sms, pool)
	guardedMailer := demo.GuardEmail(mail, pool)

	h := inspections.NewHandler(pool, nil, guardedSMS, guardedMailer, "http://localhost:3000")

	r := chi.NewRouter()
	r.Post("/inspections/{id}/send", h.Send)

	req := httptest.NewRequest("POST", "/inspections/"+inspID+"/send", nil)
	req = req.WithContext(middleware.WithShopID(req.Context(), shopID))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("Send returned %d, body=%s", w.Code, w.Body.String())
	}

	timeout := time.After(2 * time.Second)
	for smsCalled == false || emailCalled == false {
		select {
		case <-sms.called:
			smsCalled = true
		case <-mail.called:
			emailCalled = true
		case <-timeout:
			return smsCalled, emailCalled
		}
	}
	return smsCalled, emailCalled
}

// This is the regression test for the bug where Send's async delivery
// goroutine builds its context from context.Background() (no shop ID), so
// the guarded senders wired in main.go fail closed via demo.IsDemoShop and
// silently drop the SMS/email for every shop — real paying tenants included,
// not just demo ones. The positive control (a real shop still gets its
// delivery) is the load-bearing assertion here; a demo-shop-is-suppressed
// test alone would not have caught this regression.
func TestSendDeliversToRealShopThroughAsyncPath(t *testing.T) {
	pool := testPool(t)
	shopID, inspID := seedSendableInspection(t, pool, false)

	smsCalled, emailCalled := sendAndAwaitDelivery(t, pool, shopID, inspID)

	if !smsCalled {
		t.Error("SMS was not sent for a real (non-demo) shop through the async delivery path")
	}
	if !emailCalled {
		t.Error("email was not sent for a real (non-demo) shop through the async delivery path")
	}
}

// Companion negative control: a demo shop's delivery must still be
// suppressed through the same async path, proving the fix didn't just
// disable the guard.
func TestSendSuppressesForDemoShopThroughAsyncPath(t *testing.T) {
	pool := testPool(t)
	shopID, inspID := seedSendableInspection(t, pool, true)

	smsCalled, emailCalled := sendAndAwaitDelivery(t, pool, shopID, inspID)

	if smsCalled {
		t.Error("SMS was sent for a demo shop through the async delivery path, want suppressed")
	}
	if emailCalled {
		t.Error("email was sent for a demo shop through the async delivery path, want suppressed")
	}
}
