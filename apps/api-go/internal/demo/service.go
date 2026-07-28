// Package demo also implements the provisioning service: turning a captured
// email into a populated, throwaway shop; resuming that shop from the token
// emailed to the prospect; and sweeping shops whose 14 days have lapsed.
package demo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// SessionIssuer is the subset of *auth.Service the demo service depends on.
// Defined locally (rather than importing internal/auth) so tests can supply a
// fake and need no Redis.
type SessionIssuer interface {
	IssueSession(ctx context.Context, userID, shopID, role string) (string, error)
}

// Service provisions and resumes demo shops.
type Service struct {
	pool     *pgxpool.Pool
	sessions SessionIssuer
	ttl      time.Duration
}

// NewService builds a Service. ttl is how long a freshly provisioned demo
// shop lives before SweepExpired removes it.
func NewService(pool *pgxpool.Pool, sessions SessionIssuer, ttl time.Duration) *Service {
	return &Service{pool: pool, sessions: sessions, ttl: ttl}
}

// queryRower is the read subset both *pgxpool.Pool and pgx.Tx satisfy, so
// findLiveDemo can run either as a fast pre-transaction check or as the
// re-check inside a held advisory lock.
type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Provision turns an email address into a session for a populated demo shop.
// A prospect who has an unexpired demo already gets that shop back rather
// than a second one; anyone else gets a freshly seeded shop. Returns the
// session token to sign the caller in immediately and the return token to
// email so they can come back later.
func (s *Service) Provision(ctx context.Context, email, ip, userAgent string) (sessionToken, returnToken string, err error) {
	// Store the parsed address, not the raw input: mail.ParseAddress accepts
	// `Prospect <p@example.com>`, and persisting that verbatim would make the
	// same prospect look like two different leads and put a display name into
	// the To: header of every email we send them.
	parsed, err := mail.ParseAddress(strings.TrimSpace(email))
	if err != nil {
		return "", "", fmt.Errorf("invalid email")
	}
	email = normalizeEmail(parsed.Address)

	shopID, userID, role, returnToken, err := s.provisionOrReuse(ctx, email, ip, userAgent)
	if err != nil {
		return "", "", err
	}

	// Issue the session AFTER any transaction has committed: a failed commit
	// must never leave a session pointing at a rolled-back shop.
	token, err := s.sessions.IssueSession(ctx, userID, shopID, role)
	if err != nil {
		return "", "", fmt.Errorf("issue session: %w", err)
	}
	return token, returnToken, nil
}

// provisionOrReuse finds or creates the demo shop for email. Two concurrent
// callers for the same not-yet-provisioned email would otherwise both miss
// the fast lookup below and each run the create path, leaving two shops with
// the demo_leads upsert silently pointing at whichever committed last. A
// Postgres advisory lock, transaction-scoped and keyed on the email, plus a
// re-check once it's held, closes that window: the loser blocks until the
// winner commits, then finds the winner's row instead of creating a second.
func (s *Service) provisionOrReuse(ctx context.Context, email, ip, userAgent string) (shopID, userID, role, returnToken string, err error) {
	// Fast path: skip opening a transaction and taking the lock for the
	// common case of a returning prospect who already has a live demo.
	if sid, uid, r, tok, ok, err := findLiveDemo(ctx, s.pool, email); err != nil {
		return "", "", "", "", fmt.Errorf("lookup existing lead: %w", err)
	} else if ok {
		if _, err := s.pool.Exec(ctx,
			`UPDATE demo_leads SET last_seen_at = NOW() WHERE email = $1`, email); err != nil {
			return "", "", "", "", fmt.Errorf("update lead: %w", err)
		}
		return sid, uid, r, tok, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", "", "", "", fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	// Serialize concurrent Provision calls for the same email. Blocks until
	// any other transaction holding the same key has committed or rolled
	// back.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, email); err != nil {
		return "", "", "", "", fmt.Errorf("acquire provisioning lock: %w", err)
	}

	// Re-check now that we hold the lock: a concurrent caller may have
	// committed a shop for this email while we were waiting for it. The lock
	// alone only serializes the callers — this re-check is what stops the
	// second one from creating a duplicate shop.
	if sid, uid, r, tok, ok, err := findLiveDemo(ctx, tx, email); err != nil {
		return "", "", "", "", fmt.Errorf("recheck existing lead: %w", err)
	} else if ok {
		if _, err := tx.Exec(ctx,
			`UPDATE demo_leads SET last_seen_at = NOW() WHERE email = $1`, email); err != nil {
			return "", "", "", "", fmt.Errorf("update lead: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return "", "", "", "", fmt.Errorf("commit: %w", err)
		}
		return sid, uid, r, tok, nil
	}

	// 16 hex characters (64 bits). 8 was a birthday collision waiting to
	// surface as a unique-violation on shops.slug for an unlucky prospect.
	slugSuffix, err := randomHex(8)
	if err != nil {
		return "", "", "", "", fmt.Errorf("generate slug: %w", err)
	}
	slug := "demo-" + slugSuffix
	if err := tx.QueryRow(ctx,
		`INSERT INTO shops (name, slug, is_demo, expires_at) VALUES ($1, $2, TRUE, NOW() + $3::interval) RETURNING id`,
		"Demo Shop", slug, s.ttl.String(),
	).Scan(&shopID); err != nil {
		return "", "", "", "", fmt.Errorf("insert shop: %w", err)
	}

	passwordHash, err := randomBcryptHash()
	if err != nil {
		return "", "", "", "", fmt.Errorf("generate password: %w", err)
	}
	// The owner row gets a synthetic address, NEVER the prospect's. users is
	// only UNIQUE(shop_id, email), while auth resolves identity with a bare
	// `WHERE email = $1`. Storing the submitted address here would let anyone
	// POST /demo with a real customer's address and plant a second users row
	// under it — locking that customer out of password login and pointing
	// their magic link at the attacker's demo shop. The prospect's real
	// address lives in demo_leads.email and nowhere else.
	ownerEmail := "owner-" + uuid.New().String() + "@demo.invalid"
	if err := tx.QueryRow(ctx,
		`INSERT INTO users (shop_id, email, name, role, password_hash) VALUES ($1, $2, $3, 'owner', $4) RETURNING id`,
		shopID, ownerEmail, "Demo Owner", passwordHash,
	).Scan(&userID); err != nil {
		return "", "", "", "", fmt.Errorf("insert owner user: %w", err)
	}

	if err := Seed(ctx, tx, shopID); err != nil {
		return "", "", "", "", fmt.Errorf("seed shop: %w", err)
	}

	returnToken, err = randomHexToken()
	if err != nil {
		return "", "", "", "", fmt.Errorf("generate return token: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO demo_leads (email, shop_id, return_token, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email) DO UPDATE
		SET shop_id = EXCLUDED.shop_id, return_token = EXCLUDED.return_token, last_seen_at = NOW()`,
		email, shopID, returnToken, ip, userAgent,
	); err != nil {
		return "", "", "", "", fmt.Errorf("upsert demo lead: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", "", "", "", fmt.Errorf("commit: %w", err)
	}
	return shopID, userID, "owner", returnToken, nil
}

// findLiveDemo looks up the shop, owner user, and return token for an
// existing, unexpired demo shop belonging to email. ok is false (with a nil
// err) when there is none yet. q is either the pool (fast pre-lock check) or
// a tx already holding the per-email advisory lock (authoritative re-check).
func findLiveDemo(ctx context.Context, q queryRower, email string) (shopID, userID, role, returnToken string, ok bool, err error) {
	err = q.QueryRow(ctx, `
		SELECT s.id, u.id, u.role, l.return_token
		FROM demo_leads l
		JOIN shops s ON s.id = l.shop_id
		JOIN users u ON u.shop_id = s.id AND u.role = 'owner'
		WHERE l.email = $1 AND s.is_demo = TRUE AND s.expires_at > NOW()
		LIMIT 1`, email,
	).Scan(&shopID, &userID, &role, &returnToken)
	if err == pgx.ErrNoRows {
		return "", "", "", "", false, nil
	}
	if err != nil {
		return "", "", "", "", false, err
	}
	return shopID, userID, role, returnToken, true, nil
}

// Resume turns an emailed return token into a fresh session for the demo
// shop it points at, as long as that shop hasn't expired.
func (s *Service) Resume(ctx context.Context, returnToken string) (sessionToken string, err error) {
	var shopID, userID, role string
	err = s.pool.QueryRow(ctx, `
		SELECT s.id, u.id, u.role
		FROM demo_leads l
		JOIN shops s ON s.id = l.shop_id
		JOIN users u ON u.shop_id = s.id AND u.role = 'owner'
		WHERE l.return_token = $1 AND s.id IS NOT NULL AND s.is_demo = TRUE AND s.expires_at > NOW()`, returnToken,
	).Scan(&shopID, &userID, &role)
	if err != nil {
		return "", fmt.Errorf("unknown or expired token")
	}

	if _, err := s.pool.Exec(ctx,
		`UPDATE demo_leads SET verified_at = COALESCE(verified_at, NOW()), last_seen_at = NOW() WHERE return_token = $1`,
		returnToken,
	); err != nil {
		return "", fmt.Errorf("update lead: %w", err)
	}

	token, err := s.sessions.IssueSession(ctx, userID, shopID, role)
	if err != nil {
		return "", fmt.Errorf("issue session: %w", err)
	}
	return token, nil
}

// SweepExpired deletes every demo shop past its expiry. The is_demo = TRUE
// predicate is the only thing stopping this from deleting a paying
// customer's shop — never widen it. demo_leads.shop_id references shops
// ON DELETE SET NULL, so the lead (and mailing-list value) survives.
func (s *Service) SweepExpired(ctx context.Context) (int, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM shops WHERE is_demo = TRUE AND expires_at IS NOT NULL AND expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("sweep demo shops: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// normalizeEmail lowercases and trims an address so "Prospect@Example.com"
// and "prospect@example.com" are treated as one lead — demo_leads.email is
// UNIQUE and case-sensitive at the DB level.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// randomBcryptHash bcrypt-hashes 32 crypto/rand bytes. The plaintext is
// discarded immediately: this password exists only so a users row satisfies
// tenant middleware, never so anyone can sign in with it.
func randomBcryptHash() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword(buf, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// randomHexToken returns 32 random hex characters from crypto/rand — the
// credential embedded in the resume link, so it must not be predictable.
func randomHexToken() (string, error) { return randomHex(16) }

// randomHex returns 2*n hex characters read from crypto/rand.
func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
