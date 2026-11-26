package core_client

import (
	"context"

	"github.com/rendau/ruto/internal/constant"
)

// coreToken sends the gateways' shared token with every call to core. It goes
// over the plain-text connection to core, which stays inside the cluster.
type coreToken struct {
	value string
}

func (t *coreToken) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{constant.GatewayTokenMetadataKey: t.value}, nil
}

func (t *coreToken) RequireTransportSecurity() bool {
	return false
}
