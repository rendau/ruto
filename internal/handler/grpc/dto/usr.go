package dto

import (
	"github.com/rendau/ruto/internal/domain/usr/model"
	usecase "github.com/rendau/ruto/internal/usecase/usr"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeUsrMain(v *model.Usr, _ int) *ruto_v1.UsrMain {
	return &ruto_v1.UsrMain{
		Id:       v.Id,
		Active:   v.Active,
		IsAdmin:  v.IsAdmin,
		AllApps:  v.AllApps,
		AppIds:   v.AppIds,
		Name:     v.Name,
		Username: v.Username,
		Password: v.Password,
	}
}

func DecodeUsrCreate(v *ruto_v1.UsrCreate) *model.Edit {
	if v == nil {
		return &model.Edit{}
	}
	return &model.Edit{
		Active:       v.Active,
		IsAdmin:      v.IsAdmin,
		AllApps:      v.AllApps,
		UpdateAppIds: true,
		AppIds:       v.AppIds,
		Name:         v.Name,
		Username:     v.Username,
		Password:     v.Password,
	}
}

func DecodeUsrEdit(v *ruto_v1.UsrEdit) *model.Edit {
	if v == nil {
		return &model.Edit{}
	}
	return &model.Edit{
		Active:       v.Active,
		IsAdmin:      v.IsAdmin,
		AllApps:      v.AllApps,
		UpdateAppIds: v.UpdateAppIds,
		AppIds:       v.AppIds,
		Name:         v.Name,
		Username:     v.Username,
		Password:     v.Password,
	}
}

func DecodeUsrListReq(v *ruto_v1.UsrListReq) *model.ListReq {
	return &model.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		Search:     v.Search,
	}
}

func DecodeUsrUpdateProfileReq(v *ruto_v1.UsrUpdateProfileReq) *usecase.UpdateProfileReq {
	return &usecase.UpdateProfileReq{
		Name:     v.Name,
		Password: v.Password,
	}
}
