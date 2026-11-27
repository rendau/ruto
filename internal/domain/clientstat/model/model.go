// Package model holds the per-client request counters gateways report in
// their heartbeats; core keeps them in 5-minute buckets.
package model

import "time"

const (
	KindRequests = "requests"
	KindBlocked  = "blocked"
	KindLimited  = "limited"
	KindTrusted  = "trusted"
)

// ClientStat is one row of a gateway report, or of a stored bucket. Ip is the
// client address (an IPv6 client as its /64), the network of a trusted client
// (then Trusted is its name), or "" for the clients summed together.
type ClientStat struct {
	Ip       string
	Trusted  string
	Requests int64
	// CORS preflights among Requests that the gateway answered itself: no
	// rate limit counts them
	Preflights int64
	Blocked    int64
	Limited    int64
	LastRule   string
}

// Point is the sum of one step of a timeline; Ts is the start of the step.
type Point struct {
	Ts       time.Time
	Requests int64
	Blocked  int64
	Limited  int64
	// requests of trusted clients
	Trusted int64
}

type Totals struct {
	Requests int64
	Blocked  int64
	Limited  int64
	Trusted  int64
	// distinct clients with an ip
	Clients int64
}

// TrustedSum is the traffic of one trusted client, by name.
type TrustedSum struct {
	Name     string
	Requests int64
	Points   []*Point
}

type Overview struct {
	Step    time.Duration
	Points  []*Point
	Totals  *Totals
	Trusted []*TrustedSum
}

type Series struct {
	Step   time.Duration
	Points []*Point
}

// Item is a client over a range, as Top lists it.
type Item struct {
	Ip       string
	Trusted  string
	Requests int64
	// CORS preflights among Requests
	Preflights int64
	Blocked    int64
	Limited    int64
	// rule of the latest rejection in the range
	LastRule    string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	// requests of the busiest bucket of the range, preflights left out: they
	// are what rate limits count
	PeakRequests int64
	// requests of the bucket TopReq.Recent, preflights left out
	RecentRequests int64
}

type TopReq struct {
	Since time.Time
	Kind  string
	Limit int64
	// the bucket whose requests go to Item.RecentRequests
	Recent time.Time
}

// TrustedPoint is the sum of one stored bucket of one trusted client.
type TrustedPoint struct {
	Name string
	Point
}
