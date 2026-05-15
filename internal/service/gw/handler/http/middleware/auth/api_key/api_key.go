package api_key

import (
	"net/http"
	"strings"

	"github.com/samber/lo"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

const defaultHeader = "X-API-Key"

type Authorizer struct {
	headerName    string
	allowedKeyMap map[string]bool
}

func New(conf *endpointModel.AuthMethodAPIKey) *Authorizer {
	headerName := strings.TrimSpace(conf.Header)
	if headerName == "" {
		headerName = defaultHeader
	}

	return &Authorizer{
		headerName: headerName,
		allowedKeyMap: lo.SliceToMap(
			conf.Keys,
			func(key string) (string, bool) {
				return key, true
			},
		),
	}
}

func (a Authorizer) Authorize(r *http.Request) bool {
	if len(a.allowedKeyMap) == 0 {
		return false
	}

	clientKey := strings.TrimSpace(r.Header.Get(a.headerName))
	if clientKey == "" {
		return false
	}

	return a.checkKey(clientKey)
}

func (a Authorizer) checkKey(key string) bool {
	return a.allowedKeyMap[key]
}
