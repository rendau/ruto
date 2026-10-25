package changerequest

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	"github.com/rendau/ruto/internal/domain/changerequest/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/domain/secretref"
	"github.com/rendau/ruto/internal/errs"
)

const maxOps = 200

// opTarget returns the object an update/delete operation addresses.
func opTarget(op *model.Op) (kind, id string, ok bool) {
	switch op.Op {
	case model.OpUpdateApp, model.OpDeleteApp:
		return secretref.KindApp, op.Id, true
	case model.OpUpdateEndpoint, model.OpDeleteEndpoint:
		return secretref.KindEndpoint, op.Id, true
	case model.OpUpdateRoot:
		return secretref.KindRoot, "", true
	}
	return "", "", false
}

func isRef(v string) bool {
	return strings.HasPrefix(v, "$")
}

// checkShape validates operations without touching the database.
func checkShape(ops []*model.Op) error {
	if len(ops) == 0 {
		return invalid("ops: empty")
	}
	if len(ops) > maxOps {
		return invalid(fmt.Sprintf("ops: at most %d operations per request", maxOps))
	}

	refs := map[string]bool{}

	for i, op := range ops {
		at := fmt.Sprintf("ops[%d] (%s)", i, op.Op)

		switch op.Op {
		case model.OpCreateApp:
			if op.Ref != "" {
				if !isRef(op.Ref) {
					return invalid(at + ": ref must start with $")
				}
				if refs[op.Ref] {
					return invalid(at + ": duplicate ref " + op.Ref)
				}
				refs[op.Ref] = true
			}
		case model.OpCreateEndpoint:
			if op.App == "" {
				return invalid(at + ": app is required")
			}
			if isRef(op.App) && !refs[op.App] {
				return invalid(at + ": unknown ref " + op.App + " (create_app with this ref must come earlier)")
			}
		case model.OpUpdateApp, model.OpDeleteApp, model.OpUpdateEndpoint, model.OpDeleteEndpoint:
			if op.Id == "" {
				return invalid(at + ": id is required")
			}
			if isRef(op.Id) {
				return invalid(at + ": id must be an existing object, not a ref")
			}
		case model.OpUpdateRoot, model.OpDeleteSeenPaths:
		default:
			return invalid(at + ": unknown op")
		}

		needsData := op.Op != model.OpDeleteApp && op.Op != model.OpDeleteEndpoint
		if needsData && len(op.Data) == 0 {
			return invalid(at + ": data is required")
		}
		if !needsData && len(op.Data) > 0 {
			return invalid(at + ": data is not allowed")
		}
	}

	return nil
}

// hasObjectData reports whether op.Data is a configuration object, which may
// hold secret value sources.
func hasObjectData(op *model.Op) bool {
	return len(op.Data) > 0 && op.Op != model.OpDeleteSeenPaths
}

// sourceSet holds the value sources of one request, resolved together so
// every ref is read from the state before any operation runs.
type sourceSet struct {
	generated map[string]string
}

// parseOpData parses an op's data and its secret value sources.
func parseOpData(op *model.Op) (map[string]any, map[string]*secretref.Source, error) {
	var tree map[string]any
	if err := json.Unmarshal(op.Data, &tree); err != nil {
		return nil, nil, fmt.Errorf("data: %w", err)
	}

	sources := map[string]*secretref.Source{}
	_, err := secretref.Rewrite(tree, func(path []string, node any) (any, error) {
		src, err := secretref.ParseSource(node)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", secretref.FormatPath(path), err)
		}
		sources[strings.Join(path, "/")] = src
		return node, nil
	})
	if err != nil {
		return nil, nil, err
	}

	return tree, sources, nil
}

// collectRefs returns the objects the request's value sources read from and
// checks reuse names point to a generate of the same request.
func collectRefs(ops []*model.Op) (map[string]*secretref.Ref, error) {
	refs := map[string]*secretref.Ref{}
	generated := map[string]bool{}
	var reused []string

	for i, op := range ops {
		if !hasObjectData(op) {
			continue
		}
		_, sources, err := parseOpData(op)
		if err != nil {
			return nil, invalid(fmt.Sprintf("ops[%d] (%s): %s", i, op.Op, err))
		}
		for _, src := range sources {
			switch {
			case src.Ref != nil:
				refs[src.Ref.Object()] = src.Ref
			case src.Generate != nil && src.Generate.Name != "":
				generated[src.Generate.Name] = true
			case src.Reuse != "":
				reused = append(reused, src.Reuse)
			}
		}
	}

	for _, name := range reused {
		if !generated[name] {
			return nil, invalid(fmt.Sprintf("reuse %q: no generate with this name in the request", name))
		}
	}

	return refs, nil
}

func (s *sourceSet) generateNamed(src *secretref.Source) error {
	if src.Generate == nil || src.Generate.Name == "" {
		return nil
	}
	if _, ok := s.generated[src.Generate.Name]; ok {
		return nil
	}
	value, err := src.Generate.Value()
	if err != nil {
		return err
	}
	s.generated[src.Generate.Name] = value
	return nil
}

// resolve replaces every value source in tree with a concrete value.
func (s *sourceSet) resolve(ctx context.Context, cache *objectCache, tree map[string]any) (map[string]any, error) {
	result, err := secretref.Rewrite(tree, func(path []string, node any) (any, error) {
		src, err := secretref.ParseSource(node)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", secretref.FormatPath(path), err)
		}

		switch {
		case src.Literal != nil:
			return *src.Literal, nil
		case src.Ref != nil:
			value, ok, err := cache.lookupRef(ctx, src.Ref)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, invalid(fmt.Sprintf("%s: secret_ref %s points to no value", secretref.FormatPath(path), src.Ref.String()))
			}
			return value, nil
		case src.Generate != nil:
			if src.Generate.Name != "" {
				return s.generated[src.Generate.Name], nil
			}
			return src.Generate.Value()
		default:
			value, ok := s.generated[src.Reuse]
			if !ok {
				return nil, invalid(fmt.Sprintf("%s: reuse %q: no generate with this name in the request", secretref.FormatPath(path), src.Reuse))
			}
			return value, nil
		}
	})
	if err != nil {
		return nil, err
	}

	resultMap, _ := result.(map[string]any)
	return resultMap, nil
}

func decodeTree[T any](tree map[string]any) (*T, error) {
	raw, err := json.Marshal(tree)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}
	var result T
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, invalid("data: " + err.Error())
	}
	return &result, nil
}

// checkVersions fails with ChangeRequestOutdated if any object the request
// reads or writes changed since the proposal.
func (u *Usecase) checkVersions(ctx context.Context, cache *objectCache, versions map[string]string) error {
	for key, version := range versions {
		kind, id, _ := strings.Cut(key, ":")
		item, found, err := cache.get(ctx, kind, id)
		if err != nil {
			return err
		}
		if !found {
			return outdated(key + " no longer exists")
		}
		if item.Version != version {
			return outdated(key + " was changed after the request was made")
		}
	}
	return nil
}

// applyPlan performs the operations in ctx's transaction and validates the
// resulting configuration. The caller's session authorizes each write.
func (u *Usecase) applyPlan(ctx context.Context, plan *model.Plan) (*model.Result, error) {
	cache := u.newObjectCache()

	if err := u.checkVersions(ctx, cache, plan.Versions); err != nil {
		return nil, err
	}

	// Resolve every op's data before the first write: refs must read the
	// state the request was reviewed against.
	sources := &sourceSet{generated: map[string]string{}}
	trees := make([]map[string]any, len(plan.Ops))
	for i, op := range plan.Ops {
		if !hasObjectData(op) {
			continue
		}
		tree, opSources, err := parseOpData(op)
		if err != nil {
			return nil, invalid(fmt.Sprintf("ops[%d]: %s", i, err))
		}
		trees[i] = tree
		// named values first, so a reuse does not depend on field order
		for _, src := range opSources {
			if err = sources.generateNamed(src); err != nil {
				return nil, wrapOp(i, op, err)
			}
		}
	}

	resolved := make([]map[string]any, len(plan.Ops))
	for i, op := range plan.Ops {
		if trees[i] == nil {
			continue
		}
		var err error
		if resolved[i], err = sources.resolve(ctx, cache, trees[i]); err != nil {
			return nil, wrapOp(i, op, err)
		}
	}

	result := &model.Result{Created: map[string]string{}}

	resolveApp := func(v string) string {
		if isRef(v) {
			return result.Created[v]
		}
		return v
	}

	for i, op := range plan.Ops {
		if err := u.applyOp(ctx, op, resolved[i], resolveApp, result, i); err != nil {
			return nil, wrapOp(i, op, err)
		}
	}

	if err := u.snapshotUc.Validate(ctx); err != nil {
		return nil, invalid("the resulting configuration is invalid: " + err.Error())
	}

	return result, nil
}

func (u *Usecase) applyOp(
	ctx context.Context,
	op *model.Op,
	data map[string]any,
	resolveApp func(string) string,
	result *model.Result,
	idx int,
) error {
	switch op.Op {
	case model.OpCreateApp:
		obj, err := decodeTree[appModel.App](data)
		if err != nil {
			return err
		}
		obj.Id, obj.Endpoints = "", nil
		newId, err := u.appUc.Create(ctx, obj)
		if err != nil {
			return err
		}
		result.Created["#"+strconv.Itoa(idx)] = newId
		if op.Ref != "" {
			result.Created[op.Ref] = newId
		}

	case model.OpUpdateApp:
		obj, err := decodeTree[appModel.App](data)
		if err != nil {
			return err
		}
		obj.Id, obj.Endpoints = op.Id, nil
		return u.appUc.Update(ctx, op.Id, obj)

	case model.OpDeleteApp:
		return u.appUc.Delete(ctx, op.Id)

	case model.OpCreateEndpoint:
		obj, err := decodeTree[endpointModel.Endpoint](data)
		if err != nil {
			return err
		}
		obj.Id, obj.AppId = "", resolveApp(op.App)
		newId, err := u.endpointUc.Create(ctx, obj)
		if err != nil {
			return err
		}
		result.Created["#"+strconv.Itoa(idx)] = newId

	case model.OpUpdateEndpoint:
		obj, err := decodeTree[endpointModel.Endpoint](data)
		if err != nil {
			return err
		}
		obj.Id = op.Id
		if obj.AppId == "" {
			current, _, err := u.endpointSvc.Get(ctx, op.Id, true)
			if err != nil {
				return err
			}
			obj.AppId = current.AppId
		}
		obj.AppId = resolveApp(obj.AppId)
		return u.endpointUc.Update(ctx, op.Id, obj)

	case model.OpDeleteEndpoint:
		return u.endpointUc.Delete(ctx, op.Id)

	case model.OpUpdateRoot:
		obj, err := decodeTree[rootModel.Root](data)
		if err != nil {
			return err
		}
		obj.Apps = nil
		return u.rootUc.Set(ctx, obj)

	case model.OpDeleteSeenPaths:
		return u.deleteSeenPaths(ctx, op)
	}

	return nil
}

func wrapOp(i int, op *model.Op, err error) error {
	if base, ok := errsBase(err); ok {
		return errs.ErrFull{Err: base, Desc: fmt.Sprintf("ops[%d] (%s): %s", i, op.Op, errDesc(err))}
	}
	return fmt.Errorf("ops[%d] (%s): %w", i, op.Op, err)
}
