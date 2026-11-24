package dto

import (
	"time"

	"github.com/rendau/ruto/internal/domain/blockedclient/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeBlockedClientMain(v *model.BlockedClient, _ int) *ruto_v1.BlockedClientMain {
	return &ruto_v1.BlockedClientMain{
		Id:            v.Id,
		Ip:            v.Ip,
		Reason:        v.Reason,
		CreatedBy:     v.CreatedBy,
		Source:        v.Source,
		CreatedAtUnix: v.CreatedAt.Unix(),
		UpdatedAtUnix: v.UpdatedAt.Unix(),
		ExpiresAtUnix: unixOrZero(v.ExpiresAt),
		RemovedAtUnix: unixOrZero(v.RemovedAt),
		RemovedBy:     v.RemovedBy,
		RemoveReason:  v.RemoveReason,
		Active:        v.IsActive(time.Now()),
		ApiKeyId:      v.ApiKeyId,
	}
}

func EncodeBlockedClientRule(v *model.Rule, _ int) *ruto_v1.BlockedClientRule {
	return &ruto_v1.BlockedClientRule{
		Ip:            v.Ip,
		ExpiresAtUnix: unixOrZero(v.ExpiresAt),
	}
}
