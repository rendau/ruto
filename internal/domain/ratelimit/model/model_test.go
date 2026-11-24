package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeExtendAddsChildRules(t *testing.T) {
	parent := RateLimit{Rules: []*Rule{{Name: "gw", Key: "ip", Requests: 100, Period: "1m", Scope: ScopeRoot}}}
	child := RateLimit{Mode: "extend", Rules: []*Rule{{Name: "login", Key: "ip", Requests: 5, Period: "1m", Scope: ScopeEndpoint("e1")}}}

	got := Merge(parent, child)

	require.Len(t, got.Rules, 2)
	assert.Equal(t, "gw", got.Rules[0].Name)
	assert.Equal(t, "login", got.Rules[1].Name)
}

func TestMergeReplaceDropsParentRules(t *testing.T) {
	parent := RateLimit{Rules: []*Rule{{Name: "gw", Requests: 100, Period: "1m"}}}

	got := Merge(parent, RateLimit{Mode: "replace"})

	assert.Empty(t, got.Rules)
}

func TestMergeDoesNotAlias(t *testing.T) {
	parent := RateLimit{Rules: []*Rule{{Name: "gw", Requests: 100, Period: "1m"}}}

	got := Merge(parent, RateLimit{})
	got.Rules[0].Requests = 1

	assert.Equal(t, 100, parent.Rules[0].Requests)
}

func TestSetScopeKeepsInherited(t *testing.T) {
	rl := RateLimit{Rules: []*Rule{{Name: "own"}, {Name: "inherited", Scope: ScopeRoot}}}

	rl.SetScope(ScopeApp("a1"))

	assert.Equal(t, "app:a1", rl.Rules[0].Scope)
	assert.Equal(t, ScopeRoot, rl.Rules[1].Scope)
}

func TestNormalize(t *testing.T) {
	rl := RateLimit{Mode: " Replace ", Rules: []*Rule{nil, {Name: " login ", Key: " IP ", Requests: 10, Period: " 1m ", Scope: "root"}}}

	require.NoError(t, rl.Normalize())

	assert.Equal(t, "replace", rl.Mode)
	require.Len(t, rl.Rules, 1)
	assert.Equal(t, Rule{Name: "login", Key: "ip", Requests: 10, Period: "1m", ParsedPeriod: time.Minute}, *rl.Rules[0])
}

func TestNormalizeDefaults(t *testing.T) {
	rl := RateLimit{Rules: []*Rule{{Requests: 1, Period: "1s"}}}

	require.NoError(t, rl.Normalize())

	assert.Equal(t, "extend", rl.Mode)
	assert.Equal(t, "ip", rl.Rules[0].Key)
}

func TestNormalizeErrors(t *testing.T) {
	cases := map[string]RateLimit{
		"bad mode":       {Mode: "off"},
		"bad key":        {Rules: []*Rule{{Key: "user", Requests: 1, Period: "1m"}}},
		"zero requests":  {Rules: []*Rule{{Requests: 0, Period: "1m"}}},
		"bad period":     {Rules: []*Rule{{Requests: 1, Period: "minute"}}},
		"period too low": {Rules: []*Rule{{Requests: 1, Period: "100ms"}}},
		"period too big": {Rules: []*Rule{{Requests: 1, Period: "25h"}}},
		"fractional":     {Rules: []*Rule{{Requests: 1, Period: "1500ms"}}},
	}
	for name, rl := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, rl.Normalize())
		})
	}
}
