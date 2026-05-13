package api_key

import (
	"crypto/subtle"
	"net/http"
	"strings"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

const defaultHeader = "X-API-Key"

type Authorizer struct {
	conf *endpointModel.AuthMethodAPIKey
}

func New(conf *endpointModel.AuthMethodAPIKey) *Authorizer {
	return &Authorizer{conf: conf}
}

func (a Authorizer) Authorize(r *http.Request) bool {
	if len(a.conf.Keys) == 0 {
		return false
	}

	headerName := a.conf.Header
	if headerName == "" {
		headerName = defaultHeader
	}

	clientKey := strings.TrimSpace(r.Header.Get(headerName))
	if clientKey == "" {
		return false
	}

	for _, key := range a.conf.Keys {
		if subtle.ConstantTimeCompare([]byte(clientKey), []byte(key)) == 1 {
			return true
		}
	}

	return false
}
