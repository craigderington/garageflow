package realtime

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/garageflow/api-go/internal/middleware"
)

type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]map[chan []byte]struct{}),
	}
}

func (h *Hub) Subscribe(room string) chan []byte {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan []byte, 64)
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[chan []byte]struct{})
	}
	h.rooms[room][ch] = struct{}{}
	return ch
}

func (h *Hub) Unsubscribe(room string, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subs, ok := h.rooms[room]; ok {
		delete(subs, ch)
		close(ch)
		if len(subs) == 0 {
			delete(h.rooms, room)
		}
	}
}

func (h *Hub) Publish(room string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if subs, ok := h.rooms[room]; ok {
		for ch := range subs {
			select {
			case ch <- data:
			default:
			}
		}
	}
}

// heartbeatInterval keeps the connection warm. Apache's ProxyTimeout and most
// intermediaries drop a stream that goes quiet, and a shop with no activity is
// silent for hours, so an idle stream must still emit something.
// var, not const, so tests can shorten it.
var heartbeatInterval = 20 * time.Second

// ServeEvents streams shop-scoped events to the caller as Server-Sent Events.
//
// Despite the historical /ws path this is not a WebSocket: there is no upgrade
// handshake, just a long-lived text/event-stream response.
func (h *Hub) ServeEvents(w http.ResponseWriter, r *http.Request) {
	// Typed key via GetShopID. Reading the context with a bare string "shop_id"
	// silently misses, because context.Value compares key type as well as value.
	shopID := middleware.GetShopID(r.Context())
	if shopID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	room := "shop:" + shopID

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Belt and braces: the server already runs with WriteTimeout disabled, but
	// clear any inherited write deadline too. Best-effort on purpose — chi wraps
	// the ResponseWriter, and if the wrapper does not support Unwrap this
	// returns ErrNotSupported, which is harmless given WriteTimeout is 0.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Ignored by Apache (which needs flushpackets=on in the vhost) but respected
	// by nginx-family proxies, so the stream survives either front end.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Subscribe only after the headers are away, so a client that disconnects
	// during setup cannot leave a subscriber stranded in the room.
	ch := h.Subscribe(room)
	defer h.Unsubscribe(room, ch)

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			// A comment frame: valid SSE that clients ignore.
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case msg, ok := <-ch:
			if !ok {
				return
			}
			// msg is already JSON from Publish, and json.Marshal never emits a
			// raw newline, so it cannot break the one-frame-per-line framing.
			if _, err := fmt.Fprintf(w, "event: update\ndata: %s\n\n", msg); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
