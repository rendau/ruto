package dto

import (
	"github.com/rendau/ruto/internal/domain/usr/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeUsrMain(v *model.Usr, _ int) *ruto_v1.UsrMain {
	return &ruto_v1.UsrMain{
		Id:       v.Id,
		Active:   v.Active,
		IsAdmin:  v.IsAdmin,
		Name:     v.Name,
		Username: v.Username,
		Password: v.Password,
	}
}

func DecodeUsrMain(v *ruto_v1.UsrMain) *model.Usr {
	return &model.Usr{
		Id:       v.Id,
		Active:   v.Active,
		IsAdmin:  v.IsAdmin,
		Name:     v.Name,
		Username: v.Username,
		Password: v.Password,
	}
}

func DecodeUsrListReq(v *ruto_v1.UsrListReq) *model.ListReq {
	return &model.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		Username:   v.Username,
	}
}
