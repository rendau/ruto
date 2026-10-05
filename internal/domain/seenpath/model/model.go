package model

import "time"

// SeenPath is a path template (ids collapsed to {id}) that a wildcard endpoint
// actually served, with how often and when.
type SeenPath struct {
	AppId        string
	EndpointId   string
	Method       string
	Path         string
	Hits         int64
	HitsNotFound int64
	FirstSeenAt  time.Time
	LastSeenAt   time.Time
	Sample       string
}
