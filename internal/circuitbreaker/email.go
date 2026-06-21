package circuitbreaker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sony/gobreaker"

	"github.com/lkmavi/team-tasks/internal/config"
)

// EmailClient sends team invitation emails through a mock email service.
// Calls are wrapped in a circuit breaker: if the service is down, the breaker
// opens and SendInvite returns an error immediately rather than waiting for
// a timeout on every request.
type EmailClient struct {
	cb      *gobreaker.CircuitBreaker
	baseURL string
	http    *http.Client
}

// NewEmailClient creates an EmailClient with a circuit breaker configured from cbCfg.
func NewEmailClient(cfg config.EmailService, cbCfg config.CircuitBreaker) *EmailClient {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "email-service",
		MaxRequests: cbCfg.MaxRequests,
		Interval:    cbCfg.Interval,
		Timeout:     cbCfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	})
	return &EmailClient{
		cb:      cb,
		baseURL: cfg.BaseURL,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *EmailClient) SendInvite(ctx context.Context, to, teamName string) error {
	if c.baseURL == "" {
		return nil
	}

	_, err := c.cb.Execute(func() (any, error) {
		payload, err := json.Marshal(map[string]string{"to": to, "team_name": teamName})
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/send", bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("email service unreachable: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("email service: status %d", resp.StatusCode)
		}
		return struct{}{}, nil
	})
	return err
}
