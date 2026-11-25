package clientip

import "net/netip"

type CloudflareI interface {
	Contains(ip netip.Addr) bool
}

// RequestI gives the addresses of a request; an absent or broken one is an
// invalid Addr.
type RequestI interface {
	ExtractRemoteAddrIP() netip.Addr
	// ExtractForwardedFor keeps an entry that is not an address as an invalid
	// Addr, so a reader walking from the right can tell the chain is broken.
	ExtractForwardedFor() []netip.Addr
	ExtractRealIP() netip.Addr
	ExtractCfConnectingIP() netip.Addr
}
