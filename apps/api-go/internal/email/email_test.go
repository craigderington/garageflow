package email

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogSender_Send(t *testing.T) {
	s := NewLogSender()
	if err := s.Send(context.Background(), "user@example.com", "Hello", "<p>hi</p>", "hi"); err != nil {
		t.Fatalf("LogSender.Send returned error: %v", err)
	}
}

func TestMailgunSender_Send(t *testing.T) {
	var (
		gotMethod   string
		gotPath     string
		gotUser     string
		gotPass     string
		gotForm     map[string]string
		gotPassOK   bool
		contentType string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotUser, gotPass, gotPassOK = r.BasicAuth()
		contentType = r.Header.Get("Content-Type")

		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		gotForm = map[string]string{}
		for k := range r.PostForm {
			gotForm[k] = r.PostForm.Get(k)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"<test>","message":"Queued"}`))
	}))
	defer srv.Close()

	s := NewMailgunSender("secret-key", "mg.example.com", "noreply@example.com", srv.URL+"/v3")
	err := s.Send(context.Background(), "user@example.com", "Subject Line", "<p>html</p>", "plain text")
	if err != nil {
		t.Fatalf("MailgunSender.Send returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v3/mg.example.com/messages" {
		t.Errorf("path = %q, want /v3/mg.example.com/messages", gotPath)
	}
	if !gotPassOK || gotUser != "api" || gotPass != "secret-key" {
		t.Errorf("basic auth = (%q, %q, ok=%v), want (api, secret-key, true)", gotUser, gotPass, gotPassOK)
	}
	if !strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		t.Errorf("content-type = %q, want application/x-www-form-urlencoded", contentType)
	}

	wantForm := map[string]string{
		"from":    "noreply@example.com",
		"to":      "user@example.com",
		"subject": "Subject Line",
		"html":    "<p>html</p>",
		"text":    "plain text",
	}
	for k, want := range wantForm {
		if got := gotForm[k]; got != want {
			t.Errorf("form[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestMailgunSender_Send_TrimsTrailingSlash(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewMailgunSender("k", "mg.example.com", "f@example.com", srv.URL+"/v3/")
	if err := s.Send(context.Background(), "u@example.com", "s", "", "t"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if gotPath != "/v3/mg.example.com/messages" {
		t.Errorf("path = %q, want /v3/mg.example.com/messages", gotPath)
	}
}

func TestMailgunSender_Send_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer srv.Close()

	s := NewMailgunSender("bad", "mg.example.com", "f@example.com", srv.URL+"/v3")
	err := s.Send(context.Background(), "u@example.com", "s", "<p>h</p>", "t")
	if err == nil {
		t.Fatal("expected error on non-2xx status, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %v, want it to mention status 401", err)
	}
}
