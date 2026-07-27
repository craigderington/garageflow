package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDevModeReturnsSimulatedSession(t *testing.T) {
	s := NewService("", "", "")
	sess, err := s.CreateCheckoutSession(context.Background(), 1000, "usd", "Estimate", "http://app/success", "http://app/cancel", map[string]string{"estimate_id": "e1"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if sess.Mode != "dev" {
		t.Fatalf("expected dev mode, got %q", sess.Mode)
	}
	if sess.URL != "http://app/success" {
		t.Fatalf("dev url should be success url, got %q", sess.URL)
	}
}

func TestCreateCheckoutSessionHitsStripe(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/checkout/sessions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"cs_test_123","url":"https://checkout.stripe.com/c/pay/cs_test_123"}`)
	}))
	defer srv.Close()

	s := NewService("sk_test_abc", "", srv.URL)
	sess, err := s.CreateCheckoutSession(context.Background(), 24000, "usd", "Estimate 42", "http://app/s", "http://app/c", map[string]string{"estimate_id": "e42"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if sess.Mode != "stripe" || sess.ID != "cs_test_123" {
		t.Fatalf("unexpected session: %+v", sess)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Fatalf("expected basic auth, got %q", gotAuth)
	}
	if !strings.Contains(gotBody, "unit_amount%5D=24000") {
		t.Fatalf("expected amount in body, got %q", gotBody)
	}
	if !strings.Contains(gotBody, "metadata%5Bestimate_id%5D=e42") {
		t.Fatalf("expected metadata in body, got %q", gotBody)
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	secret := "whsec_test"
	s := NewService("sk_test", secret, "")
	payload := []byte(`{"type":"checkout.session.completed","data":{"object":{"metadata":{"estimate_id":"e7"}}}}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("123." + string(payload)))
	sig := "t=123,v1=" + hex.EncodeToString(mac.Sum(nil))

	evtType, meta, err := s.VerifyWebhook(payload, sig)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if evtType != "checkout.session.completed" || meta["estimate_id"] != "e7" {
		t.Fatalf("unexpected: %s %v", evtType, meta)
	}

	if _, _, err := s.VerifyWebhook(payload, "t=123,v1=deadbeef"); err == nil {
		t.Fatal("expected signature failure")
	}
}
