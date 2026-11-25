package clientip

import "net/netip"

type CloudflareI interface {
	Contains(ip netip.Addr) bool
}
