package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/lkmavi/team-tasks/internal/middleware"
)

func TestMetrics_FallbackToRawPath_WhenNoRouteCtx(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/some/path", http.NoBody)
	rec := httptest.NewRecorder()
	middleware.Metrics(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMetrics_UsesRoutePattern_WhenChiCtxPresent(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	r := chi.NewRouter()
	r.Use(middleware.Metrics)
	r.Get("/items/{id}", next)

	req := httptest.NewRequest(http.MethodGet, "/items/123", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}
