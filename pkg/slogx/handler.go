package slogx

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// DynamicHandler is a middleware-style slog.Handler that supports runtime
// reconfiguration of level, format, masking, and removal rules without restarts.
// It caches the static handler chain (WithAttrs + WithGroup) and rebuilds it
// only when the configuration pointer changes.
type DynamicHandler struct {
	cfg *atomic.Pointer[Config]

	attrs  []slog.Attr
	groups []string

	cachedHandler       atomic.Pointer[slog.Handler]
	cachedConfigVersion atomic.Pointer[Config]
}

// Enabled reports whether the record should be logged at the current dynamic level.
func (h *DynamicHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.cfg.Load().Level
}

// Handle processes a log record, injecting context-derived attributes first.
func (h *DynamicHandler) Handle(ctx context.Context, r slog.Record) error {
	cfg := h.cfg.Load()

	var ctxAttrs []slog.Attr
	for _, key := range cfg.ContextKeys {
		if val := ctx.Value(key); val != nil {
			ctxAttrs = append(ctxAttrs, slog.Any(key, val))
		}
	}

	base := h.getOrBuildCachedHandler(cfg)
	if len(ctxAttrs) > 0 {
		base = base.WithAttrs(ctxAttrs)
	}
	return base.Handle(ctx, r)
}

func (h *DynamicHandler) getOrBuildCachedHandler(cfg *Config) slog.Handler {
	if h.cachedConfigVersion.Load() == cfg {
		if cached := h.cachedHandler.Load(); cached != nil {
			return *cached
		}
	}

	hOpts := &slog.HandlerOptions{
		Level:       cfg.Level,
		ReplaceAttr: h.getReplaceAttr(cfg),
	}

	var base slog.Handler
	if cfg.Format == FormatJSON {
		base = slog.NewJSONHandler(cfg.Output, hOpts)
	} else {
		base = slog.NewTextHandler(cfg.Output, hOpts)
	}

	if len(h.attrs) > 0 {
		base = base.WithAttrs(h.attrs)
	}
	for _, g := range h.groups {
		base = base.WithGroup(g)
	}

	h.cachedHandler.Store(&base)
	h.cachedConfigVersion.Store(cfg)
	return base
}

// WithAttrs returns a new DynamicHandler with additional attributes.
func (h *DynamicHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &DynamicHandler{cfg: h.cfg, attrs: newAttrs, groups: h.groups}
}

// WithGroup returns a new DynamicHandler with an additional attribute group.
func (h *DynamicHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name
	return &DynamicHandler{cfg: h.cfg, attrs: h.attrs, groups: newGroups}
}

func (h *DynamicHandler) getReplaceAttr(cfg *Config) func([]string, slog.Attr) slog.Attr {
	return func(_ []string, a slog.Attr) slog.Attr {
		if _, shouldRemove := cfg.RemoveKeys[a.Key]; shouldRemove {
			return slog.Attr{}
		}
		if mType, ok := cfg.MaskKeys[a.Key]; ok {
			a.Value = slog.AnyValue(cfg.Masker.Mask(a.Value.Any(), mType))
			return a
		}
		if a.Key == slog.LevelKey {
			if lvl, ok := a.Value.Any().(slog.Level); ok {
				a.Value = slog.StringValue(getLevelName(lvl, cfg.LevelNames))
			}
		}
		return a
	}
}
