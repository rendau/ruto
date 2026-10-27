package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeChildWins(t *testing.T) {
	parent := Default()
	child := Compression{Response: "off", Algorithms: []string{"gzip"}, MinSize: 10}

	got := Merge(parent, child)

	assert.Equal(t, "identity", got.Backend)
	assert.Equal(t, "off", got.Response)
	assert.Equal(t, []string{"gzip"}, got.Algorithms)
	assert.Equal(t, 10, got.MinSize)
	assert.Equal(t, parent.ContentTypes, got.ContentTypes)
}

func TestMergeEmptyChildInheritsAll(t *testing.T) {
	parent := Compression{Backend: "client", Response: "on", Algorithms: []string{"zstd", "gzip"}, MinSize: 2048, ContentTypes: []string{"text/*"}}

	got := Merge(parent, Compression{})

	assert.Equal(t, parent, got)
}

func TestMergeDoesNotAliasParentSlices(t *testing.T) {
	parent := Default()

	got := Merge(parent, Compression{})
	got.Algorithms[0] = "gzip"

	assert.Equal(t, "br", parent.Algorithms[0])
}

func TestNormalize(t *testing.T) {
	m := Compression{
		Backend:      " Identity ",
		Response:     "ON",
		Algorithms:   []string{"BR", "", "gzip", "br"},
		ContentTypes: []string{"Application/JSON", " text/* ", "application/json"},
	}

	require.NoError(t, m.Normalize())

	assert.Equal(t, "identity", m.Backend)
	assert.Equal(t, "on", m.Response)
	assert.Equal(t, []string{"br", "gzip"}, m.Algorithms)
	assert.Equal(t, []string{"application/json", "text/*"}, m.ContentTypes)
}

func TestNormalizeRejects(t *testing.T) {
	tests := []struct {
		name string
		in   Compression
	}{
		{name: "backend", in: Compression{Backend: "none"}},
		{name: "response", in: Compression{Response: "yes"}},
		{name: "algorithm", in: Compression{Algorithms: []string{"deflate"}}},
		{name: "min size", in: Compression{MinSize: -1}},
		{name: "content type", in: Compression{ContentTypes: []string{"json"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.in.Normalize())
		})
	}
}

func TestMatchesContentType(t *testing.T) {
	m := Compression{ContentTypes: []string{"text/*", "application/json", "image/svg+xml"}}

	tests := []struct {
		contentType string
		want        bool
	}{
		{"application/json", true},
		{"Application/JSON; charset=utf-8", true},
		{"application/problem+json", true},
		{"text/html; charset=utf-8", true},
		{"text/event-stream", true},
		{"image/svg+xml", true},
		{"image/png", false},
		{"application/octet-stream", false},
		{"application/xml", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			assert.Equal(t, tt.want, m.MatchesContentType(tt.contentType))
		})
	}
}

func TestMatchesContentTypeDefaults(t *testing.T) {
	m := Compression{}

	assert.True(t, m.MatchesContentType("application/ld+json"))
	assert.True(t, m.MatchesContentType("text/plain"))
	assert.False(t, m.MatchesContentType("application/pdf"))
}
