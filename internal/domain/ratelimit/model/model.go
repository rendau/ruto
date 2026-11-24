package model

import (
	"time"
)

// RateLimit caps how many requests a client may send. It is defined on Root,
// App and Endpoint and flows down via Merge (see merge.go): with mode "extend"
// (the default) a level adds its rules to the inherited ones, with "replace"
// only its own rules apply — "replace" with no rules turns limits off from that
// level down.
//
// Every rule counts on the level it is written on: a root rule counts all the
// requests of a client through the whole gateway, an app rule — through the
// app, an endpoint rule — to that endpoint. A request passes only when every
// rule that reaches its endpoint allows it.
type RateLimit struct {
	Mode  string  `json:"mode"` // "" (= extend) | "extend" | "replace"
	Rules []*Rule `json:"rules"`
}

type Rule struct {
	// Name is a free-form label shown in the admin and in logs and metrics.
	Name string `json:"name"`
	// Key tells whose requests a counter adds up: "ip" — of one client
	// address, "token" — of one credential from the Authorization header (the
	// bearer token, or the user of basic auth; none — by ip), "all" — every
	// request together. A made-up bearer token gets a counter of its own, so a
	// "token" rule is paired with an "ip" one.
	Key string `json:"key"` // "ip" | "token" | "all"
	// Requests is how many requests a key may send per Period.
	Requests int `json:"requests"`
	// Period is a Go duration in whole seconds, from 1s to 24h.
	Period string `json:"period"`

	// Scope is where the rule is written and so where its counter lives:
	// "root", "app:<app id>" or "endpoint:<endpoint id>". Set by InheritDown,
	// never stored: Normalize clears it.
	Scope string `json:"scope,omitempty"`

	ParsedPeriod time.Duration `json:"-"`
}

const ScopeRoot = "root"

func ScopeApp(id string) string { return "app:" + id }

func ScopeEndpoint(id string) string { return "endpoint:" + id }

func (m *RateLimit) CloneRules() []*Rule {
	result := make([]*Rule, len(m.Rules))
	for i, rule := range m.Rules {
		result[i] = new(*rule)
	}
	return result
}

// SetScope stamps the level's own rules before they are merged down; a rule
// already stamped came from above and keeps its scope.
func (m *RateLimit) SetScope(scope string) {
	for _, rule := range m.Rules {
		if rule.Scope == "" {
			rule.Scope = scope
		}
	}
}
