package seen

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePath(t *testing.T) {
	tests := map[string]string{
		"":                          "",
		"/":                         "",
		"/api/v2/product/12345":     "api/v2/product/{id}",
		"api/v2/orders/7/items/15/": "api/v2/orders/{id}/items/{id}",
		"users/550e8400-e29b-41d4-a716-446655440000":                                    "users/{id}",
		"files/5d41402abc4b2a76b9719d911017c592":                                        "files/{id}",
		"product/operacionnaya-sistema-microsoft-windows-11-home":                       "product/{id}",
		"users/550E8400-E29B-41D4-A716-446655440000":                                    "users/{id}",
		"users/550e8400e29b41d4a716446655440000":                                        "users/{id}",
		"users/{550e8400-e29b-41d4-a716-446655440000}":                                  "users/{id}",
		"users/550e8400-e29b-41d4-a716-446655440000/orders":                             "users/{id}/orders",
		"docs/act-550e8400-e29b-41d4-a716-446655440000.pdf":                             "docs/{id}",
		"a/550e8400-e29b-41d4-a716-446655440000/b/7c9e6679-7425-40de-944b-e07fc1f90ae7": "a/{id}/b/{id}",
		"api/v3/feed":                 "api/v3/feed",
		"api/v3/decade":               "api/v3/decade",
		"docs/a/b.json":               "docs/a/b.json",
		"a/b/c/d/e/f/g/h/i/j/k/l/m/n": "a/b/c/d/e/f/g/h/i/j/k/l/...",
	}

	for in, want := range tests {
		require.Equal(t, want, NormalizePath(in), in)
	}
}

func TestService_RecordAndDrain(t *testing.T) {
	s := New()

	s.Record("app-1", "ep-1", "GET", "/product/1", false)
	s.Record("app-1", "ep-1", "GET", "/product/2", true)
	s.Record("app-1", "ep-1", "POST", "/product/2", false)
	s.Record("app-1", "ep-2", "GET", "/product/2", false)

	items := s.Drain()
	require.Len(t, items, 3)

	byKey := make(map[string]*Item)
	for _, item := range items {
		byKey[item.EndpointId+" "+item.Method+" "+item.Path] = item
	}

	got := byKey["ep-1 GET product/{id}"]
	require.NotNil(t, got)
	require.Equal(t, "app-1", got.AppId)
	require.EqualValues(t, 2, got.Hits)
	require.EqualValues(t, 1, got.HitsNotFound)
	require.Equal(t, "/product/2", got.Sample)
	require.False(t, got.LastSeenAt.IsZero())

	require.Empty(t, s.Drain())

	s.Restore(items)
	s.Record("app-1", "ep-1", "GET", "/product/3", false)
	items = s.Drain()
	require.Len(t, items, 3)
}

func TestService_CapsDistinctPathsPerEndpoint(t *testing.T) {
	s := New()

	for i := range MaxPathsPerEndpoint + 50 {
		s.Record("app-1", "ep-1", "GET", "/scan/x"+strconv.Itoa(i)+"y", false)
	}
	s.Record("app-1", "ep-2", "GET", "/fine", false)

	items := s.Drain()
	require.Len(t, items, MaxPathsPerEndpoint+2)

	var other *Item
	for _, item := range items {
		if item.Path == OtherPath {
			other = item
		}
	}
	require.NotNil(t, other)
	require.EqualValues(t, 50, other.Hits)
}
