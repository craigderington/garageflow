package middleware

import (
	"context"
	"net/http"
	"strings"
)

type SessionValidator interface {
	ValidateSession(ctx context.Context, sessionToken string) (userID, shopID, role string, err error)
}

func AuthMiddleware(authSvc SessionValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var sessionToken string

			cookie, err := r.Cookie("session")
			if err == nil && cookie.Value != "" {
				sessionToken = cookie.Value
			}

			if sessionToken == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					sessionToken = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if sessionToken == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			userID, shopID, role, err := authSvc.ValidateSession(r.Context(), sessionToken)
			if err != nil {
				http.Error(w, `{"error":"invalid session"}`, http.StatusUnauthorized)
				return
			}

			ctx := WithShopID(r.Context(), shopID)
			ctx = WithUserID(ctx, userID)
			ctx = WithUserRole(ctx, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RBACMiddleware(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := GetUserRole(r.Context())
			for _, allowed := range allowedRoles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		})
	}
}
