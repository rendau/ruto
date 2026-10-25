package changerequest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	"github.com/rendau/ruto/internal/domain/changerequest/model"
	"github.com/rendau/ruto/internal/domain/secretref"
	seenPathModel "github.com/rendau/ruto/internal/domain/seenpath/model"
)

const maxSeenPathRows = 1000

// parseSeenPaths reads the data of delete_seen_paths with rows normalized
// the way the gateway stores them: upper-case method, path without a
// leading slash.
func parseSeenPaths(op *model.Op) (*model.SeenPathsData, error) {
	var data model.SeenPathsData
	if err := json.Unmarshal(op.Data, &data); err != nil {
		return nil, invalid("data: " + err.Error())
	}

	data.App = strings.TrimSpace(data.App)
	if data.App == "" {
		return nil, invalid("data.app is required: app id or name")
	}
	if isRef(data.App) {
		return nil, invalid("data.app must be an existing app: an app created by the request has no report yet")
	}
	if len(data.Rows) == 0 {
		return nil, invalid("data.rows is empty")
	}
	if len(data.Rows) > maxSeenPathRows {
		return nil, invalid(fmt.Sprintf("data.rows: at most %d rows per operation", maxSeenPathRows))
	}

	rows := make([]model.SeenPathRow, 0, len(data.Rows))
	for i, row := range data.Rows {
		row.Method = strings.ToUpper(strings.TrimSpace(row.Method))
		row.Path = strings.TrimLeft(strings.TrimSpace(row.Path), "/")
		if row.Method == "" {
			return nil, invalid(fmt.Sprintf("data.rows[%d]: method is required", i))
		}
		rows = append(rows, row)
	}
	data.Rows = lo.Uniq(rows)

	return &data, nil
}

// prepareSeenPaths pins a proposed delete_seen_paths to the app id and
// checks every row is in the report, so the reviewer sees what goes away.
func (u *Usecase) prepareSeenPaths(ctx context.Context, op *model.Op) error {
	data, err := parseSeenPaths(op)
	if err != nil {
		return err
	}

	app, err := u.findApp(ctx, data.App)
	if err != nil {
		return err
	}
	data.App = app.Id

	items, err := u.listSeenPaths(ctx, data)
	if err != nil {
		return err
	}
	found := lo.SliceToMap(items, func(v *seenPathModel.SeenPath) (model.SeenPathRow, bool) {
		return model.SeenPathRow{Method: v.Method, Path: v.Path}, true
	})
	missing := lo.Filter(data.Rows, func(v model.SeenPathRow, _ int) bool { return !found[v] })
	if len(missing) > 0 {
		return invalid(fmt.Sprintf("not in the seen paths report of app %q: %s",
			app.Name, strings.Join(lo.Map(missing, seenPathRowTitle), ", ")))
	}

	if op.Data, err = json.Marshal(data); err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	return nil
}

// findApp looks an app up by exact name (case-insensitive), then by id.
func (u *Usecase) findApp(ctx context.Context, ref string) (*appModel.App, error) {
	items, _, err := u.appSvc.List(ctx, &appModel.ListReq{NameEqCI: &ref})
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}
	if len(items) == 1 {
		return items[0], nil
	}

	item, found, err := u.appSvc.Get(ctx, ref, false)
	if err != nil {
		return nil, fmt.Errorf("appSvc.Get: %w", err)
	}
	if !found {
		return nil, errNotFound(secretref.KindApp, ref)
	}

	return item, nil
}

// listSeenPaths returns the stored rows matching the operation's rows; one
// row may be counted by several wildcard endpoints.
func (u *Usecase) listSeenPaths(ctx context.Context, data *model.SeenPathsData) ([]*seenPathModel.SeenPath, error) {
	items, err := u.seenPathSvc.ListByPaths(ctx, data.App, lo.Map(data.Rows, func(v model.SeenPathRow, _ int) *seenPathModel.PathKey {
		return &seenPathModel.PathKey{Method: v.Method, Path: v.Path}
	}))
	if err != nil {
		return nil, fmt.Errorf("seenPathSvc.ListByPaths: %w", err)
	}
	return items, nil
}

// deleteSeenPaths applies delete_seen_paths. Rows that left the report since
// the proposal (cleared in the admin UI) are already gone, which is the goal.
func (u *Usecase) deleteSeenPaths(ctx context.Context, op *model.Op) error {
	data, err := parseSeenPaths(op)
	if err != nil {
		return err
	}

	items, err := u.listSeenPaths(ctx, data)
	if err != nil {
		return err
	}

	return u.seenPathUc.Delete(ctx, data.App, lo.Map(items, func(v *seenPathModel.SeenPath, _ int) *seenPathModel.SeenPathKey {
		return &seenPathModel.SeenPathKey{EndpointId: v.EndpointId, Method: v.Method, Path: v.Path}
	}))
}

// seenPathsView renders the rows a delete_seen_paths removes, with the
// counters they have now.
func (u *Usecase) seenPathsView(ctx context.Context, cache *objectCache, op *model.Op, view *OpView) (*OpView, error) {
	data, err := parseSeenPaths(op)
	if err != nil {
		view.Notes = append(view.Notes, "cannot render the change: "+errDesc(err))
		return view, nil
	}

	view.TargetKind = secretref.KindApp
	view.TargetId = data.App
	view.AppName = u.appName(ctx, cache, data.App, nil)

	items, err := u.listSeenPaths(ctx, data)
	if err != nil {
		return nil, err
	}

	byRow := map[model.SeenPathRow]*SeenPathView{}
	for _, item := range items {
		row := model.SeenPathRow{Method: item.Method, Path: item.Path}
		if v, ok := byRow[row]; ok {
			v.Hits += item.Hits
			v.HitsNotFound += item.HitsNotFound
			continue
		}
		byRow[row] = &SeenPathView{
			Method:       item.Method,
			Path:         item.Path,
			Hits:         item.Hits,
			HitsNotFound: item.HitsNotFound,
			Sample:       item.Sample,
		}
	}

	for _, row := range data.Rows {
		if v, ok := byRow[row]; ok {
			view.SeenPaths = append(view.SeenPaths, v)
			continue
		}
		view.SeenPaths = append(view.SeenPaths, &SeenPathView{Method: row.Method, Path: row.Path})
		view.Notes = append(view.Notes, seenPathRowTitle(row, 0)+" is no longer in the report")
	}

	return view, nil
}

func seenPathRowTitle(v model.SeenPathRow, _ int) string {
	return v.Method + " /" + v.Path
}
