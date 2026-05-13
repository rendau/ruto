package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

const defaultAPIKeyHeader = "X-API-Key"

type apiKeyAuthorizer struct {
	conf *endpointModel.AuthMethodAPIKey
}

func newAPIKeyAuthorizer(conf *endpointModel.AuthMethodAPIKey) authorizerI {
	return apiKeyAuthorizer{conf: conf}
}

func (a apiKeyAuthorizer) Authorize(r *http.Request) bool {
	return authorizeAPIKey(r, a.conf)
}

func authorizeAPIKey(r *http.Request, conf *endpointModel.AuthMethodAPIKey) bool {
	if len(conf.Keys) == 0 {
		return false
	}

	headerName := conf.Header
	if headerName == "" {
		headerName = defaultAPIKeyHeader
	}

	clientKey := strings.TrimSpace(r.Header.Get(headerName))
	if clientKey == "" {
		return false
	}

	for _, key := range conf.Keys {
		if subtle.ConstantTimeCompare([]byte(clientKey), []byte(key)) == 1 {
			return true
		}
	}

	return false
}
