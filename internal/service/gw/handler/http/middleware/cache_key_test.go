package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

func TestCacheKeyHeaders_AcceptEncodingOnlyWhenBackendMayCompress(t *testing.T) {
	ep := &endpointModel.Endpoint{}
	ep.Cache.Server.VaryHeaders = []string{"X-Lang"}

	ep.Compression.Backend = "identity"
	assert.Equal(t, []string{"X-Lang"}, cacheKeyHeaders(ep))

	ep.Compression.Backend = "client"
	assert.Equal(t, []string{"Accept-Encoding", "X-Lang"}, cacheKeyHeaders(ep))
}
