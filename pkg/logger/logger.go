// Package logger provides a pre-configured slogx logger for the service.
// Use New to create an instance based on the runtime environment.
// For context propagation use slogx.ToContext / slogx.FromContext directly.
package logger

import (
	"log/slog"

	"github.com/lkmavi/team-tasks/pkg/slogx"
)

// New returns a logger tuned for the given environment:
//   - "prod"  — JSON format, INFO level
//   - "dev"   — text format, DEBUG level
//   - "local" — text format, DEBUG level (default)
//
// Sensitive field names (password, token, secret) are stripped from all
// output regardless of environment.
func New(env string) *slogx.Logger {
	format := slogx.FormatText
	level := slog.LevelDebug

	if env == "prod" {
		format = slogx.FormatJSON
		level = slog.LevelInfo
	}

	return slogx.New(
		slogx.WithOutput(nil),
		slogx.WithFormat(format),
		slogx.WithLevel(level),
		slogx.WithRemoval(
			slogx.NewRemovalSet().
				Add("password").
				Add("token").
				Add("secret"),
		),
		slogx.WithContextKeys("request_id", "trace_id"),
	)
}

// Nop returns a logger that discards all output. Useful in tests.
func Nop() *slogx.Logger {
	return slogx.NewNop()
}
