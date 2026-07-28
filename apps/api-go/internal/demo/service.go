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

// Provision turns an email address into a session for a populated demo shop.
// A prospect who has an unexpired demo already gets that shop back rather
// than a second one; anyone else gets a freshly seeded shop. Returns the
// session token to sign the caller in immediately and the return token to
// email so they can come back later.
func (s *Service) Provision(ctx context.Context, email, ip, userAgent string) (sessionToken, returnToken string, err error) {
	email = normalizeEmail(email)
	if _, err := mail.ParseAddress(email); err != nil {
		return "", "", fmt.Errorf("invalid email")
	}

	// An existing, unexpired demo for this email is reused rather than
	// duplicated — a prospect who submits twice must not accumulate shops.
	var shopID, userID, role, existingToken string
	err = s.pool.QueryRow(ctx, `
		SELECT s.id, u.id, u.role, l.return_token
		FROM demo_leads l
		JOIN shops s ON s.id = l.shop_id
		JOIN users u ON u.shop_id = s.id AND u.role = 'owner'
		WHERE l.email = $1 AND s.is_demo = TRUE AND s.expires_at > NOW()
		LIMIT 1`, email,
	).Scan(&shopID, &userID, &role, &existingToken)
	switch err {
	case nil:
		if _, err := s.pool.Exec(ctx,
			`UPDATE demo_leads SET last_seen_at = NOW() WHERE email = $1`, email); err != nil {
			return "", "", fmt.Errorf("update lead: %w", err)
		}
		token, err := s.sessions.IssueSession(ctx, userID, shopID, role)
		if err != nil {
			return "", "", fmt.Errorf("issue session: %w", err)
		}
		return token, existingToken, nil
	case pgx.ErrNoRows:
		// Fall through to provisioning a new shop.
	default:
		return "", "", fmt.Errorf("lookup existing lead: %w", err)
	}

	newShopID, newUserID, newToken, err := s.provisionNewShop(ctx, email, ip, userAgent)
	if err != nil {
		return "", "", err
	}

	// Issue the session AFTER the transaction commits: a failed commit must
	// never leave a session pointing at a rolled-back shop.
	token, err := s.sessions.IssueSession(ctx, newUserID, newShopID, "owner")
	if err != nil {
		return "", "", fmt.Errorf("issue session: %w", err)
	}
	return token, newToken, nil
}

// provisionNewShop creates the shop, its owner user, and the seed data in one
// transaction, then upserts the demo_leads row. It returns before a session
// is issued so the caller can do that only after this commits.
func (s *Service) provisionNewShop(ctx context.Context, email, ip, userAgent string) (shopID, userID, returnToken string, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	slug := "demo-" + uuid.New().String()[:8]
	if err := tx.QueryRow(ctx,
		`INSERT INTO shops (name, slug, is_demo, expires_at) VALUES ($1, $2, TRUE, NOW() + $3::interval) RETURNING id`,
		"Demo Shop", slug, s.ttl.String(),
	).Scan(&shopID); err != nil {
		return "", "", "", fmt.Errorf("insert shop: %w", err)
	}

	passwordHash, err := randomBcryptHash()
	if err != nil {
		return "", "", "", fmt.Errorf("generate password: %w", err)
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO users (shop_id, email, name, role, password_hash) VALUES ($1, $2, $3, 'owner', $4) RETURNING id`,
		shopID, email, "Demo Owner", passwordHash,
	).Scan(&userID); err != nil {
		return "", "", "", fmt.Errorf("insert owner user: %w", err)
	}

	if err := Seed(ctx, tx, shopID); err != nil {
		return "", "", "", fmt.Errorf("seed shop: %w", err)
	}

	returnToken, err = randomHexToken()
	if err != nil {
		return "", "", "", fmt.Errorf("generate return token: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO demo_leads (email, shop_id, return_token, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email) DO UPDATE
		SET shop_id = EXCLUDED.shop_id, return_token = EXCLUDED.return_token, last_seen_at = NOW()`,
		email, shopID, returnToken, ip, userAgent,
	); err != nil {
		return "", "", "", fmt.Errorf("upsert demo lead: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", "", "", fmt.Errorf("commit: %w", err)
	}
	return shopID, userID, returnToken, nil
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
		WHERE l.return_token = $1 AND s.id IS NOT NULL AND s.expires_at > NOW()`, returnToken,
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
func randomHexToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
