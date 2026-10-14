package dto

import (
	"time"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/domain/changerequest/model"
	usecase "github.com/rendau/ruto/internal/usecase/changerequest"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func DecodeChangeRequestListReq(v *ruto_v1.ChangeRequestListReq) *model.ListReq {
	return &model.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		Status:     v.Status,
	}
}

func EncodeChangeRequestMain(v *model.ChangeRequest, _ int) *ruto_v1.ChangeRequestMain {
	return &ruto_v1.ChangeRequestMain{
		Id:              v.Id,
		Status:          v.Status,
		Title:           v.Title,
		Description:     v.Description,
		Publish:         v.Publish,
		OpsCount:        int64(len(v.Plan.Ops)),
		CreatedAtUnix:   v.CreatedAt.Unix(),
		ApiKeyName:      v.ApiKeyName,
		DecidedAtUnix:   unixOrZero(v.DecidedAt),
		Comment:         v.Comment,
		Error:           v.Error,
		PublishedAtUnix: unixOrZero(v.PublishedAt),
		PublishError:    v.PublishError,
	}
}

func EncodeChangeRequestDetail(v *usecase.Detail) *ruto_v1.ChangeRequestDetail {
	return &ruto_v1.ChangeRequestDetail{
		Main:           EncodeChangeRequestMain(v.ChangeRequest, 0),
		AuthorName:     v.AuthorName,
		DecidedName:    v.DecidedName,
		Outdated:       v.Outdated,
		OutdatedReason: v.OutdatedReason,
		Ops:            lo.Map(v.Ops, encodeChangeRequestOp),
	}
}

func encodeChangeRequestOp(v *usecase.OpView, _ int) *ruto_v1.ChangeRequestOp {
	return &ruto_v1.ChangeRequestOp{
		Op:         v.Op,
		TargetKind: v.TargetKind,
		TargetId:   v.TargetId,
		Title:      v.Title,
		AppName:    v.AppName,
		Changes:    lo.Map(v.Changes, encodeChangeRequestChange),
		Notes:      v.Notes,
	}
}

func encodeChangeRequestChange(v *usecase.Change, _ int) *ruto_v1.ChangeRequestChange {
	return &ruto_v1.ChangeRequestChange{
		Path:   v.Path,
		Before: v.Before,
		After:  v.After,
		Secret: v.Secret,
	}
}

func unixOrZero(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.Unix()
}
