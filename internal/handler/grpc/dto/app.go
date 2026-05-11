package dto

import (
	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	"github.com/rendau/ruto/internal/domain/app/model"
)

func EncodeAppMain(v *model.App, _ int) *ruto_v1.AppMain {
	return &ruto_v1.AppMain{
		Id:         v.Id,
		Active:     v.Active,
		PathPrefix: v.PathPrefix,
		Name:       v.Name,
		Backend:    EncodeAppBackend(v.Backend),
	}
}

func DecodeAppListReq(v *ruto_v1.AppListReq) *model.ListReq {
	return &model.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		Active:     v.Active,
	}
}

func DecodeAppMain(v *ruto_v1.AppMain) *model.App {
	return &model.App{
		Id:         v.Id,
		Active:     v.Active,
		PathPrefix: v.PathPrefix,
		Name:       v.Name,
		Backend:    DecodeAppBackend(v.Backend),
	}
}

// AppBackend

func EncodeAppBackend(x model.AppBackend) *ruto_v1.AppBackend {
	return &ruto_v1.AppBackend{
		Url: x.Url,
	}
}

func DecodeAppBackend(x *ruto_v1.AppBackend) model.AppBackend {
	if x == nil {
		return model.AppBackend{}
	}
	return model.AppBackend{
		Url: x.Url,
	}
}
