package circuitbreaker_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lkmavi/team-tasks/internal/circuitbreaker"
	"github.com/lkmavi/team-tasks/internal/config"
)

func cbCfg() config.CircuitBreaker {
	return config.CircuitBreaker{MaxRequests: 5, Interval: time.Second, Timeout: time.Second}
}

func TestEmailClient_EmptyBaseURL_NoOp(t *testing.T) {
	c := circuitbreaker.NewEmailClient(config.EmailService{BaseURL: ""}, cbCfg())
	require.NoError(t, c.SendInvite(context.Background(), "a@b.com", "Team"))
}

func TestEmailClient_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/send", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := circuitbreaker.NewEmailClient(config.EmailService{BaseURL: srv.URL}, cbCfg())
	require.NoError(t, c.SendInvite(context.Background(), "a@b.com", "MyTeam"))
}

func TestEmailClient_ServerError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := circuitbreaker.NewEmailClient(config.EmailService{BaseURL: srv.URL}, cbCfg())
	assert.Error(t, c.SendInvite(context.Background(), "a@b.com", "MyTeam"))
}

func TestEmailClient_Unreachable_ReturnsError(t *testing.T) {
	c := circuitbreaker.NewEmailClient(config.EmailService{BaseURL: "http://127.0.0.1:1"}, cbCfg())
	assert.Error(t, c.SendInvite(context.Background(), "a@b.com", "MyTeam"))
}

// TestEmailClient_InvalidURL covers the http.NewRequestWithContext error branch
// (null byte makes the URL invalid).
func TestEmailClient_InvalidURL_ReturnsError(t *testing.T) {
	c := circuitbreaker.NewEmailClient(config.EmailService{BaseURL: "http://\x00"}, cbCfg())
	assert.Error(t, c.SendInvite(context.Background(), "a@b.com", "MyTeam"))
}
