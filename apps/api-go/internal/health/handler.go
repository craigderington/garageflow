// Package health exposes a dependency-aware liveness endpoint.
//
// The point is that a wedged process fails the check rather than answering
// "fine" while its database connection is dead — an endpoint that only proves
// the HTTP server accepted a connection tells a monitor almost nothing.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// checkTimeout bounds the whole check. A monitor polls on a short interval, so
// a hung dependency has to fail fast rather than pile up in-flight requests.
const checkTimeout = 2 * time.Second

// Check is one dependency to probe. Ping must respect the context deadline.
type Check struct {
	Name string
	Ping func(context.Context) error
}

type Handler struct {
	checks []Check
}

func New(checks ...Check) *Handler {
	return &Handler{checks: checks}
}

type response struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// ServeHTTP probes every dependency concurrently and reports 200 only if all
// of them answer. A failure yields 503 with the offending error per dependency,
// so an alert says which one broke without needing a log dive.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), checkTimeout)
	defer cancel()

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results = make(map[string]string, len(h.checks))
		healthy = true
	)

	for _, c := range h.checks {
		wg.Add(1)
		go func(c Check) {
			defer wg.Done()
			status := "ok"
			if err := c.Ping(ctx); err != nil {
				status = err.Error()
			}
			mu.Lock()
			defer mu.Unlock()
			results[c.Name] = status
			if status != "ok" {
				healthy = false
			}
		}(c)
	}
	wg.Wait()

	body := response{Status: "ok", Checks: results}
	code := http.StatusOK
	if !healthy {
		body.Status = "degraded"
		code = http.StatusServiceUnavailable
	}

	// Never let a proxy or monitor serve a stale verdict.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
}
