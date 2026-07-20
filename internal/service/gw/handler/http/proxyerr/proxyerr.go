package proxyerr

import "context"

type ctxKeyT struct{}

var ctxKey ctxKeyT

type holder struct {
	reason string
}

// WithHolder installs a mutable holder into ctx so a downstream proxy handler
// can report why it failed. The request-log middleware reads it back after the
// handler returns to log a human-readable cause instead of a bare status code.
func WithHolder(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey, &holder{})
}

// Set records the failure reason if a holder is present in ctx.
func Set(ctx context.Context, reason string) {
	if h, ok := ctx.Value(ctxKey).(*holder); ok {
		h.reason = reason
	}
}

// Get returns the recorded reason, or "" if none.
func Get(ctx context.Context) string {
	if h, ok := ctx.Value(ctxKey).(*holder); ok {
		return h.reason
	}
	return ""
}
