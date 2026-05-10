package dto

import (
	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	domModel "github.com/rendau/ruto/internal/domain/app/model"
)

func EncodeAppMain(v *domModel.Main, _ int) *ruto_v1.AppMain {
	if v == nil {
		return nil
	}

	return &ruto_v1.AppMain{
		Id:         v.Id,
		Active:     v.Active,
		PathPrefix: v.PathPrefix,
		Name:       v.Name,
		Backend:    EncodeAppBackend(v.Backend),
	}
}

func DecodeAppListReq(v *ruto_v1.AppListReq) *domModel.ListReq {
	if v == nil {
		return nil
	}
	return &domModel.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		Active:     v.Active,
	}
}

func DecodeAppCreateReq(v *ruto_v1.AppCreateReq) *domModel.Edit {
	if v == nil {
		return nil
	}

	return &domModel.Edit{
		Active:     v.Active,
		PathPrefix: v.PathPrefix,
		Name:       v.Name,
		Backend:    DecodeAppBackend(v.Backend),
	}
}

func DecodeAppUpdateReq(v *ruto_v1.AppUpdateReq) *domModel.Edit {
	if v == nil {
		return nil
	}

	return &domModel.Edit{
		Active:     v.Active,
		PathPrefix: v.PathPrefix,
		Name:       v.Name,
		Backend:    DecodeAppBackend(v.Backend),
	}
}

// AppBackend

func EncodeAppBackend(x *domModel.Backend) *ruto_v1.AppBackend {
	if x == nil {
		return nil
	}
	return &ruto_v1.AppBackend{
		Url: x.Url,
	}
}

func DecodeAppBackend(x *ruto_v1.AppBackend) *domModel.Backend {
	if x == nil {
		return nil
	}
	return &domModel.Backend{
		Url: x.Url,
	}
}
