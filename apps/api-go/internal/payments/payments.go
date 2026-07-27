// Package payments integrates Stripe Checkout for collecting payment on
// approved estimates. It talks to the Stripe REST API directly (no SDK
// dependency). When no secret key is configured it runs in dev mode: it
// returns a simulated session so the payment flow is fully exercisable
// locally and in E2E without real Stripe credentials.
package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	secretKey     string
	webhookSecret string
	baseURL       string
	client        *http.Client
}

func NewService(secretKey, webhookSecret, baseURL string) *Service {
	if baseURL == "" {
		baseURL = "https://api.stripe.com"
	}
	return &Service{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		baseURL:       strings.TrimRight(baseURL, "/"),
		client:        &http.Client{Timeout: 20 * time.Second},
	}
}

// Enabled reports whether a real Stripe key is configured.
func (s *Service) Enabled() bool { return s.secretKey != "" }

type CheckoutSession struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Mode string `json:"mode"` // "stripe" or "dev"
}

// CreateCheckoutSession creates a Stripe Checkout Session for a one-off amount
// (in cents). metadata is attached so the webhook can reconcile the payment.
func (s *Service) CreateCheckoutSession(ctx context.Context, amountCents int64, currency, productName, successURL, cancelURL string, metadata map[string]string) (*CheckoutSession, error) {
	if !s.Enabled() {
		// Dev mode: no Stripe configured. Return a simulated session that points
		// straight at the success URL so the caller can complete the flow.
		return &CheckoutSession{ID: "dev_" + metadata["estimate_id"], URL: successURL, Mode: "dev"}, nil
	}

	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", currency)
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(amountCents, 10))
	form.Set("line_items[0][price_data][product_data][name]", productName)
	for k, v := range metadata {
		form.Set("metadata["+k+"]", v)
		form.Set("payment_intent_data[metadata]["+k+"]", v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.SetBasicAuth(s.secretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stripe request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("stripe checkout failed (%d): %s", resp.StatusCode, string(body))
	}
	var out struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode stripe response: %w", err)
	}
	return &CheckoutSession{ID: out.ID, URL: out.URL, Mode: "stripe"}, nil
}

// VerifyWebhook validates the Stripe-Signature header against the raw payload
// using the configured webhook secret. Returns the parsed event type and the
// metadata on the contained object. When no webhook secret is set, signature
// verification is skipped (dev mode).
func (s *Service) VerifyWebhook(payload []byte, sigHeader string) (eventType string, metadata map[string]string, err error) {
	if s.webhookSecret != "" {
		if !validSignature(payload, sigHeader, s.webhookSecret) {
			return "", nil, fmt.Errorf("invalid signature")
		}
	}
	var evt struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				Metadata map[string]string `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		return "", nil, fmt.Errorf("decode event: %w", err)
	}
	return evt.Type, evt.Data.Object.Metadata, nil
}

// validSignature implements Stripe's v1 signature scheme: the signed payload is
// "<t>.<rawBody>", HMAC-SHA256 with the webhook secret, compared to the v1 sig.
func validSignature(payload []byte, sigHeader, secret string) bool {
	var t, v1 string
	for _, part := range strings.Split(sigHeader, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			t = kv[1]
		case "v1":
			v1 = kv[1]
		}
	}
	if t == "" || v1 == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(t + "." + string(payload)))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(v1))
}
