package auth

import (
	"encoding/json"
	"net/http"

	"github.com/garageflow/api-go/internal/middleware"
)

type Handler struct {
	svc *Service
	// devCodes echoes the magic-link code back in the HTTP response. It exists
	// so local dev and the E2E suite can log in without reading email. It must
	// stay off in production: the code alone is enough to mint a session for
	// any known address, so returning it is an authentication bypass.
	devCodes bool
}

func NewHandler(svc *Service, devCodes bool) *Handler {
	return &Handler{svc: svc, devCodes: devCodes}
}

// magicLinkResponse builds the POST /auth/magic-link body, including the code
// only when dev codes are enabled.
func magicLinkResponse(code string, devCodes bool) map[string]string {
	resp := map[string]string{"status": "sent"}
	if devCodes {
		resp["code"] = code
	}
	return resp
}

type magicLinkRequest struct {
	Email string `json:"email"`
}

type verifyRequest struct {
	Code string `json:"code"`
}

func (h *Handler) MagicLink(w http.ResponseWriter, r *http.Request) {
	var req magicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	code, err := h.svc.GenerateMagicLink(r.Context(), req.Email)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(magicLinkResponse(code, h.devCodes))
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	sessionToken, err := h.svc.VerifyMagicLink(r.Context(), req.Code)
	if err != nil {
		http.Error(w, `{"error":"invalid code"}`, http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	json.NewEncoder(w).Encode(map[string]string{"session": sessionToken})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RevokeSession(r.Context(), middleware.GetSessionToken(r.Context())); err != nil {
		http.Error(w, `{"error":"logout failed"}`, http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
	})
	json.NewEncoder(w).Encode(map[string]string{"status": "logged_out"})
}

type passwordSetRequest struct {
	Password string `json:"password"`
}

func (h *Handler) SetPassword(w http.ResponseWriter, r *http.Request) {
	var req passwordSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if err := h.svc.SetPasswordForUser(r.Context(), middleware.GetUserID(r.Context()), req.Password); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "password_set"})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	userID, shopID, role, err := h.svc.VerifyPassword(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	sessionToken, err := h.svc.IssueSession(r.Context(), userID, shopID, role)
	if err != nil {
		http.Error(w, `{"error":"login failed"}`, http.StatusServiceUnavailable)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	json.NewEncoder(w).Encode(map[string]string{"session": sessionToken})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"user_id": middleware.GetUserID(r.Context()),
		"shop_id": middleware.GetShopID(r.Context()),
		"role":    middleware.GetUserRole(r.Context()),
	})
}
