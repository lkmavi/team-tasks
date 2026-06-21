package slogx

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

// FromContext extracts the Logger from the provided context.
// If no Logger is found, it returns a new Logger instance wrapping the slog.Default() logger.
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(ctxKey{}).(*Logger); ok {
		return l
	}
	return &Logger{Logger: slog.Default()}
}

// ToContext injects the Logger into the provided context and returns the resulting context.
func ToContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}
