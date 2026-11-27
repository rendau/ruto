package model

import "time"

// WindowDays is how many days, today included, the hits of a listed origin
// cover. Older days are pruned.
const WindowDays = 7

// SeenOrigin is a browser origin (scheme://host[:port]) that called the
// gateway, with how often and when. It is a candidate for CORS allow_origins.
type SeenOrigin struct {
	Origin string
	// Hits: in a gateway report, the requests since its previous report; in
	// the list, the requests over the last WindowDays days.
	Hits        int64
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}

// WindowStart is the first day (midnight in now's location) of the window
// that ends on the day of now.
func WindowStart(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d-(WindowDays-1), 0, 0, 0, 0, now.Location())
}
