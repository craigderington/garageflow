package sms

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNoCredsUsesLogSender(t *testing.T) {
	s := New("", "", "", "")
	if s.Enabled() {
		t.Fatal("expected disabled log sender")
	}
	if err := s.Send(context.Background(), "+15555550100", "hi"); err != nil {
		t.Fatalf("log sender should not error: %v", err)
	}
}

func TestTwilioSendBuildsRequest(t *testing.T) {
	var auth, body, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"sid":"SM123"}`)
	}))
	defer srv.Close()

	s := New("ACxxx", "token", "+15555550000", srv.URL)
	if !s.Enabled() {
		t.Fatal("expected enabled twilio sender")
	}
	if err := s.Send(context.Background(), "+15555550100", "Your inspection: http://x/y"); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if !strings.Contains(path, "/Accounts/ACxxx/Messages.json") {
		t.Fatalf("unexpected path %q", path)
	}
	if !strings.HasPrefix(auth, "Basic ") {
		t.Fatalf("expected basic auth, got %q", auth)
	}
	for _, want := range []string{"To=%2B15555550100", "From=%2B15555550000", "Body="} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}
