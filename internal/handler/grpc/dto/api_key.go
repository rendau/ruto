package dto

import (
	"github.com/rendau/ruto/internal/domain/apikey/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeApiKeyMain(v *model.ApiKey, _ int) *ruto_v1.ApiKeyMain {
	result := &ruto_v1.ApiKeyMain{
		Id:            v.Id,
		Name:          v.Name,
		KeyPrefix:     v.KeyPrefix,
		CreatedAtUnix: v.CreatedAt.Unix(),
	}
	if v.LastUsedAt != nil {
		result.LastUsedAtUnix = v.LastUsedAt.Unix()
	}
	return result
}
