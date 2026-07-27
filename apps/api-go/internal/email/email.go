// Package email provides transactional email sending for GarageFlow.
//
// It exposes a small Sender interface with two implementations: a Mailgun
// REST API client (using only the standard library) and a no-op logging
// sender used for local development and E2E tests where no Mailgun
// credentials are configured.
package email

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Sender sends a single transactional email.
type Sender interface {
	Send(ctx context.Context, to, subject, html, text string) error
}

// LogSender is a no-op Sender that logs what it would have sent. It is used
// when Mailgun is not configured so that local dev and E2E tests can run
// without real credentials.
type LogSender struct{}

// NewLogSender returns a no-op logging Sender.
func NewLogSender() *LogSender {
	return &LogSender{}
}

// Send logs the message instead of delivering it.
func (s *LogSender) Send(ctx context.Context, to, subject, html, text string) error {
	log.Printf("[email] would send to=%s subject=%q", to, subject)
	return nil
}

// MailgunSender sends email via the Mailgun REST API using net/http.
type MailgunSender struct {
	apiKey  string
	domain  string
	from    string
	baseURL string
	client  *http.Client
}

// NewMailgunSender constructs a Mailgun-backed Sender.
//
// baseURL should be the Mailgun API base (e.g. "https://api.mailgun.net/v3"
// or "https://api.eu.mailgun.net/v3" for the EU region); a trailing slash is
// trimmed automatically.
func NewMailgunSender(apiKey, domain, from, baseURL string) *MailgunSender {
	return &MailgunSender{
		apiKey:  apiKey,
		domain:  domain,
		from:    from,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Send delivers an email through Mailgun's messages endpoint.
func (s *MailgunSender) Send(ctx context.Context, to, subject, html, text string) error {
	endpoint := fmt.Sprintf("%s/%s/messages", s.baseURL, s.domain)

	form := url.Values{}
	form.Set("from", s.from)
	form.Set("to", to)
	form.Set("subject", subject)
	if text != "" {
		form.Set("text", text)
	}
	if html != "" {
		form.Set("html", html)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build mailgun request: %w", err)
	}
	req.SetBasicAuth("api", s.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send mailgun request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("mailgun returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}
