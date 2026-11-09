package dto

import (
	"time"

	"github.com/rendau/ruto/internal/domain/seenorigin/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeSeenOriginMain(v *model.SeenOrigin, _ int) *ruto_v1.SeenOriginMain {
	return &ruto_v1.SeenOriginMain{
		Origin:          v.Origin,
		Hits:            v.Hits,
		FirstSeenAtUnix: v.FirstSeenAt.Unix(),
		LastSeenAtUnix:  v.LastSeenAt.Unix(),
	}
}

func DecodeGatewaySeenOrigin(v *ruto_v1.GatewaySeenOrigin, _ int) *model.SeenOrigin {
	return &model.SeenOrigin{
		Origin:     v.Origin,
		Hits:       v.Hits,
		LastSeenAt: time.Unix(v.LastSeenAtUnix, 0),
	}
}
