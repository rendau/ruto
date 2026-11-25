package dto

import (
	"time"

	"github.com/samber/lo"

	blockedClientModel "github.com/rendau/ruto/internal/domain/blockedclient/model"
	"github.com/rendau/ruto/internal/domain/clientstat/model"
	usecase "github.com/rendau/ruto/internal/usecase/clientstat"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func DecodeGatewayClientStat(v *ruto_v1.GatewayClientStat, _ int) *model.ClientStat {
	return &model.ClientStat{
		Ip:       v.Ip,
		Trusted:  v.Trusted,
		Requests: v.Requests,
		Blocked:  v.Blocked,
		Limited:  v.Limited,
		LastRule: v.LastRule,
	}
}

// DecodeClientStatRange caps the seconds before converting them, so a huge
// value cannot overflow the duration into a small or negative range.
func DecodeClientStatRange(seconds int64) time.Duration {
	return time.Duration(min(seconds, int64(model.MaxRange/time.Second))) * time.Second
}

func EncodeClientStatOverviewRep(v *model.Overview) *ruto_v1.ClientStatOverviewRep {
	return &ruto_v1.ClientStatOverviewRep{
		StepSeconds: int64(v.Step.Seconds()),
		Points:      lo.Map(v.Points, EncodeClientStatPoint),
		Totals: &ruto_v1.ClientStatTotals{
			Requests: v.Totals.Requests,
			Blocked:  v.Totals.Blocked,
			Limited:  v.Totals.Limited,
			Trusted:  v.Totals.Trusted,
			Clients:  v.Totals.Clients,
		},
		Trusted: lo.Map(v.Trusted, EncodeClientStatTrusted),
	}
}

func EncodeClientStatSeriesRep(v *model.Series) *ruto_v1.ClientStatSeriesRep {
	return &ruto_v1.ClientStatSeriesRep{
		StepSeconds: int64(v.Step.Seconds()),
		Points:      lo.Map(v.Points, EncodeClientStatPoint),
	}
}

func EncodeClientStatPoint(v *model.Point, _ int) *ruto_v1.ClientStatPoint {
	return &ruto_v1.ClientStatPoint{
		Ts:       v.Ts.Unix(),
		Requests: v.Requests,
		Blocked:  v.Blocked,
		Limited:  v.Limited,
		Trusted:  v.Trusted,
	}
}

func EncodeClientStatTrusted(v *model.TrustedSum, _ int) *ruto_v1.ClientStatTrusted {
	return &ruto_v1.ClientStatTrusted{
		Name:     v.Name,
		Requests: v.Requests,
		Points:   lo.Map(v.Points, EncodeClientStatPoint),
	}
}

func EncodeClientStatItem(v *usecase.TopItem, _ int) *ruto_v1.ClientStatItem {
	return &ruto_v1.ClientStatItem{
		Ip:              v.Ip,
		Trusted:         v.Trusted,
		Requests:        v.Requests,
		Blocked:         v.Blocked,
		Limited:         v.Limited,
		LastRule:        v.LastRule,
		FirstSeenAtUnix: v.FirstSeenAt.Unix(),
		LastSeenAtUnix:  v.LastSeenAt.Unix(),
		Block:           encodeClientStatBlock(v.Block),
	}
}

func encodeClientStatBlock(v *blockedClientModel.BlockedClient) *ruto_v1.ClientStatBlock {
	if v == nil {
		return nil
	}
	return &ruto_v1.ClientStatBlock{
		Id:            v.Id,
		Ip:            v.Ip,
		Reason:        v.Reason,
		Source:        v.Source,
		CreatedBy:     v.CreatedBy,
		ExpiresAtUnix: unixOrZero(v.ExpiresAt),
	}
}
