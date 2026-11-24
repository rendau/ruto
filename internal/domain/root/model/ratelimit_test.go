package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rateLimitModel "github.com/rendau/ruto/internal/domain/ratelimit/model"
)

func rule(name string) *rateLimitModel.Rule {
	return &rateLimitModel.Rule{Name: name, Requests: 10, Period: "1m"}
}

func TestRateLimitInheritDownScopes(t *testing.T) {
	extendEp := &endpointModel.Endpoint{Id: "e1", RateLimit: rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{rule("ep")}}}
	replaceEp := &endpointModel.Endpoint{Id: "e2", RateLimit: rateLimitModel.RateLimit{Mode: "replace"}}
	root := &Root{
		RateLimit: rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{rule("gw")}},
		Apps: []*appModel.App{{
			Id:        "a1",
			RateLimit: rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{rule("app")}},
			Endpoints: []*endpointModel.Endpoint{extendEp, replaceEp},
		}},
	}

	root.InheritDown()

	got := make([]string, 0)
	for _, r := range extendEp.RateLimit.Rules {
		got = append(got, r.Name+"@"+r.Scope)
	}
	assert.Equal(t, []string{"gw@root", "app@app:a1", "ep@endpoint:e1"}, got)
	assert.Empty(t, replaceEp.RateLimit.Rules)

	// the root keeps its own rules unchanged for the next app
	require.Len(t, root.RateLimit.Rules, 1)
	assert.Equal(t, "root", root.RateLimit.Rules[0].Scope)
}

func TestRateLimitNormalizeClearsScope(t *testing.T) {
	r := rule("gw")
	r.Scope = "app:x"
	root := NewEmpty()
	root.RateLimit.Rules = []*rateLimitModel.Rule{r}

	require.NoError(t, root.Normalize())

	assert.Empty(t, root.RateLimit.Rules[0].Scope)
	assert.Equal(t, "extend", root.RateLimit.Mode)
}
