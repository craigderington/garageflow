package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func okCheck(name string) Check {
	return Check{Name: name, Ping: func(context.Context) error { return nil }}
}

func do(t *testing.T, h *Handler) (int, response) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	var body response
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return rec.Code, body
}

func TestHealthyReportsOK(t *testing.T) {
	code, body := do(t, New(okCheck("postgres"), okCheck("redis")))

	if code != http.StatusOK {
		t.Errorf("status = %d, want 200", code)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want ok", body.Status)
	}
	for _, name := range []string{"postgres", "redis"} {
		if body.Checks[name] != "ok" {
			t.Errorf("checks[%s] = %q, want ok", name, body.Checks[name])
		}
	}
}

// The whole point: one dead dependency must fail the endpoint, and the
// response must name which one so an alert is actionable.
func TestFailingDependencyReports503(t *testing.T) {
	h := New(
		okCheck("postgres"),
		Check{Name: "redis", Ping: func(context.Context) error {
			return errors.New("connection refused")
		}},
	)

	code, body := do(t, h)

	if code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", code)
	}
	if body.Status != "degraded" {
		t.Errorf("status = %q, want degraded", body.Status)
	}
	if body.Checks["postgres"] != "ok" {
		t.Errorf("checks[postgres] = %q, want ok", body.Checks["postgres"])
	}
	if body.Checks["redis"] != "connection refused" {
		t.Errorf("checks[redis] = %q, want the error text", body.Checks["redis"])
	}
}

// A hung dependency must fail within the timeout rather than hold the
// connection open until the monitor itself gives up.
func TestHungDependencyTimesOut(t *testing.T) {
	h := New(Check{Name: "postgres", Ping: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}})

	start := time.Now()
	code, body := do(t, h)

	if code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", code)
	}
	if elapsed := time.Since(start); elapsed > checkTimeout+time.Second {
		t.Errorf("took %s, want it to give up near %s", elapsed, checkTimeout)
	}
	if body.Checks["postgres"] == "ok" {
		t.Error("a hung dependency was reported as ok")
	}
}
