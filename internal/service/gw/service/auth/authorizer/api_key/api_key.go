package api_key

import (
	"strings"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

type Authorizer struct {
	header        string
	allowedKeyMap map[string]bool
}

const (
	bearerPrefix = "bearer "
)

func New(conf *authModel.AuthMethodAPIKey) *Authorizer {
	header := strings.ToLower(strings.TrimSpace(conf.Header))
	if header == "" {
		header = constant.AuthAPIKeyDefaultHeader
	}

	return &Authorizer{
		header:        header,
		allowedKeyMap: lo.SliceToMap(conf.Keys, func(item authModel.AuthMethodAPIKeyItem) (string, bool) { return item.Key, true }),
	}
}

func (a *Authorizer) Authorize(req *model.AuthRequest) model.Result {
	apiKey := req.ExtractAPIKey(a.header)
	if apiKey == "" {
		return model.NoCredentials
	}

	if a.allowedKeyMap[apiKey] || a.allowedKeyMap[stripBearer(apiKey)] {
		return model.Allowed
	}

	return model.InvalidCredentials
}

func stripBearer(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= len(bearerPrefix) && strings.EqualFold(trimmed[:len(bearerPrefix)], bearerPrefix) {
		return strings.TrimSpace(trimmed[len(bearerPrefix):])
	}

	return value
}
