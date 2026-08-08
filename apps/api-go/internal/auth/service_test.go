package auth

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// testRedis returns a client pointed at the compose Redis, skipping when it is
// unreachable so the unit suite still runs on a bare checkout.
func testRedis(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable at %s: %v", addr, err)
	}
	return rdb, func() { rdb.Close() }
}

func TestIssueSessionRoundTrips(t *testing.T) {
	rdb, cleanup := testRedis(t)
	defer cleanup()

	svc := &Service{rdb: rdb}

	token, err := svc.IssueSession(context.Background(), "user-1", "shop-1", "owner")
	if err != nil {
		t.Fatalf("IssueSession: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	uid, sid, role, err := svc.ValidateSession(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if uid != "user-1" || sid != "shop-1" || role != "owner" {
		t.Errorf("got (%q,%q,%q), want (user-1,shop-1,owner)", uid, sid, role)
	}
}

func TestRevokeSessionInvalidatesToken(t *testing.T) {
	rdb, cleanup := testRedis(t)
	defer cleanup()
	svc := &Service{rdb: rdb}
	token, err := svc.IssueSession(context.Background(), "user-2", "shop-2", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeSession(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := svc.ValidateSession(context.Background(), token); err == nil {
		t.Fatal("revoked session still validates")
	}
}
