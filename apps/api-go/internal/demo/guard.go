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
// tenant that leaks a real text is worse than a real tenant that drops one.
// Most genuine send paths run behind TenantMiddleware and so have a shop, but
// not all: a handler that dispatches delivery on a detached goroutine (a new
// context.Background(), not the request context) must explicitly re-inject
// the shop ID with middleware.WithShopID before sending, or this fails closed
// and silently suppresses the send for that shop too.
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

func (g *guardedSMS) Enabled() bool { return g.inner.Enabled() }

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
