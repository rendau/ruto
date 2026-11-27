package clientip

import (
	"context"
	"net/netip"
)

type contextKey struct{}

// WithContext keeps the client address of a request for the code that needs
// it further down (the sign-in throttle).
func WithContext(ctx context.Context, ip netip.Addr) context.Context {
	return context.WithValue(ctx, contextKey{}, ip)
}

// FromContext is the address WithContext kept, an invalid Addr when none.
func FromContext(ctx context.Context) netip.Addr {
	ip, _ := ctx.Value(contextKey{}).(netip.Addr)
	return ip
}
