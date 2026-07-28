package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

// spoofHeaders are every header chimw.RealIP will believe.
var spoofHeaders = []string{"True-Client-IP", "X-Real-IP", "X-Forwarded-For"}

// The whole point of the fix: a caller on one connection must not be able to
// buy itself extra requests by rotating a header. Each subtest drives the real
// middleware chain (capturePeerIP -> RealIP -> limiter) so the ordering is
// covered too, not just the key function in isolation.
func TestDemoLimiterIsNotDefeatedBySpoofedHeaders(t *testing.T) {
	for _, header := range spoofHeaders {
		t.Run(header, func(t *testing.T) {
			h := limitedHandler(2)

			var lastStatus int
			for i := 0; i < 5; i++ {
				req := httptest.NewRequest(http.MethodPost, "/demo", nil)
				req.RemoteAddr = "203.0.113.9:44321" // one connection throughout
				req.Header.Set(header, fakeClientIP(i))
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				lastStatus = rec.Code
			}

			if lastStatus != http.StatusTooManyRequests {
				t.Errorf("after 5 requests from one peer rotating %s, status = %d, want 429", header, lastStatus)
			}
		})
	}
}

// The positive control: distinct real peers must not share a bucket, or one
// prospect signing up locks out everyone else.
func TestDemoLimiterSeparatesDistinctPeers(t *testing.T) {
	h := limitedHandler(2)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/demo", nil)
		req.RemoteAddr = fakeClientIP(i) + ":44321"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d from a fresh peer got %d, want 200", i, rec.Code)
		}
	}
}

// Behind the loopback reverse proxy every peer is 127.0.0.1, so the limiter
// falls back to the trusted hop's X-Forwarded-For entry. A client that prefixes
// its own values must still be bucketed by the entry the proxy appended.
func TestDemoLimiterUsesTheProxyAppendedForwardedForEntry(t *testing.T) {
	h := limitedHandler(2)

	var lastStatus int
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/demo", nil)
		req.RemoteAddr = "127.0.0.1:44321"
		// The client tried to look like a different caller each time; the proxy
		// appended the real peer last.
		req.Header.Set("X-Forwarded-For", fakeClientIP(i)+", 198.51.100.7")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		lastStatus = rec.Code
	}

	if lastStatus != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 — the client-supplied leftmost X-Forwarded-For entry was trusted", lastStatus)
	}
}

// Two genuinely different clients behind the same proxy must not share a
// bucket.
func TestDemoLimiterSeparatesClientsBehindTheSameProxy(t *testing.T) {
	h := limitedHandler(2)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/demo", nil)
		req.RemoteAddr = "127.0.0.1:44321"
		req.Header.Set("X-Forwarded-For", fakeClientIP(i))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d from a distinct proxied client got %d, want 200", i, rec.Code)
		}
	}
}

// A second X-Forwarded-For header line must not shadow the proxy's entry:
// Header.Get would return only the first one.
func TestLastForwardedForReadsTheFinalHeaderLine(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/demo", nil)
	req.Header.Add("X-Forwarded-For", "1.1.1.1")
	req.Header.Add("X-Forwarded-For", "198.51.100.7")

	if got := lastForwardedFor(req); got != "198.51.100.7" {
		t.Errorf("lastForwardedFor = %q, want the trusted hop's entry 198.51.100.7", got)
	}
}

// limitedHandler builds the same chain main.go registers on /demo.
func limitedHandler(perMin int) http.Handler {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	limiter := httprate.Limit(perMin, time.Minute, httprate.WithKeyFuncs(keyByTrustedIP))
	return capturePeerIP(chimw.RealIP(limiter(ok)))
}

// fakeClientIP returns a distinct TEST-NET-1 address. Deliberately not a
// private range: isLocalProxy must see these as internet clients, not hops.
func fakeClientIP(i int) string {
	return "192.0.2." + strconv.Itoa(i+1)
}
