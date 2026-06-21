package slogx

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
)

// Logger wraps slog.Logger with atomic configuration updates.
// It allows changing level, format, and sanitization rules at runtime.
type Logger struct {
	*slog.Logger
	cfgPtr *atomic.Pointer[Config]
}

// New creates a Logger with the provided options.
func New(opts ...Option) *Logger {
	o := defaultOptions()
	for _, fn := range opts {
		if fn != nil {
			fn(o)
		}
	}

	ptr := &atomic.Pointer[Config]{}
	ptr.Store(o.initialConfig)

	handler := &DynamicHandler{cfg: ptr}

	return &Logger{
		Logger: slog.New(handler),
		cfgPtr: ptr,
	}
}

// NewNop creates a logger that discards all output.
func NewNop() *Logger {
	return New(WithOutput(io.Discard))
}

// SetupDefault initializes a Logger and sets it as the global slog default.
func SetupDefault(opts ...Option) {
	l := New(opts...)
	slog.SetDefault(l.Logger)
}

// With returns a derived Logger with additional attributes, sharing the same config.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{Logger: l.Logger.With(args...), cfgPtr: l.cfgPtr}
}

// WithGroup returns a grouped Logger that shares the same config pointer.
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{Logger: l.Logger.WithGroup(name), cfgPtr: l.cfgPtr}
}

// UpdateConfig atomically applies fn to a copy of the current configuration.
func (l *Logger) UpdateConfig(fn func(*Config)) {
	newCfg := l.cfgPtr.Load().Clone()
	fn(newCfg)
	l.cfgPtr.Store(newCfg)
}

// SetLevel is a convenience method to update the logging threshold.
func (l *Logger) SetLevel(lvl slog.Level) {
	l.UpdateConfig(func(c *Config) { c.Level = lvl })
}

// TraceContext logs a message at LevelTrace.
func (l *Logger) TraceContext(ctx context.Context, msg string, args ...any) {
	l.Log(ctx, LevelTrace, msg, args...)
}

// FatalContext logs a message at LevelFatal and exits with code 1.
func (l *Logger) FatalContext(ctx context.Context, msg string, args ...any) {
	l.Log(ctx, LevelFatal, msg, args...)
	os.Exit(1)
}

// Err creates a structured slog.Attr for an error value.
func Err(err error) slog.Attr {
	if err == nil {
		return slog.String("error", "nil")
	}
	return slog.String("error", err.Error())
}
