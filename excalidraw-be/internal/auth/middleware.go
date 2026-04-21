package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const ContextKeyUserID contextKey = "userID"

func JWTMiddleware(authService *AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"Missing authorization header","code":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				http.Error(w, `{"error":"Invalid authorization format","code":"invalid_auth"}`, http.StatusUnauthorized)
				return
			}

			claims, err := authService.ValidateAccessToken(parts[1])
			if err != nil {
				http.Error(w, `{"error":"Invalid or expired token","code":"invalid_token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), "userID", claims.UserID)
			ctx = context.WithValue(ctx, "username", claims.Username)
			ctx = context.WithValue(ctx, "email", claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value("userID").(string); ok {
		return v
	}
	return ""
}
