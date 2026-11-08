package dto

import (
	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeGatewaySettings(m *snapshotModel.GatewaySettings) *ruto_v1.GatewaySettings {
	return &ruto_v1.GatewaySettings{
		RedisAddr:     m.RedisAddr,
		RedisPassword: m.RedisPassword,
		RedisDb:       int32(m.RedisDB),
	}
}
