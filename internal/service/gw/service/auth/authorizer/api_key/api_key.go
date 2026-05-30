package api_key

import (
	"strings"

	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

type Authorizer struct {
	header        string
	allowedKeyMap map[string]bool
}

const defaultAPIKeyHeader = "x-api-key"

func New(conf *authModel.AuthMethodAPIKey) *Authorizer {
	header := strings.ToLower(strings.TrimSpace(conf.Header))
	if header == "" {
		header = defaultAPIKeyHeader
	}

	return &Authorizer{
		header:        header,
		allowedKeyMap: lo.SliceToMap(conf.Keys, func(key string) (string, bool) { return key, true }),
	}
}

func (a *Authorizer) Authorize(req *model.AuthRequest) bool {
	if len(a.allowedKeyMap) == 0 {
		return false
	}

	apiKey := req.ExtractAPIKey(a.header)
	if apiKey == "" {
		return false
	}

	return a.allowedKeyMap[apiKey]
}
