package realtime

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
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

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	shopID := r.Context().Value("shop_id").(string)
	room := "shop:" + shopID

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.Subscribe(room)
	defer h.Unsubscribe(room, ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var payload map[string]interface{}
			json.Unmarshal(msg, &payload)
			data, _ := json.Marshal(payload)
			event := chi.URLParam(r, "event")
			if event == "" {
				event = "update"
			}
			log.Printf("SSE: %s %s", event, data)
			flusher.Flush()
		}
	}
}
