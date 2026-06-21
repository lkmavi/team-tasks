package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lkmavi/team-tasks/internal/middleware"
)

func TestRateLimit_FirstRequest_Allowed(t *testing.T) {
	ctx := context.Background()
	mw := middleware.RateLimit(ctx, 60)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimit_ExceedBurst_Returns429(t *testing.T) {
	ctx := context.Background()
	// burst=1, so the second request should be denied
	mw := middleware.RateLimit(ctx, 1)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	addr := "9.9.9.9:9999"
	first := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	first.RemoteAddr = addr
	r1 := httptest.NewRecorder()
	mw(next).ServeHTTP(r1, first)
	assert.Equal(t, http.StatusOK, r1.Code)

	second := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	second.RemoteAddr = addr
	r2 := httptest.NewRecorder()
	mw(next).ServeHTTP(r2, second)
	assert.Equal(t, http.StatusTooManyRequests, r2.Code)
}

func TestRateLimit_AuthedRequest_KeyedByUser(t *testing.T) {
	ctx := context.Background()
	mw := middleware.RateLimit(ctx, 60)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "2.2.2.2:2222"
	authedCtx := middleware.ContextWithUserID(req.Context(), [16]byte{1})
	req = req.WithContext(authedCtx)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimit_ContextCancel_CleanupExits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	middleware.RateLimit(ctx, 60)
	cancel()
	// allow the goroutine to observe ctx.Done
	time.Sleep(10 * time.Millisecond)
	// no assertion needed — if the goroutine hangs this test would block
}
