package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lkmavi/team-tasks/internal/config"
)

func TestServerAddr(t *testing.T) {
	tests := []struct {
		port int
		want string
	}{
		{8080, ":8080"},
		{443, ":443"},
		{0, ":0"},
	}
	for _, tc := range tests {
		s := config.Server{Port: tc.port}
		assert.Equal(t, tc.want, s.Addr())
	}
}

const testYAML = `
env: test
server:
  port: 9090
  read_timeout: 5s
  write_timeout: 10s
  shutdown_timeout: 15s
database:
  host: localhost
  port: 3306
  name: testdb
  user: testuser
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: 1h
redis:
  addr: localhost:6379
  db: 1
  pool_size: 5
  task_list_ttl: 10m
auth:
  jwt_expiry: 24h
rate_limit:
  requests_per_minute: 60
circuit_breaker:
  max_requests: 3
  interval: 30s
  timeout: 10s
email_service:
  base_url: http://localhost:8025
`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestMustLoad_ParsesAllFields(t *testing.T) {
	cfg := config.MustLoad(writeTempConfig(t, testYAML))

	assert.Equal(t, "test", cfg.Env)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, 5*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 10*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 15*time.Second, cfg.Server.ShutdownTimeout)

	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 3306, cfg.Database.Port)
	assert.Equal(t, "testdb", cfg.Database.Name)
	assert.Equal(t, "testuser", cfg.Database.User)
	assert.Equal(t, 10, cfg.Database.MaxOpenConns)
	assert.Equal(t, 5, cfg.Database.MaxIdleConns)
	assert.Equal(t, time.Hour, cfg.Database.ConnMaxLifetime)

	assert.Equal(t, "localhost:6379", cfg.Redis.Addr)
	assert.Equal(t, 1, cfg.Redis.DB)
	assert.Equal(t, 5, cfg.Redis.PoolSize)
	assert.Equal(t, 10*time.Minute, cfg.Redis.TaskListTTL)

	assert.Equal(t, 24*time.Hour, cfg.Auth.JWTExpiry)
	assert.Equal(t, 60, cfg.RateLimit.RequestsPerMinute)
	assert.Equal(t, uint32(3), cfg.CircuitBreaker.MaxRequests)
	assert.Equal(t, 30*time.Second, cfg.CircuitBreaker.Interval)
	assert.Equal(t, 10*time.Second, cfg.CircuitBreaker.Timeout)
	assert.Equal(t, "http://localhost:8025", cfg.EmailService.BaseURL)
}

func TestMustLoad_SecretsFromEnvOnly(t *testing.T) {
	t.Setenv("DATABASE_PASSWORD", "db-secret")
	t.Setenv("REDIS_PASSWORD", "redis-secret")
	t.Setenv("AUTH_JWT_SECRET", "jwt-secret")

	cfg := config.MustLoad(writeTempConfig(t, testYAML))

	assert.Equal(t, "db-secret", cfg.Database.Password)
	assert.Equal(t, "redis-secret", cfg.Redis.Password)
	assert.Equal(t, "jwt-secret", cfg.Auth.JWTSecret)
}

func TestMustLoad_EnvOverridesYAML(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SERVER_PORT", "8443")
	t.Setenv("RATE_LIMIT_RPM", "200")

	cfg := config.MustLoad(writeTempConfig(t, testYAML))

	assert.Equal(t, "production", cfg.Env)
	assert.Equal(t, 8443, cfg.Server.Port)
	assert.Equal(t, 200, cfg.RateLimit.RequestsPerMinute)
}

func TestMustLoad_DefaultEnv(t *testing.T) {
	if val, ok := os.LookupEnv("APP_ENV"); ok {
		os.Unsetenv("APP_ENV")
		t.Cleanup(func() { os.Setenv("APP_ENV", val) })
	}

	cfg := config.MustLoad(writeTempConfig(t, `
server:
  port: 8080
database:
  host: localhost
  port: 3306
redis:
  addr: localhost:6379
`))

	assert.Equal(t, "local", cfg.Env)
}

func TestMustLoad_PanicsOnMissingFile(t *testing.T) {
	assert.Panics(t, func() {
		config.MustLoad("/nonexistent/path/config.yaml")
	})
}

func TestMustLoad_PanicsOnInvalidYAML(t *testing.T) {
	// go-yaml rejects tab characters used as indentation
	path := writeTempConfig(t, "server:\n\tport: 8080")
	assert.Panics(t, func() {
		config.MustLoad(path)
	})
}
