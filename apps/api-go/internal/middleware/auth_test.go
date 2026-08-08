package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRBACMiddleware(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
	handler := RBACMiddleware("owner", "admin")(next)

	t.Run("rejects technician", func(t *testing.T) {
		called = false
		req := httptest.NewRequest(http.MethodPost, "/users", nil)
		req = req.WithContext(WithUserRole(req.Context(), "technician"))
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusForbidden || called {
			t.Fatalf("code=%d called=%v, want 403 and not called", res.Code, called)
		}
	})

	t.Run("allows admin", func(t *testing.T) {
		called = false
		req := httptest.NewRequest(http.MethodPost, "/users", nil)
		req = req.WithContext(WithUserRole(req.Context(), "admin"))
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK || !called {
			t.Fatalf("code=%d called=%v, want 200 and called", res.Code, called)
		}
	})
}
