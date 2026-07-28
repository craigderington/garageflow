package realtime

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/garageflow/api-go/internal/middleware"
)

// subscribers reports how many streams are attached to a room, so tests can
// wait for the handler to actually subscribe before publishing. Publishing
// early is silently dropped — Publish is non-blocking by design.
func (h *Hub) subscribers(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[room])
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// serve starts the handler behind a real server, injecting shopID the way
// TenantMiddleware would. An empty shopID exercises the unauthenticated path.
func serve(t *testing.T, h *Hub, shopID string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shopID != "" {
			r = r.WithContext(middleware.WithShopID(r.Context(), shopID))
		}
		h.ServeEvents(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// The handler used to read the context with a bare string key, which never
// matched the typed key the middleware writes, and then panicked on an
// unchecked type assertion. Unauthenticated must be a clean 401.
func TestServeEventsRejectsMissingShop(t *testing.T) {
	srv := serve(t, NewHub(), "")

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestServeEventsStreamsPublishedEvent(t *testing.T) {
	h := NewHub()
	srv := serve(t, h, "shop-1")

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	waitFor(t, "subscription", func() bool { return h.subscribers("shop:shop-1") == 1 })

	// A different shop's room must not reach this subscriber.
	h.Publish("shop:shop-2", []byte(`{"kind":"other.tenant"}`))
	h.Publish("shop:shop-1", []byte(`{"kind":"ro.updated"}`))

	reader := bufio.NewReader(resp.Body)
	var frame []string
	for len(frame) < 2 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read: %v (got %v)", err, frame)
		}
		if line = strings.TrimRight(line, "\n"); line != "" {
			frame = append(frame, line)
		}
	}

	if frame[0] != "event: update" {
		t.Errorf("frame[0] = %q, want %q", frame[0], "event: update")
	}
	if want := `data: {"kind":"ro.updated"}`; frame[1] != want {
		t.Errorf("frame[1] = %q, want %q", frame[1], want)
	}
	if strings.Contains(strings.Join(frame, "\n"), "other.tenant") {
		t.Error("received an event published to a different shop")
	}
}

// An idle shop sends nothing for hours; without a heartbeat the proxy drops it.
func TestServeEventsHeartbeat(t *testing.T) {
	original := heartbeatInterval
	heartbeatInterval = 10 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = original })

	srv := serve(t, NewHub(), "shop-1")

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := strings.TrimRight(line, "\n"); got != ": ping" {
		t.Errorf("heartbeat = %q, want %q", got, ": ping")
	}
}

// Disconnecting must drop the subscriber, or an idle shop leaks a channel per
// reconnect until the room map grows without bound.
func TestServeEventsUnsubscribesOnDisconnect(t *testing.T) {
	h := NewHub()
	srv := serve(t, h, "shop-1")

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	waitFor(t, "subscription", func() bool { return h.subscribers("shop:shop-1") == 1 })

	resp.Body.Close()
	waitFor(t, "unsubscribe", func() bool { return h.subscribers("shop:shop-1") == 0 })
}
