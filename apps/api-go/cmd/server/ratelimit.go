package main

import (
	"context"
	"net"
	"net/http"
	"strings"
)

// peerIPCtxKey is the context key under which capturePeerIP stashes the
// connection's real remote address.
type peerIPCtxKey struct{}

// capturePeerIP records r.RemoteAddr — the actual TCP peer — into the request
// context.
//
// It MUST be registered BEFORE chimw.RealIP. RealIP overwrites r.RemoteAddr
// from True-Client-IP / X-Real-IP / X-Forwarded-For unconditionally and
// without verifying anything, so any rate limiter keying off RemoteAddr after
// RealIP has run (httprate.LimitByIP does exactly that) is defeated by
// rotating a single request header. That matters most on POST /demo, where
// every request costs a bcrypt hash plus a seeded shop that lives 14 days and
// triggers one unauthenticated outbound email to a caller-chosen address.
func capturePeerIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), peerIPCtxKey{}, hostOnly(r.RemoteAddr))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// keyByTrustedIP is the httprate key function for endpoints whose limit must
// not be bypassable by a request header.
//
// It starts from the pre-RealIP TCP peer. When that peer is one of our own
// reverse proxies (loopback in production — Apache proxies to 127.0.0.1:28301
// — or the docker bridge in compose), every connection shares one peer
// address, and keying on it alone would collapse the limit into a single
// global bucket. In that case it falls back to the LAST X-Forwarded-For entry:
// both mod_proxy_http and Caddy APPEND the connection's peer to that header,
// so the rightmost value is written by the trusted hop and anything a client
// supplies is pushed to the left and ignored. infra/apache/garageflow.conf
// additionally unsets any inbound X-Forwarded-For, so in production there is
// exactly one entry and it is the real client.
func keyByTrustedIP(r *http.Request) (string, error) {
	peer, _ := r.Context().Value(peerIPCtxKey{}).(string)
	if peer == "" {
		// capturePeerIP was not registered; r.RemoteAddr may already have been
		// rewritten by RealIP, but it is the only thing left to key on.
		peer = hostOnly(r.RemoteAddr)
	}
	if !isLocalProxy(peer) {
		return peer, nil
	}
	if fwd := lastForwardedFor(r); fwd != "" {
		return fwd, nil
	}
	return peer, nil
}

// lastForwardedFor returns the rightmost entry across every X-Forwarded-For
// header on the request. Values (not Get) because a client can send its own
// X-Forwarded-For header and a proxy may add a second one rather than
// extending the first — the trusted hop's entry is always the last of the last.
func lastForwardedFor(r *http.Request) string {
	values := r.Header.Values("X-Forwarded-For")
	if len(values) == 0 {
		return ""
	}
	parts := strings.Split(values[len(values)-1], ",")
	return strings.TrimSpace(parts[len(parts)-1])
}

// isLocalProxy reports whether addr looks like one of our own hops rather than
// a client on the internet. The API listens on loopback behind Apache in
// production and on a docker bridge behind Caddy in compose; it is never
// exposed directly to a private network, so treating those ranges as "a proxy
// spoke to us" is safe here.
func isLocalProxy(addr string) bool {
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

// hostOnly strips the port from a host:port pair, tolerating a bare host.
func hostOnly(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return strings.TrimSpace(remoteAddr)
	}
	return host
}
