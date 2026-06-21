package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config is the root configuration loaded from a YAML file and overridden by
// environment variables. Secrets (passwords, tokens) are ENV-only: their
// yaml tag is "-" so they can never be committed in plain text.
type Config struct {
	Env            string         `yaml:"env"              env:"APP_ENV"      env-default:"local"`
	Server         Server         `yaml:"server"`
	Database       Database       `yaml:"database"`
	Redis          Redis          `yaml:"redis"`
	Auth           Auth           `yaml:"auth"`
	RateLimit      RateLimit      `yaml:"rate_limit"`
	CircuitBreaker CircuitBreaker `yaml:"circuit_breaker"`
	EmailService   EmailService   `yaml:"email_service"`
}

// Server holds HTTP server tuning parameters.
type Server struct {
	Port            int           `yaml:"port"             env:"SERVER_PORT"`
	ReadTimeout     time.Duration `yaml:"read_timeout"     env:"SERVER_READ_TIMEOUT"`
	WriteTimeout    time.Duration `yaml:"write_timeout"    env:"SERVER_WRITE_TIMEOUT"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"SERVER_SHUTDOWN_TIMEOUT"`
}

// Addr returns the listen address in the form ":port".
func (s Server) Addr() string {
	return fmt.Sprintf(":%d", s.Port)
}

// Database holds MySQL connection parameters.
// Password is read exclusively from DATABASE_PASSWORD env variable.
type Database struct {
	Host            string        `yaml:"host"              env:"DATABASE_HOST"`
	Port            int           `yaml:"port"              env:"DATABASE_PORT"`
	Name            string        `yaml:"name"              env:"DATABASE_NAME"`
	User            string        `yaml:"user"              env:"DATABASE_USER"`
	Password        string        `yaml:"-"                 env:"DATABASE_PASSWORD"`
	MaxOpenConns    int           `yaml:"max_open_conns"    env:"DATABASE_MAX_OPEN_CONNS"`
	MaxIdleConns    int           `yaml:"max_idle_conns"    env:"DATABASE_MAX_IDLE_CONNS"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env:"DATABASE_CONN_MAX_LIFETIME"`
}

// Redis holds go-redis client parameters.
// Password is read exclusively from REDIS_PASSWORD env variable.
type Redis struct {
	Addr        string        `yaml:"addr"          env:"REDIS_ADDR"`
	Password    string        `yaml:"-"             env:"REDIS_PASSWORD"`
	DB          int           `yaml:"db"            env:"REDIS_DB"`
	PoolSize    int           `yaml:"pool_size"     env:"REDIS_POOL_SIZE"`
	TaskListTTL time.Duration `yaml:"task_list_ttl" env:"REDIS_TASK_LIST_TTL"`
}

// Auth holds JWT signing parameters.
// JWTSecret is read exclusively from AUTH_JWT_SECRET env variable.
type Auth struct {
	JWTSecret string        `yaml:"-"          env:"AUTH_JWT_SECRET"`
	JWTExpiry time.Duration `yaml:"jwt_expiry" env:"AUTH_JWT_EXPIRY"`
}

// RateLimit configures the per-user request rate limiter.
type RateLimit struct {
	RequestsPerMinute int `yaml:"requests_per_minute" env:"RATE_LIMIT_RPM"`
}

// CircuitBreaker configures the gobreaker used for external service calls.
type CircuitBreaker struct {
	MaxRequests uint32        `yaml:"max_requests" env:"CB_MAX_REQUESTS"`
	Interval    time.Duration `yaml:"interval"     env:"CB_INTERVAL"`
	Timeout     time.Duration `yaml:"timeout"      env:"CB_TIMEOUT"`
}

// EmailService holds the base URL for the mock email notification service.
type EmailService struct {
	BaseURL string `yaml:"base_url" env:"EMAIL_SERVICE_BASE_URL"`
}

// MustLoad reads the YAML file at path, then overlays values from environment
// variables. It panics on any read or parse error so misconfiguration is
// detected immediately at startup.
func MustLoad(path string) *Config {
	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		panic(fmt.Sprintf("config: %s", err))
	}
	return &cfg
}
