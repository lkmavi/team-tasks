package mysql

import (
	"fmt"
	"log/slog"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"github.com/lkmavi/team-tasks/internal/config"
)

const (
	maxRetries    = 10
	retryInterval = 3 * time.Second
)

// New opens a *sqlx.DB connection to MySQL and configures the connection pool.
// It retries up to maxRetries times with a fixed interval to tolerate the MySQL
// container starting after the app container.
func New(cfg *config.Database) (*sqlx.DB, error) {
	dsn := buildDSN(cfg)

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		db, err := sqlx.Open("mysql", dsn)
		if err != nil {
			// Open only validates the DSN — a parse error won't fix itself on retry.
			return nil, fmt.Errorf("mysql: invalid dsn: %w", err)
		}

		if err = db.Ping(); err == nil {
			db.SetMaxOpenConns(cfg.MaxOpenConns)
			db.SetMaxIdleConns(cfg.MaxIdleConns)
			db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
			return db, nil
		}

		_ = db.Close()
		lastErr = err

		slog.Warn("mysql: not ready, retrying",
			"attempt", attempt,
			"of", maxRetries,
			"backoff", retryInterval,
			"error", err.Error(),
		)
		time.Sleep(retryInterval)
	}

	return nil, fmt.Errorf("mysql: unavailable after %d attempts: %w", maxRetries, lastErr)
}

// buildDSN assembles the go-sql-driver/mysql connection string.
// parseTime=true maps DATETIME columns to time.Time; loc=UTC ensures all
// timestamps are interpreted in UTC regardless of the server timezone.
func buildDSN(cfg *config.Database) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=UTC",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)
}
