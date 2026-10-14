package changerequest

import (
	"context"
	"fmt"

	"github.com/rendau/ruto/internal/domain/secretref"
)

// object is the stored state of one configuration object.
type object struct {
	Kind    string
	Id      string
	Value   any
	Tree    map[string]any
	Version string
}

func (o *object) Key() string {
	return secretref.ObjectKey(o.Kind, o.Id)
}

// loadObject reads an object as stored; found=false for a missing app or
// endpoint.
func (u *Usecase) loadObject(ctx context.Context, kind, id string) (*object, bool, error) {
	var value any

	switch kind {
	case secretref.KindRoot:
		item, err := u.rootSvc.Get(ctx)
		if err != nil {
			return nil, false, fmt.Errorf("rootSvc.Get: %w", err)
		}
		item.Apps = nil
		value = item
	case secretref.KindApp:
		item, found, err := u.appSvc.Get(ctx, id, false)
		if err != nil {
			return nil, false, fmt.Errorf("appSvc.Get: %w", err)
		}
		if !found {
			return nil, false, nil
		}
		item.Endpoints = nil
		value = item
	case secretref.KindEndpoint:
		item, found, err := u.endpointSvc.Get(ctx, id, false)
		if err != nil {
			return nil, false, fmt.Errorf("endpointSvc.Get: %w", err)
		}
		if !found {
			return nil, false, nil
		}
		value = item
	default:
		return nil, false, fmt.Errorf("unknown object kind %q", kind)
	}

	version, err := secretref.Version(value)
	if err != nil {
		return nil, false, fmt.Errorf("secretref.Version: %w", err)
	}

	tree, err := secretref.ToTree(value)
	if err != nil {
		return nil, false, fmt.Errorf("secretref.ToTree: %w", err)
	}
	treeMap, _ := tree.(map[string]any)
	delete(treeMap, "apps")
	delete(treeMap, "endpoints")

	return &object{Kind: kind, Id: id, Value: value, Tree: treeMap, Version: version}, true, nil
}

// objectCache memoizes loadObject within one operation.
type objectCache struct {
	u     *Usecase
	items map[string]*object
}

func (u *Usecase) newObjectCache() *objectCache {
	return &objectCache{u: u, items: map[string]*object{}}
}

func (c *objectCache) get(ctx context.Context, kind, id string) (*object, bool, error) {
	key := secretref.ObjectKey(kind, id)
	if item, ok := c.items[key]; ok {
		return item, item != nil, nil
	}

	item, found, err := c.u.loadObject(ctx, kind, id)
	if err != nil {
		return nil, false, err
	}
	c.items[key] = item

	return item, found, nil
}

// lookupRef returns the current value a ref points to.
func (c *objectCache) lookupRef(ctx context.Context, ref *secretref.Ref) (string, bool, error) {
	item, found, err := c.get(ctx, ref.Kind, ref.Id)
	if err != nil || !found {
		return "", false, err
	}
	value, ok := secretref.Lookup(item.Tree, ref.Path)
	return value, ok, nil
}

// ObjectView is an object as an agent sees it: secrets masked, plus the
// version to pass back with an update.
type ObjectView struct {
	Kind    string
	Id      string
	Version string
	Data    any
}

// View returns the masked object. Root uses an empty id.
func (u *Usecase) View(ctx context.Context, kind, id string) (*ObjectView, error) {
	if !u.sessionSvc.FromContext(ctx).IsAdmin() {
		return nil, errNoPermission()
	}

	item, found, err := u.loadObject(ctx, kind, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errNotFound(kind, id)
	}

	return &ObjectView{
		Kind:    kind,
		Id:      id,
		Version: item.Version,
		Data:    secretref.Mask(kind, id, item.Tree),
	}, nil
}
