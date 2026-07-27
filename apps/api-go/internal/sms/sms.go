// Package sms sends text messages via Twilio. With no credentials it falls back
// to a no-op logging sender so local dev and E2E work without an account
// (mirrors internal/email and internal/payments dev modes).
package sms

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

type Sender interface {
	Send(ctx context.Context, to, body string) error
	Enabled() bool
}

// New returns a Twilio sender when account SID + auth token are set, else a
// logging no-op sender.
func New(accountSID, authToken, from, baseURL string) Sender {
	if accountSID == "" || authToken == "" || from == "" {
		return &LogSender{}
	}
	if baseURL == "" {
		baseURL = "https://api.twilio.com"
	}
	return &TwilioSender{
		sid:     accountSID,
		token:   authToken,
		from:    from,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

type LogSender struct{}

func (l *LogSender) Send(_ context.Context, to, body string) error {
	log.Printf("[sms] would send to=%s body=%q", to, body)
	return nil
}
func (l *LogSender) Enabled() bool { return false }

type TwilioSender struct {
	sid, token, from, baseURL string
	client                    *http.Client
}

func (t *TwilioSender) Enabled() bool { return true }

func (t *TwilioSender) Send(ctx context.Context, to, body string) error {
	if to == "" {
		return fmt.Errorf("missing recipient")
	}
	form := url.Values{}
	form.Set("To", to)
	form.Set("From", t.from)
	form.Set("Body", body)

	endpoint := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", t.baseURL, t.sid)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.SetBasicAuth(t.sid, t.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio send failed (%d): %s", resp.StatusCode, string(b))
	}
	return nil
}
