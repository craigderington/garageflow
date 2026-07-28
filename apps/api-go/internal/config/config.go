package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port          string
	DatabaseURL   string
	RedisURL      string
	SessionSecret string
	// AuthDevCodes echoes magic-link codes back in the API response so local
	// dev and E2E can log in without email. Never enable in production.
	AuthDevCodes bool
	// AuthRateLimitPerMin caps unauthenticated /auth requests per client IP
	// each minute. 0 disables the limiter, which local dev and the E2E suite
	// need since they authenticate far faster than any real client.
	AuthRateLimitPerMin int
	// DemoRateLimitPerMin caps POST /demo per client IP. Lower than the auth
	// limit because provisioning a shop is far more expensive than a login
	// attempt. 0 disables the limit, matching AuthRateLimitPerMin.
	DemoRateLimitPerMin int
	SMTPHost            string
	SMTPPort            int
	SMTPUser            string
	SMTPPass            string
	SMTPFrom            string
	AppURL              string
	MinIOEndpoint       string
	MinIOAccessKey      string
	MinIOSecretKey      string
	MinIOBucket         string
	MailgunAPIKey       string
	MailgunDomain       string
	MailgunFrom         string
	MailgunBaseURL      string

	StripeSecretKey      string
	StripePublishableKey string
	StripeWebhookSecret  string
	StripeBaseURL        string

	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFrom       string
	TwilioBaseURL    string
}

func Load() *Config {
	return &Config{
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://garageflow:garageflow@localhost:5432/garageflow?sslmode=disable"),
		RedisURL:            getEnv("REDIS_URL", "redis://localhost:6379/0"),
		SessionSecret:       getEnv("SESSION_SECRET", "dev-secret-change-in-production"),
		AuthDevCodes:        getEnvBool("AUTH_DEV_CODES", false),
		AuthRateLimitPerMin: getEnvInt("AUTH_RATE_LIMIT_PER_MIN", 10),
		DemoRateLimitPerMin: getEnvInt("DEMO_RATE_LIMIT_PER_MIN", 3),
		SMTPHost:            getEnv("SMTP_HOST", "localhost"),
		SMTPPort:            getEnvInt("SMTP_PORT", 1025),
		SMTPUser:            getEnv("SMTP_USER", ""),
		SMTPPass:            getEnv("SMTP_PASS", ""),
		SMTPFrom:            getEnv("SMTP_FROM", "noreply@garageflow.app"),
		AppURL:              getEnv("APP_URL", "http://localhost:3000"),
		MinIOEndpoint:       getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:      getEnv("MINIO_ACCESS_KEY", "garageflow"),
		MinIOSecretKey:      getEnv("MINIO_SECRET_KEY", "garageflow123"),
		MinIOBucket:         getEnv("MINIO_BUCKET", "garageflow"),
		MailgunAPIKey:       getEnv("MAILGUN_API_KEY", ""),
		MailgunDomain:       getEnv("MAILGUN_DOMAIN", ""),
		MailgunFrom:         getEnv("MAILGUN_FROM", getEnv("SMTP_FROM", "noreply@garageflow.app")),
		MailgunBaseURL:      getEnv("MAILGUN_BASE_URL", "https://api.mailgun.net/v3"),

		StripeSecretKey:      getEnv("STRIPE_SECRET_KEY", ""),
		StripePublishableKey: getEnv("STRIPE_PUBLISHABLE_KEY", ""),
		StripeWebhookSecret:  getEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripeBaseURL:        getEnv("STRIPE_BASE_URL", "https://api.stripe.com"),

		TwilioAccountSID: getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:  getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioFrom:       getEnv("TWILIO_FROM", ""),
		TwilioBaseURL:    getEnv("TWILIO_BASE_URL", "https://api.twilio.com"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
