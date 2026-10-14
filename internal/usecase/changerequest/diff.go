package changerequest

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	"github.com/rendau/ruto/internal/domain/changerequest/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/domain/secretref"
)

// Secret leaves are compared through markers, so the diff never holds a
// secret value: a stored secret becomes the ref of its own location, and a
// source in the request becomes a marker of that source.
const (
	markerRef    = "\x00ref:"
	markerGen    = "\x00gen:"
	markerReuse  = "\x00reuse:"
	maxValueShow = 2000
)

func (u *Usecase) detail(ctx context.Context, item *model.ChangeRequest) (*Detail, error) {
	result := &Detail{ChangeRequest: item}

	result.AuthorName = u.usrName(ctx, item.AuthorUsrId)
	result.DecidedName = u.usrName(ctx, item.DecidedUsrId)

	if len(item.Result.Review) > 0 {
		if err := json.Unmarshal(item.Result.Review, &result.Ops); err == nil {
			return result, nil
		}
	}

	cache := u.newObjectCache()

	if !item.IsClosed() {
		if err := u.checkVersions(ctx, cache, item.Plan.Versions); err != nil {
			result.Outdated = true
			result.OutdatedReason = errDesc(err)
		}
	}

	ops, err := u.opViews(ctx, cache, item)
	if err != nil {
		return nil, err
	}
	result.Ops = ops

	return result, nil
}

// review renders the request's diff for storing with the decision.
func (u *Usecase) review(ctx context.Context, item *model.ChangeRequest) (json.RawMessage, error) {
	ops, err := u.opViews(ctx, u.newObjectCache(), item)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(ops)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}
	return raw, nil
}

func (u *Usecase) opViews(ctx context.Context, cache *objectCache, item *model.ChangeRequest) ([]*OpView, error) {
	var result []*OpView

	refNames := map[string]string{}
	for _, op := range item.Plan.Ops {
		if op.Op == model.OpCreateApp && op.Ref != "" {
			var data struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(op.Data, &data)
			refNames[op.Ref] = data.Name
		}
	}

	for _, op := range item.Plan.Ops {
		view, err := u.opView(ctx, cache, op, refNames)
		if err != nil {
			return nil, err
		}
		result = append(result, view)
	}

	return result, nil
}

func (u *Usecase) usrName(ctx context.Context, id int64) string {
	if id == 0 {
		return ""
	}
	usr, found, err := u.usrSvc.Get(ctx, id, false)
	if err != nil || !found {
		return "#" + strconv.FormatInt(id, 10)
	}
	return usr.Name
}

func (u *Usecase) opView(ctx context.Context, cache *objectCache, op *model.Op, refNames map[string]string) (*OpView, error) {
	view := &OpView{Op: op.Op, TargetId: op.Id}

	var current *object
	if kind, id, ok := opTarget(op); ok {
		view.TargetKind = kind
		item, found, err := cache.get(ctx, kind, id)
		if err != nil {
			return nil, err
		}
		if !found {
			view.Notes = append(view.Notes, kind+" "+id+" no longer exists")
		}
		current = item
	}

	switch op.Op {
	case model.OpCreateApp:
		view.TargetKind = secretref.KindApp
		view.TargetId = op.Ref
	case model.OpCreateEndpoint:
		view.TargetKind = secretref.KindEndpoint
		view.AppName = u.appName(ctx, cache, op.App, refNames)
	case model.OpUpdateRoot:
		view.Title = "root"
	}

	if current != nil {
		switch v := current.Value.(type) {
		case *appModel.App:
			view.Title = v.Name
		case *endpointModel.Endpoint:
			view.Title = endpointTitle(v)
			view.AppName = u.appName(ctx, cache, v.AppId, refNames)
		}
	}

	if op.Op == model.OpDeleteApp && current != nil {
		_, count, err := u.endpointSvc.List(ctx, &endpointModel.ListReq{
			ListParams: listOnlyCount(),
			AppId:      &op.Id,
		})
		if err != nil {
			return nil, fmt.Errorf("endpointSvc.List: %w", err)
		}
		if count > 0 {
			view.Notes = append(view.Notes, fmt.Sprintf("all %d endpoints of this app are deleted with it", count))
		}
	}

	if len(op.Data) == 0 {
		return view, nil
	}

	after, err := canonicalAfter(op)
	if err != nil {
		view.Notes = append(view.Notes, "cannot render the change: "+err.Error())
		return view, nil
	}
	if view.Title == "" {
		view.Title = titleOf(op, after)
	}

	var before map[string]any
	if current != nil {
		before = markStored(current)
	}
	dropIdentity(op, before)
	dropIdentity(op, after)

	beforeLeaves := flatten(before)
	afterLeaves := flatten(after)

	keys := make([]string, 0, len(beforeLeaves)+len(afterLeaves))
	for k := range beforeLeaves {
		keys = append(keys, k)
	}
	for k := range afterLeaves {
		if _, ok := beforeLeaves[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.SortFunc(keys, comparePaths)

	isCreate := current == nil
	for _, key := range keys {
		b, hasBefore := beforeLeaves[key]
		a, hasAfter := afterLeaves[key]
		if hasBefore && hasAfter && b == a {
			continue
		}
		if isCreate && isZero(a) {
			continue
		}

		path := strings.Split(key, "/")
		change := &Change{Path: secretref.FormatPath(path), Secret: secretref.IsSecretPath(path)}
		if hasBefore {
			change.Before = u.renderLeaf(ctx, cache, b, true)
		}
		if hasAfter {
			change.After = u.renderLeaf(ctx, cache, a, false)
		}
		view.Changes = append(view.Changes, change)
	}

	return view, nil
}

func (u *Usecase) appName(ctx context.Context, cache *objectCache, app string, refNames map[string]string) string {
	if isRef(app) {
		return refNames[app] + " (new)"
	}
	item, found, err := cache.get(ctx, secretref.KindApp, app)
	if err != nil || !found {
		return app
	}
	return item.Value.(*appModel.App).Name
}

func endpointTitle(ep *endpointModel.Endpoint) string {
	if ep.Type == endpointModel.TypeGRPC {
		return "gRPC " + ep.Grpc.Path
	}
	return ep.Http.Method + " /" + strings.TrimPrefix(ep.Http.Path, "/")
}

func titleOf(op *model.Op, after map[string]any) string {
	switch op.Op {
	case model.OpCreateApp:
		name, _ := after["name"].(string)
		return name
	case model.OpCreateEndpoint:
		ep, err := decodeTree[endpointModel.Endpoint](after)
		if err == nil {
			return endpointTitle(ep)
		}
	}
	return ""
}

// canonicalAfter turns the op's data into the shape of a stored object (all
// fields present, unknown ones dropped) with sources replaced by markers.
func canonicalAfter(op *model.Op) (map[string]any, error) {
	tree, _, err := parseOpData(op)
	if err != nil {
		return nil, err
	}

	marked, err := secretref.Rewrite(tree, func(_ []string, node any) (any, error) {
		src, err := secretref.ParseSource(node)
		if err != nil {
			return nil, err
		}
		switch {
		case src.Literal != nil:
			return *src.Literal, nil
		case src.Ref != nil:
			return markerRef + src.Ref.String(), nil
		case src.Generate != nil:
			return fmt.Sprintf("%s%s:%d", markerGen, src.Generate.Format, src.Generate.Chars()), nil
		default:
			return markerReuse + src.Reuse, nil
		}
	})
	if err != nil {
		return nil, err
	}
	markedMap, _ := marked.(map[string]any)

	var value any
	switch op.Op {
	case model.OpCreateApp, model.OpUpdateApp:
		value, err = decodeTree[appModel.App](markedMap)
	case model.OpCreateEndpoint, model.OpUpdateEndpoint:
		value, err = decodeTree[endpointModel.Endpoint](markedMap)
	case model.OpUpdateRoot:
		value, err = decodeTree[rootModel.Root](markedMap)
	}
	if err != nil {
		return nil, err
	}

	result, err := secretref.ToTree(value)
	if err != nil {
		return nil, err
	}
	resultMap, _ := result.(map[string]any)
	delete(resultMap, "apps")
	delete(resultMap, "endpoints")

	return resultMap, nil
}

// markStored replaces stored secrets by the refs of their own locations.
func markStored(item *object) map[string]any {
	marked, _ := secretref.Rewrite(item.Tree, func(path []string, _ any) (any, error) {
		ref := &secretref.Ref{Kind: item.Kind, Id: item.Id, Path: path}
		return markerRef + ref.String(), nil
	})
	result, _ := marked.(map[string]any)
	return result
}

// dropIdentity removes fields the operation fixes itself.
func dropIdentity(op *model.Op, tree map[string]any) {
	if tree == nil {
		return
	}
	delete(tree, "id")
	if op.Op == model.OpCreateEndpoint {
		delete(tree, "app_id")
	}
}

// renderLeaf shows a leaf for review; stored is true for the current value.
func (u *Usecase) renderLeaf(ctx context.Context, cache *objectCache, leaf string, stored bool) string {
	switch {
	case strings.HasPrefix(leaf, markerRef):
		ref, err := secretref.ParseRef(strings.TrimPrefix(leaf, markerRef))
		if err != nil {
			return "secret"
		}
		value, ok, _ := cache.lookupRef(ctx, ref)
		if stored {
			return secretref.MaskedLiteralChars(value)
		}
		label := u.refLabel(ctx, cache, ref)
		if !ok {
			return "secret copied from " + label + " (source not found)"
		}
		return fmt.Sprintf("secret copied from %s (%d chars)", label, utf8.RuneCountInString(value))
	case strings.HasPrefix(leaf, markerGen):
		format, chars, _ := strings.Cut(strings.TrimPrefix(leaf, markerGen), ":")
		return fmt.Sprintf("new random secret (%s, %s chars)", format, chars)
	case strings.HasPrefix(leaf, markerReuse):
		return fmt.Sprintf("same value as generated %q", strings.TrimPrefix(leaf, markerReuse))
	}

	if len(leaf) > maxValueShow {
		return leaf[:maxValueShow] + "…"
	}
	return leaf
}

// refLabel names a secret location for people: app "orders" variables.DB_PASS.
func (u *Usecase) refLabel(ctx context.Context, cache *objectCache, ref *secretref.Ref) string {
	path := secretref.FormatPath(ref.Path)

	item, found, err := cache.get(ctx, ref.Kind, ref.Id)
	if err != nil || !found {
		return ref.String()
	}

	switch v := item.Value.(type) {
	case *appModel.App:
		return fmt.Sprintf("app %q %s", v.Name, path)
	case *endpointModel.Endpoint:
		return fmt.Sprintf("endpoint %q of app %q %s", endpointTitle(v), u.appName(ctx, cache, v.AppId, nil), path)
	default:
		return "root " + path
	}
}

// flatten maps "a/b/0/c" paths to leaf values rendered as text.
func flatten(tree map[string]any) map[string]string {
	result := map[string]string{}
	if tree != nil {
		flattenInto(tree, nil, result)
	}
	return result
}

func flattenInto(node any, path []string, out map[string]string) {
	switch v := node.(type) {
	case map[string]any:
		if len(v) == 0 && len(path) > 0 {
			out[strings.Join(path, "/")] = "{}"
			return
		}
		for k, child := range v {
			flattenInto(child, append(slices.Clone(path), k), out)
		}
	case []any:
		if len(v) == 0 {
			out[strings.Join(path, "/")] = "[]"
			return
		}
		for i, child := range v {
			flattenInto(child, append(slices.Clone(path), strconv.Itoa(i)), out)
		}
	case string:
		out[strings.Join(path, "/")] = v
	default:
		raw, _ := json.Marshal(v)
		out[strings.Join(path, "/")] = string(raw)
	}
}

func isZero(v string) bool {
	return v == "" || v == "false" || v == "0" || v == "null" || v == "[]" || v == "{}"
}

// comparePaths orders paths segment by segment, numbers numerically.
func comparePaths(a, b string) int {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		an, aErr := strconv.Atoi(as[i])
		bn, bErr := strconv.Atoi(bs[i])
		if aErr == nil && bErr == nil {
			return an - bn
		}
		return strings.Compare(as[i], bs[i])
	}
	return len(as) - len(bs)
}
