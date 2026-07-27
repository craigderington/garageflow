package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/mail"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/garageflow/api-go/internal/email"
)

type Service struct {
	db     *pgxpool.Pool
	rdb    *redis.Client
	secret string
	mailer email.Sender
	appURL string
}

func NewService(db *pgxpool.Pool, rdb *redis.Client, secret string, mailer email.Sender, appURL string) *Service {
	if mailer == nil {
		mailer = email.NewLogSender()
	}
	return &Service{db: db, rdb: rdb, secret: secret, mailer: mailer, appURL: appURL}
}

func (s *Service) GenerateMagicLink(ctx context.Context, email string) (string, error) {
	if _, err := mail.ParseAddress(email); err != nil {
		return "", fmt.Errorf("invalid email")
	}

	token := uuid.New().String()
	hash := sha256.Sum256([]byte(token + s.secret))
	code := hex.EncodeToString(hash[:])

	key := fmt.Sprintf("magic_link:%s", code)
	if err := s.rdb.Set(ctx, key, email, 15*time.Minute).Err(); err != nil {
		return "", fmt.Errorf("cache token: %w", err)
	}

	// Best-effort delivery: a failure to email must not block the dev/E2E
	// flow that relies on the code being returned to the caller.
	if err := s.sendMagicLinkEmail(ctx, email, code); err != nil {
		log.Printf("[auth] magic-link email send failed for %s: %v", email, err)
	}

	return code, nil
}

func (s *Service) sendMagicLinkEmail(ctx context.Context, email, code string) error {
	loginURL := fmt.Sprintf("%s/auth/verify?code=%s", s.appURL, url.QueryEscape(code))

	subject := "Your GarageFlow login link"
	text := fmt.Sprintf(
		"Sign in to GarageFlow by opening the link below:\n\n%s\n\n"+
			"Or enter this code in the app:\n\n%s\n\n"+
			"This link expires in 15 minutes. If you did not request it, you can ignore this email.\n",
		loginURL, code,
	)
	html := fmt.Sprintf(
		`<!DOCTYPE html><html><body style="font-family:Arial,Helvetica,sans-serif;color:#1a1a1a">`+
			`<h2 style="margin-bottom:8px">Sign in to GarageFlow</h2>`+
			`<p>Click the button below to sign in:</p>`+
			`<p><a href="%s" style="display:inline-block;padding:12px 20px;background:#f97316;color:#fff;text-decoration:none;border-radius:6px;font-weight:bold">Sign in</a></p>`+
			`<p>Or enter this code in the app:</p>`+
			`<p style="font-size:18px;font-weight:bold;letter-spacing:1px">%s</p>`+
			`<p style="color:#666;font-size:13px">This link expires in 15 minutes. If you did not request it, you can ignore this email.</p>`+
			`</body></html>`,
		loginURL, code,
	)

	return s.mailer.Send(ctx, email, subject, html, text)
}

func (s *Service) VerifyMagicLink(ctx context.Context, code string) (string, error) {
	key := fmt.Sprintf("magic_link:%s", code)
	email, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("invalid or expired token")
	}
	s.rdb.Del(ctx, key)

	var userID string
	var shopID string
	var role string
	err = s.db.QueryRow(ctx,
		`SELECT id, shop_id, role FROM users WHERE email = $1`, email,
	).Scan(&userID, &shopID, &role)
	if err != nil {
		return "", fmt.Errorf("user not found")
	}

	sessionToken := uuid.New().String()
	sessionKey := fmt.Sprintf("session:%s", sessionToken)
	sessionData := fmt.Sprintf(`{"uid":"%s","sid":"%s","role":"%s"}`, userID, shopID, role)
	if err := s.rdb.Set(ctx, sessionKey, sessionData, 24*time.Hour).Err(); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return sessionToken, nil
}

func (s *Service) ValidateSession(ctx context.Context, sessionToken string) (userID, shopID, role string, err error) {
	sessionKey := fmt.Sprintf("session:%s", sessionToken)
	data, err := s.rdb.Get(ctx, sessionKey).Result()
	if err != nil {
		return "", "", "", fmt.Errorf("invalid session")
	}
	var session struct {
		UID  string `json:"uid"`
		SID  string `json:"sid"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return "", "", "", fmt.Errorf("invalid session data")
	}
	return session.UID, session.SID, session.Role, nil
}

func (s *Service) CreateUser(ctx context.Context, shopID, email, name, role string) error {
	id := uuid.New().String()
	_, err := s.db.Exec(ctx,
		`INSERT INTO users (id, shop_id, email, name, role) VALUES ($1, $2, $3, $4, $5)`,
		id, shopID, email, name, role,
	)
	return err
}

func (s *Service) SetPassword(ctx context.Context, email, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE email = $2`,
		string(hash), email,
	)
	return err
}

func (s *Service) VerifyPassword(ctx context.Context, email, password string) (string, string, string, error) {
	var userID, shopID, role, hash string
	err := s.db.QueryRow(ctx,
		`SELECT id, shop_id, role, COALESCE(password_hash, '') FROM users WHERE email = $1`, email,
	).Scan(&userID, &shopID, &role, &hash)
	if err != nil {
		return "", "", "", fmt.Errorf("user not found")
	}
	if hash == "" {
		return "", "", "", fmt.Errorf("password not set")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", "", "", fmt.Errorf("invalid password")
	}
	return userID, shopID, role, nil
}

func generateRandomCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
