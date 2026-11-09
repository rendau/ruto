package model

import "time"

// SeenOrigin is a browser origin (scheme://host[:port]) that called the
// gateway, with how often and when. It is a candidate for CORS allow_origins.
type SeenOrigin struct {
	Origin      string
	Hits        int64
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}
