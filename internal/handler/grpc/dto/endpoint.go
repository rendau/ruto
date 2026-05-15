package dto

import (
	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	"github.com/rendau/ruto/internal/domain/endpoint/model"
)

func EncodeEndpointMain(v *model.Endpoint, _ int) *ruto_v1.EndpointMain {
	return &ruto_v1.EndpointMain{
		Id:      v.Id,
		AppId:   v.AppId,
		Active:  v.Active,
		Method:  v.Method,
		Path:    v.Path,
		Backend: EncodeEndpointBackend(v.Backend),
		Auth:    EncodeEndpointAuth(v.Auth),
	}
}

func DecodeEndpointMain(v *ruto_v1.EndpointMain) *model.Endpoint {
	return &model.Endpoint{
		Id:      v.Id,
		AppId:   v.AppId,
		Active:  v.Active,
		Method:  v.Method,
		Path:    v.Path,
		Backend: DecodeEndpointBackend(v.Backend),
		Auth:    DecodeEndpointAuth(v.Auth),
	}
}

func DecodeEndpointListReq(v *ruto_v1.EndpointListReq) *model.ListReq {
	return &model.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		AppId:      v.AppId,
		Active:     v.Active,
	}
}

func EncodeEndpointBackend(x model.Backend) *ruto_v1.EndpointBackend {
	return &ruto_v1.EndpointBackend{
		CustomPath: x.CustomPath,
	}
}

func DecodeEndpointBackend(x *ruto_v1.EndpointBackend) model.Backend {
	if x == nil {
		return model.Backend{}
	}
	return model.Backend{
		CustomPath: x.CustomPath,
	}
}
