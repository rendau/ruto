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

const (
	defaultAPIKeyHeader = "x-api-key"
	bearerPrefix        = "bearer "
)

func New(conf *authModel.AuthMethodAPIKey) *Authorizer {
	header := strings.ToLower(strings.TrimSpace(conf.Header))
	if header == "" {
		header = defaultAPIKeyHeader
	}

	return &Authorizer{
		header:        header,
		allowedKeyMap: lo.SliceToMap(conf.Keys, func(item authModel.AuthMethodAPIKeyItem) (string, bool) { return item.Key, true }),
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

	if a.allowedKeyMap[apiKey] {
		return true
	}

	if stripped := stripBearer(apiKey); stripped != apiKey {
		return a.allowedKeyMap[stripped]
	}

	return false
}

func stripBearer(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= len(bearerPrefix) && strings.EqualFold(trimmed[:len(bearerPrefix)], bearerPrefix) {
		return strings.TrimSpace(trimmed[len(bearerPrefix):])
	}

	return value
}
