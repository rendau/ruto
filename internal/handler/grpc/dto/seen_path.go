package dto

import (
	"time"

	"github.com/rendau/ruto/internal/domain/seenpath/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeSeenPathMain(v *model.SeenPath, _ int) *ruto_v1.SeenPathMain {
	return &ruto_v1.SeenPathMain{
		AppId:           v.AppId,
		EndpointId:      v.EndpointId,
		Method:          v.Method,
		Path:            v.Path,
		Hits:            v.Hits,
		HitsNotFound:    v.HitsNotFound,
		FirstSeenAtUnix: v.FirstSeenAt.Unix(),
		LastSeenAtUnix:  v.LastSeenAt.Unix(),
		Sample:          v.Sample,
	}
}

func DecodeGatewaySeenPath(v *ruto_v1.GatewaySeenPath, _ int) *model.SeenPath {
	return &model.SeenPath{
		AppId:        v.AppId,
		EndpointId:   v.EndpointId,
		Method:       v.Method,
		Path:         v.Path,
		Hits:         v.Hits,
		HitsNotFound: v.HitsNotFound,
		LastSeenAt:   time.Unix(v.LastSeenAtUnix, 0),
		Sample:       v.Sample,
	}
}
