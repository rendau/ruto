package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/connectivity"
)

func TestConns_RetainClosesTheBackendsThatLeft(t *testing.T) {
	conns := NewConns()

	first, err := conns.Get("127.0.0.1:1")
	require.NoError(t, err)
	second, err := conns.Get("127.0.0.1:2")
	require.NoError(t, err)

	again, err := conns.Get("127.0.0.1:1")
	require.NoError(t, err)
	assert.Same(t, first, again, "one connection per address")

	conns.Retain(map[string]struct{}{"127.0.0.1:1": {}})
	assert.Equal(t, connectivity.Shutdown, second.GetState(), "the backend that left is closed")
	assert.NotEqual(t, connectivity.Shutdown, first.GetState(), "the backend that stayed is kept")

	kept, err := conns.Get("127.0.0.1:1")
	require.NoError(t, err)
	assert.Same(t, first, kept)

	conns.Close()
	assert.Equal(t, connectivity.Shutdown, first.GetState())
	assert.Empty(t, conns.items)
}
