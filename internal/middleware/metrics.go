package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	appmetrics "github.com/lkmavi/team-tasks/internal/metrics"
)

// Metrics records Prometheus request duration and total count per route.
// Uses chi's route pattern (e.g. /api/v1/tasks/{id}) instead of the raw URL
// to avoid Prometheus label cardinality explosion from UUIDs.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		next.ServeHTTP(ww, r)

		// Prefer the matched route pattern; fall back to raw path only when chi
		// has no routing info (e.g. the /metrics endpoint itself).
		path := r.URL.Path
		if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
			path = rctx.RoutePattern()
		}

		method := r.Method
		status := fmt.Sprintf("%d", ww.Status())

		appmetrics.RequestsTotal.WithLabelValues(method, path, status).Inc()
		appmetrics.RequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
	})
}
