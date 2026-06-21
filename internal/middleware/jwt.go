package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey struct{}

var userIDKey = contextKey{}

// JWT returns a chi middleware that extracts and validates the Bearer token.
// If no token is present the request continues unauthenticated (userID not set in context).
// If a token is present but invalid the middleware responds 401.
func JWT(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				next.ServeHTTP(w, r)
				return
			}

			raw, ok := strings.CutPrefix(auth, "Bearer ")
			if !ok {
				http.Error(w, `{"message":"invalid authorization header"}`, http.StatusUnauthorized)
				return
			}

			tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})
			if err != nil || !tok.Valid {
				http.Error(w, `{"message":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			sub, err := tok.Claims.GetSubject()
			if err != nil {
				http.Error(w, `{"message":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}

			id, err := uuid.Parse(sub)
			if err != nil {
				http.Error(w, `{"message":"invalid token subject"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromCtx extracts the authenticated user's ID from the request context.
func UserIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

// ContextWithUserID returns a derived context with the given user ID set.
// Intended for use in tests that need to simulate an authenticated request.
func ContextWithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}
