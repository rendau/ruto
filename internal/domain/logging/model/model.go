package model

import "github.com/rendau/ruto/internal/constant"

// Logging holds request-logging configuration. It is defined on Root, App and
// Endpoint and flows down via Merge (see merge.go) honoring extend/replace.
//
// Level controls when a request is logged: "all" logs every request, "error"
// logs only failed ones. Empty level means "inherit" during merge and falls
// back to "error" at request time (see EffectiveLevel).
//
// The boolean flags select what gets logged in addition to the always-present
// method and path. Body limits cap how many bytes of the request/response body
// end up in the log; anything beyond the limit is truncated.
type Logging struct {
	Mode          string `json:"mode"`  // "extend" | "replace"
	Level         string `json:"level"` // "" | "all" | "error"
	Headers       bool   `json:"headers"`
	QueryParams   bool   `json:"query_params"`
	ReqBody       bool   `json:"req_body"`
	RespBody      bool   `json:"resp_body"`
	ReqBodyLimit  int    `json:"req_body_limit"`  // bytes, 0 => DefaultBodyLogLimit
	RespBodyLimit int    `json:"resp_body_limit"` // bytes, 0 => DefaultBodyLogLimit
}

// EffectiveLevel resolves the level used at request time, defaulting empty to
// "error".
func (m *Logging) EffectiveLevel() string {
	if m.Level == "" {
		return constant.LoggingLevelError
	}
	return m.Level
}

// ReqBodyLimitOrDefault returns the configured request-body limit, or the
// default when unset.
func (m *Logging) ReqBodyLimitOrDefault() int {
	if m.ReqBodyLimit <= 0 {
		return constant.DefaultBodyLogLimit
	}
	return m.ReqBodyLimit
}

// RespBodyLimitOrDefault returns the configured response-body limit, or the
// default when unset.
func (m *Logging) RespBodyLimitOrDefault() int {
	if m.RespBodyLimit <= 0 {
		return constant.DefaultBodyLogLimit
	}
	return m.RespBodyLimit
}
