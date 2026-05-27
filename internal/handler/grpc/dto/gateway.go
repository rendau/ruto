package dto

import (
	"github.com/samber/lo"

	usecase "github.com/rendau/ruto/internal/usecase/gateway"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func DecodeGatewayHeartbeatReq(v *ruto_v1.GatewayHeartbeatRequest) *usecase.Heartbeat {
	if v == nil {
		return nil
	}

	return &usecase.Heartbeat{
		GatewayID:       v.GatewayId,
		PodName:         v.PodName,
		HostName:        v.HostName,
		SnapshotVersion: v.SnapshotVersion,
		LastApplyAtUnix: v.LastApplyAtUnix,
		StartedAtUnix:   v.StartedAtUnix,
		LastError:       v.LastError,
	}
}

func EncodeGatewayListResponse(items []*usecase.Item) *ruto_v1.GatewayListResponse {
	return &ruto_v1.GatewayListResponse{
		Results: lo.Map(items, EncodeGatewayStateItem),
	}
}

func EncodeGatewayStateItem(item *usecase.Item, _ int) *ruto_v1.GatewayStateItem {
	if item == nil {
		return &ruto_v1.GatewayStateItem{}
	}

	return &ruto_v1.GatewayStateItem{
		GatewayId:       item.GatewayID,
		PodName:         item.PodName,
		HostName:        item.HostName,
		SnapshotVersion: item.SnapshotVersion,
		LastApplyAtUnix: item.LastApplyAtUnix,
		StartedAtUnix:   item.StartedAtUnix,
		LastError:       item.LastError,
		LastSeenAtUnix:  item.LastSeenAtUnix,
		Status:          item.Status,
	}
}
