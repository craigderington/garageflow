package demo

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/garageflow/api-go/internal/email"
)

// Handler exposes the demo service over HTTP: provisioning a fresh demo shop
// and resuming one from an emailed return link.
type Handler struct {
	svc    *Service
	mailer email.Sender
	appURL string
}

// NewHandler builds a Handler. mailer must be the RAW, unwrapped sender — the
// demo's own return-link email is not customer outbound and must not be
// suppressed by GuardEmail (which fails closed with no shop in context yet).
func NewHandler(svc *Service, mailer email.Sender, appURL string) *Handler {
	return &Handler{svc: svc, mailer: mailer, appURL: appURL}
}

type startRequest struct {
	Email string `json:"email"`
}

// Start handles POST /demo: provisions (or resumes) a demo shop for the given
// email, signs the caller in via cookie, and best-effort emails a return
// link. Neither the session token nor the return token ever appears in the
// response body — only the cookie carries the session.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	ip := r.RemoteAddr
	sessionToken, returnToken, err := h.svc.Provision(r.Context(), req.Email, ip, r.UserAgent())
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	setSessionCookie(w, sessionToken)
	h.sendReturnLink(r, req.Email, returnToken)

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type resumeRequest struct {
	Token string `json:"token"`
}

// Resume handles POST /demo/resume: exchanges an emailed return token for a
// fresh session on the demo shop it points at.
func (h *Handler) Resume(w http.ResponseWriter, r *http.Request) {
	var req resumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	sessionToken, err := h.svc.Resume(r.Context(), req.Token)
	if err != nil {
		http.Error(w, `{"error":"invalid or expired demo link"}`, http.StatusUnauthorized)
		return
	}

	setSessionCookie(w, sessionToken)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// setSessionCookie sets the session cookie with exactly the attributes
// auth.Handler.Verify uses. A mismatch means the demo session is not
// recognised by the rest of the app.
func setSessionCookie(w http.ResponseWriter, sessionToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
}

// sendReturnLink best-effort emails the prospect a link back to their demo
// shop. Sent with h.mailer, which must be the raw, unwrapped sender: at this
// point there is no shop in the request context, so GuardEmail would fail
// closed and silently suppress it.
func (h *Handler) sendReturnLink(r *http.Request, to, returnToken string) {
	resumeURL := fmt.Sprintf("%s/demo/resume?token=%s", strings.TrimRight(h.appURL, "/"), url.QueryEscape(returnToken))
	subject := "Your GarageFlow demo shop"
	text := fmt.Sprintf(
		"Your demo shop is ready. Come back to it any time in the next 14 days:\n\n%s\n\n"+
			"It is preloaded with customers, vehicles, repair orders and an inspection to try.\n",
		resumeURL,
	)
	if err := h.mailer.Send(r.Context(), to, subject, "", text); err != nil {
		log.Printf("[demo] failed to send return link to %s: %v", to, err)
	}
}
