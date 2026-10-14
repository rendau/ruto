package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	changeRequestModel "github.com/rendau/ruto/internal/domain/changerequest/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	usecaseChangeRequest "github.com/rendau/ruto/internal/usecase/changerequest"
)

func (s *sessionServer) registerChangeTools(srv *mcpsdk.Server) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "propose_changes",
		Description: `Propose configuration changes as one change request. Nothing changes until a person applies the request in the ruto admin UI; the request is checked right away (a failing check returns the error and nothing is stored).

Operations (applied in order, all or nothing):
- create_app: data = app object; ref = "$name" to use the new app in later operations.
- update_app: id, version, data = full app object (get_app, edited).
- delete_app: id, version. Deletes all its endpoints too.
- create_endpoint: app = app id or a "$ref" of create_app in this request; data = endpoint object (type "http": http.method, http.path, e.g. "users/{id}" or a trailing "*" when the app allows wildcards; backend.custom_path; transform.request / transform.response JavaScript).
- update_endpoint: id, version, data = full endpoint object (get_endpoint, edited).
- delete_endpoint: id, version.
- update_root: version, data = full root object (get_root, edited).

Secret positions take value sources, never secret values: keep or copy a {"secret_ref": ...} object, {"generate": {...}, "name": ...}, {"reuse": name} or a plain non-secret string.`,
	}, s.proposeChanges)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_change_request",
		Description: "A change request made with your key: status (pending, applied, rejected, cancelled, expired, outdated), the reviewer's comment, the error, ids of created objects, the publish result and the reviewed changes (before → after, secrets described, not shown).",
	}, s.getChangeRequest)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_change_requests",
		Description: "Change requests made with your key, newest first.",
	}, s.listChangeRequests)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "cancel_change_request",
		Description: "Withdraw a pending change request made with your key.",
	}, s.cancelChangeRequest)
}

type OpIn struct {
	Op      string         `json:"op" jsonschema:"create_app | update_app | delete_app | create_endpoint | update_endpoint | delete_endpoint | update_root"`
	Ref     string         `json:"ref,omitempty" jsonschema:"create_app: a name starting with $ to reference the new app in later operations"`
	Id      string         `json:"id,omitempty" jsonschema:"update_*/delete_*: id of the target"`
	App     string         `json:"app,omitempty" jsonschema:"create_endpoint: app id or $ref"`
	Version string         `json:"version,omitempty" jsonschema:"update_*/delete_*: version returned by get_* when you read the object"`
	Data    map[string]any `json:"data,omitempty" jsonschema:"the full object for create_*/update_*"`
}

type ProposeIn struct {
	Title       string `json:"title" jsonschema:"short summary the reviewer sees first"`
	Description string `json:"description,omitempty" jsonschema:"why the change is needed and anything the reviewer should check"`
	Publish     bool   `json:"publish,omitempty" jsonschema:"ask to publish right after the request is applied"`
	Ops         []OpIn `json:"ops"`
}

type ProposeOut struct {
	Id      int64  `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (s *sessionServer) proposeChanges(ctx context.Context, req *mcpsdk.CallToolRequest, in ProposeIn) (*mcpsdk.CallToolResult, ProposeOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ProposeOut{}, err
	}

	ops := make([]*usecaseChangeRequest.ProposeOp, 0, len(in.Ops))
	for _, op := range in.Ops {
		var data json.RawMessage
		if op.Data != nil {
			if data, err = json.Marshal(op.Data); err != nil {
				return nil, ProposeOut{}, fmt.Errorf("data: %w", err)
			}
		}
		ops = append(ops, &usecaseChangeRequest.ProposeOp{
			Op: changeRequestModel.Op{
				Op:   op.Op,
				Ref:  op.Ref,
				Id:   op.Id,
				App:  op.App,
				Data: data,
			},
			Version: op.Version,
		})
	}

	id, err := s.h.changeRequestUc.Propose(ctx, &usecaseChangeRequest.ProposeReq{
		Title:       in.Title,
		Description: in.Description,
		Publish:     in.Publish,
		Ops:         ops,
	})
	if err != nil {
		return nil, ProposeOut{}, toolErr(err)
	}

	return nil, ProposeOut{
		Id:      id,
		Status:  changeRequestModel.StatusPending,
		Message: fmt.Sprintf("change request #%d is waiting for a person to apply it in the ruto admin UI (Change requests). Nothing is changed until then.", id),
	}, nil
}

type ChangeRequestIdIn struct {
	Id int64 `json:"id" jsonschema:"change request id"`
}

type ChangeOut struct {
	Path   string `json:"path"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

type OpViewOut struct {
	Op       string      `json:"op"`
	TargetId string      `json:"target_id,omitempty"`
	Title    string      `json:"title,omitempty"`
	AppName  string      `json:"app_name,omitempty"`
	Changes  []ChangeOut `json:"changes,omitempty"`
	Notes    []string    `json:"notes,omitempty"`
}

type ChangeRequestOut struct {
	Id           int64             `json:"id"`
	Status       string            `json:"status"`
	Title        string            `json:"title"`
	Description  string            `json:"description,omitempty"`
	Publish      bool              `json:"publish"`
	CreatedAt    string            `json:"created_at"`
	DecidedAt    string            `json:"decided_at,omitempty"`
	DecidedBy    string            `json:"decided_by,omitempty"`
	Comment      string            `json:"comment,omitempty"`
	Error        string            `json:"error,omitempty"`
	Created      map[string]string `json:"created,omitempty"`
	PublishedAt  string            `json:"published_at,omitempty"`
	PublishError string            `json:"publish_error,omitempty"`
	Outdated     string            `json:"outdated,omitempty"`
	Ops          []OpViewOut       `json:"ops,omitempty"`
}

func (s *sessionServer) getChangeRequest(ctx context.Context, req *mcpsdk.CallToolRequest, in ChangeRequestIdIn) (*mcpsdk.CallToolResult, ChangeRequestOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ChangeRequestOut{}, err
	}

	item, err := s.h.changeRequestUc.Get(ctx, in.Id)
	if err != nil {
		return nil, ChangeRequestOut{}, toolErr(err)
	}

	result := changeRequestOut(item.ChangeRequest, 0)
	result.DecidedBy = item.DecidedName
	result.Outdated = item.OutdatedReason
	result.Ops = lo.Map(item.Ops, func(v *usecaseChangeRequest.OpView, _ int) OpViewOut {
		return OpViewOut{
			Op:       v.Op,
			TargetId: v.TargetId,
			Title:    v.Title,
			AppName:  v.AppName,
			Notes:    v.Notes,
			Changes: lo.Map(v.Changes, func(c *usecaseChangeRequest.Change, _ int) ChangeOut {
				return ChangeOut{Path: c.Path, Before: c.Before, After: c.After}
			}),
		}
	})

	return nil, result, nil
}

type ChangeRequestListIn struct {
	Status string `json:"status,omitempty" jsonschema:"filter: pending | applied | rejected | cancelled | expired | outdated"`
	Page   int64  `json:"page,omitempty" jsonschema:"page number from 0"`
}

type ChangeRequestListOut struct {
	TotalCount int64              `json:"total_count"`
	Requests   []ChangeRequestOut `json:"requests"`
}

func (s *sessionServer) listChangeRequests(ctx context.Context, req *mcpsdk.CallToolRequest, in ChangeRequestListIn) (*mcpsdk.CallToolResult, ChangeRequestListOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ChangeRequestListOut{}, err
	}

	pars := &changeRequestModel.ListReq{
		ListParams: commonModel.ListParams{Page: in.Page, PageSize: 50, WithTotalCount: true},
	}
	if in.Status != "" {
		pars.Status = &in.Status
	}

	items, total, err := s.h.changeRequestUc.List(ctx, pars)
	if err != nil {
		return nil, ChangeRequestListOut{}, toolErr(err)
	}

	return nil, ChangeRequestListOut{TotalCount: total, Requests: lo.Map(items, changeRequestOut)}, nil
}

type OkOut struct {
	Ok bool `json:"ok"`
}

func (s *sessionServer) cancelChangeRequest(ctx context.Context, req *mcpsdk.CallToolRequest, in ChangeRequestIdIn) (*mcpsdk.CallToolResult, OkOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, OkOut{}, err
	}

	if err = s.h.changeRequestUc.Cancel(ctx, in.Id); err != nil {
		return nil, OkOut{}, toolErr(err)
	}

	return nil, OkOut{Ok: true}, nil
}

func changeRequestOut(v *changeRequestModel.ChangeRequest, _ int) ChangeRequestOut {
	return ChangeRequestOut{
		Id:           v.Id,
		Status:       v.Status,
		Title:        v.Title,
		Description:  v.Description,
		Publish:      v.Publish,
		CreatedAt:    v.CreatedAt.Format(time.RFC3339),
		DecidedAt:    timeOut(v.DecidedAt),
		Comment:      v.Comment,
		Error:        v.Error,
		Created:      v.Result.Created,
		PublishedAt:  timeOut(v.PublishedAt),
		PublishError: v.PublishError,
	}
}

func timeOut(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.Format(time.RFC3339)
}
