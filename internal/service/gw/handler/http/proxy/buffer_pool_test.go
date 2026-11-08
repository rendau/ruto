package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBufferPool_HandsOutFullSizeBuffers(t *testing.T) {
	pool := &bufferPool{}
	pool.pool.New = buffers.pool.New

	first := pool.Get()
	require.Len(t, first, copyBufferSize)

	pool.Put(first[:10])
	second := pool.Get()
	assert.Len(t, second, copyBufferSize, "a buffer comes back at full length however much of it was used")

	pool.Put(make([]byte, 16))
	third := pool.Get()
	assert.Len(t, third, copyBufferSize, "a short foreign buffer is not kept")
}
