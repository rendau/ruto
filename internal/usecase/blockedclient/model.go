package blockedclient

import "time"

type CreateReq struct {
	// an address or a network in CIDR notation
	Ip     string
	Reason string
	// 0 = without end (admins only)
	Duration time.Duration
}

// RemoveReq addresses the block by Id or, when Id is empty, by the network it
// covers (the active block of exactly that network).
type RemoveReq struct {
	Id     string
	Ip     string
	Reason string
}
