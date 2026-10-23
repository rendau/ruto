package cache

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_SetGet(t *testing.T) {
	s := New(1 << 20)

	require.True(t, s.Set("k", &Entry{Status: 200, Header: http.Header{"A": {"b"}}, Body: []byte("body")}, time.Minute))

	entry := s.Get("k")
	require.NotNil(t, entry)
	assert.Equal(t, 200, entry.Status)
	assert.Equal(t, "body", string(entry.Body))
	assert.Equal(t, int64(len("k")+len("body")+len("A")+len("b")), s.Size())

	assert.Nil(t, s.Get("missing"))
}

func TestService_Expired(t *testing.T) {
	s := New(1 << 20)

	require.True(t, s.Set("k", &Entry{Body: []byte("body")}, time.Millisecond))
	time.Sleep(5 * time.Millisecond)

	assert.Nil(t, s.Get("k"))
	assert.NotZero(t, s.Size(), "an expired entry holds memory until the sweep")

	s.Sweep()
	assert.Zero(t, s.Size())
}

func TestService_MaxSize(t *testing.T) {
	s := New(10)

	require.True(t, s.Set("a", &Entry{Body: []byte("12345")}, time.Minute))
	assert.False(t, s.Set("b", &Entry{Body: []byte("12345")}, time.Minute), "no room left")
	assert.Nil(t, s.Get("b"))

	// replacing an entry frees its old size first
	require.True(t, s.Set("a", &Entry{Body: []byte("123456789")}, time.Minute))
	assert.Equal(t, int64(10), s.Size())
}
