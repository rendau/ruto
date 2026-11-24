package apikey

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/domain/apikey/model"
	apikeyService "github.com/rendau/ruto/internal/domain/apikey/service"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	usrModel "github.com/rendau/ruto/internal/domain/usr/model"
	"github.com/rendau/ruto/internal/errs"
)

type testSvc struct {
	items   []*model.ApiKey
	deleted []string
}

func (s *testSvc) List(_ context.Context, pars *model.ListReq) ([]*model.ApiKey, int64, error) {
	return s.items, int64(len(s.items)), nil
}

func (s *testSvc) Get(_ context.Context, id string, errNE bool) (*model.ApiKey, bool, error) {
	for _, v := range s.items {
		if v.Id == id {
			return v, true, nil
		}
	}
	return nil, false, errs.ObjectNotFound
}

func (s *testSvc) GetByKey(_ context.Context, key string) (*model.ApiKey, bool, error) {
	for _, v := range s.items {
		if v.KeyHash == apikeyService.HashKey(key) {
			return v, true, nil
		}
	}
	return nil, false, nil
}

func (s *testSvc) Create(_ context.Context, obj *model.ApiKey) (string, error) {
	obj.Id = "key-" + obj.Name
	s.items = append(s.items, obj)
	return obj.Id, nil
}

func (s *testSvc) Delete(_ context.Context, id string) error {
	s.deleted = append(s.deleted, id)
	return nil
}

func (s *testSvc) TouchLastUsed(context.Context, string) error { return nil }

type testUsr struct{}

func (u *testUsr) Get(_ context.Context, id int64, _ bool) (*usrModel.Usr, bool, error) {
	return &usrModel.Usr{Id: id, Active: true, IsAdmin: true, Name: "Ivan"}, true, nil
}

type ctxKey struct{}

type testSession struct{}

func (s *testSession) FromContext(ctx context.Context) *sessionModel.Session {
	if v, ok := ctx.Value(ctxKey{}).(*sessionModel.Session); ok {
		return v
	}
	return sessionModel.New(0)
}

type testBlocks struct {
	removed []string
}

func (b *testBlocks) RemoveKeyBlocks(_ context.Context, apiKeyId, keyName string) (int64, error) {
	b.removed = append(b.removed, apiKeyId+"/"+keyName)
	return 1, nil
}

func human() context.Context {
	return context.WithValue(context.Background(), ctxKey{}, &sessionModel.Session{Id: 1, Admin: true})
}

func TestScope(t *testing.T) {
	svc := &testSvc{}
	uc := New(svc, &testUsr{}, &testSession{}, &testBlocks{})

	_, fullKey, err := uc.Create(human(), "claude", "")
	require.NoError(t, err)
	_, blocklistKey, err := uc.Create(human(), "pulse", model.ScopeBlocklist)
	require.NoError(t, err)
	_, _, err = uc.Create(human(), "x", "admin")
	assert.Error(t, err)

	assert.Equal(t, model.ScopeFull, svc.items[0].Scope, "full by default")
	assert.Equal(t, model.ScopeBlocklist, svc.items[1].Scope)

	_, scope, err := uc.Authenticate(context.Background(), fullKey)
	require.NoError(t, err)
	assert.Equal(t, model.ScopeFull, scope)
	session, scope, err := uc.Authenticate(context.Background(), blocklistKey)
	require.NoError(t, err)
	assert.Equal(t, model.ScopeBlocklist, scope)
	assert.Equal(t, "key-pulse", session.ApiKeyId)

	// a key stored before scopes existed
	svc.items[0].Scope = ""
	_, scope, err = uc.Authenticate(context.Background(), fullKey)
	require.NoError(t, err)
	assert.Equal(t, model.ScopeFull, scope)
}

// Deleting a key lifts its active blocks.
func TestDeleteLiftsBlocks(t *testing.T) {
	svc := &testSvc{items: []*model.ApiKey{{Id: "k1", UsrId: 1, Name: "pulse"}, {Id: "k2", UsrId: 2, Name: "other"}}}
	blocks := &testBlocks{}
	uc := New(svc, &testUsr{}, &testSession{}, blocks)

	require.NoError(t, uc.Delete(human(), "k1"))
	assert.Equal(t, []string{"k1"}, svc.deleted)
	assert.Equal(t, []string{"k1/pulse"}, blocks.removed)

	assert.Equal(t, errs.NoPermission, uc.Delete(human(), "k2"), "another admin's key")
	assert.Len(t, blocks.removed, 1)
}
