package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lkmavi/team-tasks/internal/middleware"
)

const testSecret = "test-secret-key"

func signedToken(sub, secret string) string {
	claims := jwt.MapClaims{
		"sub": sub,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString([]byte(secret))
	return signed
}

func jwtMiddleware() func(http.Handler) http.Handler {
	return middleware.JWT(testSecret)
}

func TestJWT_NoAuthHeader_PassesThrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, ok := middleware.UserIDFromCtx(r.Context())
		assert.False(t, ok, "no userID expected when no token")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	jwtMiddleware()(next).ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestJWT_NonBearerPrefix_Returns401(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Token somevalue")
	rec := httptest.NewRecorder()

	jwtMiddleware()(http.NotFoundHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWT_InvalidToken_Returns401(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer notavalidjwt")
	rec := httptest.NewRecorder()

	jwtMiddleware()(http.NotFoundHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWT_ValidToken_NonUUIDSubject_Returns401(t *testing.T) {
	tok := signedToken("not-a-uuid", testSecret)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	jwtMiddleware()(http.NotFoundHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWT_ValidToken_UUIDSubject_InjectsUserID(t *testing.T) {
	userID := uuid.New()
	tok := signedToken(userID.String(), testSecret)

	var gotID uuid.UUID
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := middleware.UserIDFromCtx(r.Context())
		require.True(t, ok)
		gotID = id
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	jwtMiddleware()(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, userID, gotID)
}

func TestContextWithUserID_RoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := middleware.ContextWithUserID(t.Context(), id)
	got, ok := middleware.UserIDFromCtx(ctx)
	assert.True(t, ok)
	assert.Equal(t, id, got)
}
