package model

import "time"

type ListReq struct {
	EndpointId string
	Since      time.Time
	Until      time.Time
	OnlyErrors bool
	Limit      int
}

type Entry struct {
	Ts       time.Time
	Status   string
	Duration string
	Error    string
	Message  string
	Raw      string
}

// NewEntryFromFields builds an Entry from a parsed access-log record (slog
// JSON fields), shared by all log-store providers.
func NewEntryFromFields(ts time.Time, fields map[string]any, raw string) *Entry {
	str := func(key string) string {
		v, _ := fields[key].(string)
		return v
	}

	return &Entry{
		Ts:       ts,
		Status:   str("status"),
		Duration: str("duration"),
		Error:    str("error"),
		Message:  str("msg"),
		Raw:      raw,
	}
}
