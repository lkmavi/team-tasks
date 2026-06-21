package slogx

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Format defines the log output format.
type Format int

// RemoveMap is a set of attribute keys to exclude from log output.
type RemoveMap map[string]struct{}

const (
	// FormatText is a human-readable key=value format.
	FormatText Format = iota
	// FormatJSON is a structured JSON format.
	FormatJSON
)

// Config holds the atomic logger configuration state.
type Config struct {
	Level       slog.Level
	Format      Format
	Output      io.Writer
	MaskKeys    MaskMap
	RemoveKeys  RemoveMap
	LevelNames  LevelNames
	Masker      Masker
	ContextKeys []string
}

// Clone returns a deep copy of Config for thread-safe updates.
func (c *Config) Clone() *Config {
	newCfg := *c

	newCfg.MaskKeys = make(MaskMap, len(c.MaskKeys))
	for k, v := range c.MaskKeys {
		newCfg.MaskKeys[k] = v
	}

	newCfg.RemoveKeys = make(RemoveMap, len(c.RemoveKeys))
	for k, v := range c.RemoveKeys {
		newCfg.RemoveKeys[k] = v
	}

	newCfg.LevelNames = make(LevelNames, len(c.LevelNames))
	for k, v := range c.LevelNames {
		newCfg.LevelNames[k] = v
	}

	newCfg.ContextKeys = make([]string, len(c.ContextKeys))
	copy(newCfg.ContextKeys, c.ContextKeys)

	return &newCfg
}

type options struct {
	initialConfig *Config
}

// Option is a functional configuration parameter for logger initialization.
type Option func(*options)

// WithOutput sets the output destination.
func WithOutput(w io.Writer) Option {
	return func(o *options) {
		if w != nil {
			o.initialConfig.Output = w
		}
	}
}

// WithFormat sets the log output format (Text or JSON).
func WithFormat(f Format) Option {
	return func(o *options) {
		o.initialConfig.Format = f
	}
}

// ParseFormat converts a string to a Format, defaulting to FormatText.
func ParseFormat(s string) Format {
	if strings.ToLower(s) == "json" {
		return FormatJSON
	}
	return FormatText
}

// WithLevel sets the initial logging threshold.
func WithLevel(l slog.Level) Option {
	return func(o *options) {
		o.initialConfig.Level = l
	}
}

// WithMaskKey associates a single attribute key with a MaskType.
func WithMaskKey(key string, mType MaskType) Option {
	return func(o *options) {
		o.initialConfig.MaskKeys[key] = mType
	}
}

// WithMaskKeys applies a batch of masking rules from a MaskMap.
func WithMaskKeys(keys MaskMap) Option {
	return func(o *options) {
		for k, v := range keys {
			o.initialConfig.MaskKeys[k] = v
		}
	}
}

// WithMaskRules applies masking rules from a MaskRules builder.
func WithMaskRules(r *MaskRules) Option {
	return func(o *options) {
		if r == nil {
			return
		}
		for k, v := range r.rules {
			o.initialConfig.MaskKeys[k] = v
		}
	}
}

// WithMasker replaces the default masking logic with a custom Masker.
func WithMasker(m Masker) Option {
	return func(o *options) {
		if m != nil {
			o.initialConfig.Masker = m
		}
	}
}

// WithRemoval registers all keys in a RemovalSet for removal from log output.
func WithRemoval(set *RemovalSet) Option {
	return func(o *options) {
		for _, k := range set.Keys() {
			o.initialConfig.RemoveKeys[k] = struct{}{}
		}
	}
}

// WithLevelNames customizes the string representation of log levels.
func WithLevelNames(m LevelNames) Option {
	return func(o *options) {
		for k, v := range m {
			o.initialConfig.LevelNames[k] = v
		}
	}
}

// WithContextKeys registers keys to be automatically extracted from context and logged.
func WithContextKeys(keys ...string) Option {
	return func(o *options) {
		o.initialConfig.ContextKeys = append(o.initialConfig.ContextKeys, keys...)
	}
}

func defaultOptions() *options {
	ln := make(LevelNames, len(defaultLevelNames))
	for k, v := range defaultLevelNames {
		ln[k] = v
	}
	return &options{
		initialConfig: &Config{
			Level:      LevelTrace,
			Format:     FormatText,
			Output:     os.Stdout,
			MaskKeys:   make(MaskMap),
			RemoveKeys: make(RemoveMap),
			LevelNames: ln,
			Masker:     &DefaultMasker{},
		},
	}
}
