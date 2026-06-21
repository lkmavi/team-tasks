package middleware

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/lkmavi/team-tasks/pkg/slogx"
)

const requestIDHeader = "X-Request-ID"

// RequestID assigns a unique request ID to every inbound request.
// If the caller already supplies X-Request-ID it is reused verbatim.
// The ID is written back to the response header and injected into the
// request context as a slog attribute so all downstream log calls —
// from middleware, handlers, services, and storages — carry it automatically.
// Use slogx.FromContext(ctx) anywhere in the call chain to get the enriched logger.
func RequestID(log *slogx.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(requestIDHeader)
			if requestID == "" {
				id, err := uuid.NewV7()
				if err != nil {
					http.Error(w, "internal server error", http.StatusInternalServerError)
					return
				}
				requestID = id.String()
			}
			w.Header().Set(requestIDHeader, requestID)

			reqLog := log.With(slog.String("request_id", requestID))
			ctx := slogx.ToContext(r.Context(), reqLog)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
