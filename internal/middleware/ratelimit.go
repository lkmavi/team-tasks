package middleware

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type userLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit returns a per-user sliding-window rate limiter middleware.
// Authenticated requests are keyed by userID; unauthenticated ones by remote IP.
// The cleanup goroutine stops when ctx is canceled (pass the app lifecycle context).
func RateLimit(ctx context.Context, requestsPerMinute int) func(http.Handler) http.Handler {
	var (
		mu       sync.Mutex
		limiters = make(map[string]*userLimiter)
	)

	r := rate.Limit(float64(requestsPerMinute) / 60.0)
	burst := requestsPerMinute

	getLimiter := func(key string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		ul, ok := limiters[key]
		if !ok {
			ul = &userLimiter{limiter: rate.NewLimiter(r, burst)}
			limiters[key] = ul
		}
		ul.lastSeen = time.Now()
		return ul.limiter
	}

	startLimiterCleanup(ctx, &mu, limiters)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var key string
			if id, ok := UserIDFromCtx(r.Context()); ok {
				key = "u:" + id.String()
			} else {
				host, _, err := net.SplitHostPort(r.RemoteAddr)
				if err != nil {
					host = r.RemoteAddr
				}
				key = "ip:" + host
			}

			if !getLimiter(key).Allow() {
				http.Error(w, `{"message":"too many requests"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func startLimiterCleanup(ctx context.Context, mu *sync.Mutex, limiters map[string]*userLimiter) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				for key, ul := range limiters {
					if time.Since(ul.lastSeen) > 10*time.Minute {
						delete(limiters, key)
					}
				}
				mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
}
