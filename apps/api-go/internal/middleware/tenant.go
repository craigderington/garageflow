package middleware

import (
	"context"
	"net/http"
)

type CtxKey string

const (
	ShopIDKey   CtxKey = "shop_id"
	UserIDKey   CtxKey = "user_id"
	UserRoleKey CtxKey = "user_role"
)

func GetShopID(ctx context.Context) string {
	v, _ := ctx.Value(ShopIDKey).(string)
	return v
}

func GetUserID(ctx context.Context) string {
	v, _ := ctx.Value(UserIDKey).(string)
	return v
}

func GetUserRole(ctx context.Context) string {
	v, _ := ctx.Value(UserRoleKey).(string)
	return v
}

func GetCtxVal(ctx context.Context, key CtxKey) string {
	v, _ := ctx.Value(key).(string)
	return v
}

func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shopID := GetShopID(r.Context())
		if shopID == "" {
			http.Error(w, `{"error":"tenant required"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func WithShopID(ctx context.Context, shopID string) context.Context {
	return context.WithValue(ctx, ShopIDKey, shopID)
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

func WithUserRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, UserRoleKey, role)
}
