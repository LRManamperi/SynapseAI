package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/synapseai/platform/pkg/jwt"
)

type contextKey string

const UserContextKey contextKey = "user"

type UserContext struct {
	UserID string
	Email  string
	Role   string
}

// AuthMiddleware validates JWT tokens from requests
func AuthMiddleware(jwtManager *jwt.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "authorization header required", http.StatusUnauthorized)
				return
			}

			// Extract token from "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}

			token := parts[1]
			claims, err := jwtManager.ValidateToken(token)
			if err != nil {
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Add user context to request
			userCtx := &UserContext{
				UserID: claims.UserID,
				Email:  claims.Email,
				Role:   claims.Role,
			}

			ctx := context.WithValue(r.Context(), UserContextKey, userCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext retrieves user information from request context
func GetUserFromContext(ctx context.Context) (*UserContext, bool) {
	user, ok := ctx.Value(UserContextKey).(*UserContext)
	return user, ok
}

// CORSMiddleware handles CORS headers
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple logging - in production use structured logger
		println(r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
