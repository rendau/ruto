package usr

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	"github.com/rendau/ruto/internal/domain/usr/model"
	"github.com/rendau/ruto/internal/errs"
)

type testSessionService struct {
	session *sessionModel.Session
}

func (s *testSessionService) FromContext(_ context.Context) *sessionModel.Session {
	return s.session
}

func (s *testSessionService) CtxIsAuthorized(_ context.Context) bool {
	return s.session.IsAuthorized()
}

func (s *testSessionService) CtxIsAdmin(_ context.Context) bool {
	return s.session.IsAdmin()
}

func (s *testSessionService) CreateToken(_ int64, _ bool, _ bool, _ []string) (string, error) {
	panic("unexpected call")
}

type testUsrService struct {
	get             func(ctx context.Context, id int64) (*model.Usr, error)
	passwordMatches func(item *model.Usr, password string) (bool, error)
	hasAny          func(ctx context.Context) (bool, error)
	create          func(ctx context.Context, obj *model.Edit) (int64, error)
	update          func(ctx context.Context, id int64, obj *model.Edit) error
}

func (s *testUsrService) List(_ context.Context, _ *model.ListReq) ([]*model.Usr, int64, error) {
	panic("unexpected call")
}

func (s *testUsrService) Get(ctx context.Context, id int64, _ bool) (*model.Usr, bool, error) {
	if s.get == nil {
		panic("unexpected call")
	}
	item, err := s.get(ctx, id)
	return item, err == nil, err
}

func (s *testUsrService) PasswordMatches(item *model.Usr, password string) (bool, error) {
	if s.passwordMatches == nil {
		panic("unexpected call")
	}
	return s.passwordMatches(item, password)
}

func (s *testUsrService) AuthByUsernamePassword(_ context.Context, _, _ string) (*model.Usr, bool, error) {
	panic("unexpected call")
}

func (s *testUsrService) HasAny(ctx context.Context) (bool, error) {
	return s.hasAny(ctx)
}

func (s *testUsrService) Create(ctx context.Context, obj *model.Edit) (int64, error) {
	return s.create(ctx, obj)
}

func (s *testUsrService) Update(ctx context.Context, id int64, obj *model.Edit) error {
	return s.update(ctx, id, obj)
}

func (s *testUsrService) Delete(_ context.Context, _ int64) error {
	panic("unexpected call")
}

func requireUsernameExists(t *testing.T, err error) {
	t.Helper()

	fullErr, ok := errors.AsType[errs.ErrFull](err)
	require.True(t, ok, "expected errs.ErrFull, got %v", err)
	assert.Equal(t, errs.UsernameExists, fullErr.Err)
	assert.Contains(t, fullErr.Fields, "username")
}

func TestUsecase_Create_UsernameExists(t *testing.T) {
	uc := New(
		&testUsrService{
			create: func(_ context.Context, _ *model.Edit) (int64, error) {
				return 0, fmt.Errorf("repoDb.Create: %w", errs.UsernameExists)
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1, Admin: true}},
	)

	_, err := uc.Create(context.Background(), &model.Edit{
		Name:     new("Jane"),
		Username: new("jane"),
		Password: new("secret"),
	})
	requireUsernameExists(t, err)
}

func TestUsecase_Create_FirstAdmin_UsernameExists(t *testing.T) {
	uc := New(
		&testUsrService{
			hasAny: func(_ context.Context) (bool, error) { return false, nil },
			create: func(_ context.Context, obj *model.Edit) (int64, error) {
				require.NotNil(t, obj.IsAdmin)
				require.True(t, *obj.IsAdmin)
				return 0, fmt.Errorf("repoDb.Create: %w", errs.UsernameExists)
			},
		},
		&testSessionService{},
	)

	_, err := uc.Create(context.Background(), &model.Edit{
		Name:     new("Admin"),
		Username: new("admin"),
		Password: new("secret"),
	})
	requireUsernameExists(t, err)
}

func TestUsecase_Update_UsernameExists(t *testing.T) {
	uc := New(
		&testUsrService{
			update: func(_ context.Context, id int64, _ *model.Edit) error {
				require.Equal(t, int64(7), id)
				return fmt.Errorf("repoDb.Update: %w", errs.UsernameExists)
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1, Admin: true}},
	)

	err := uc.Update(context.Background(), 7, &model.Edit{Username: new("jane")})
	requireUsernameExists(t, err)
}

func TestUsecase_Create_OtherErrorIsWrapped(t *testing.T) {
	boom := errors.New("boom")
	uc := New(
		&testUsrService{
			create: func(_ context.Context, _ *model.Edit) (int64, error) { return 0, boom },
		},
		&testSessionService{session: &sessionModel.Session{Id: 1, Admin: true}},
	)

	_, err := uc.Create(context.Background(), &model.Edit{
		Name:     new("Jane"),
		Username: new("jane"),
		Password: new("secret"),
	})
	require.ErrorIs(t, err, boom)
	_, isFull := errors.AsType[errs.ErrFull](err)
	assert.False(t, isFull)
}

func newProfileUsecase(updated **model.Edit) *Usecase {
	usrSvc := &testUsrService{
		get: func(_ context.Context, id int64) (*model.Usr, error) {
			return &model.Usr{Id: id, Active: true, Password: "hash-of-old-pass"}, nil
		},
		passwordMatches: func(item *model.Usr, password string) (bool, error) {
			return item.Password == "hash-of-"+password, nil
		},
		update: func(_ context.Context, _ int64, obj *model.Edit) error {
			*updated = obj
			return nil
		},
	}
	return New(usrSvc, &testSessionService{session: &sessionModel.Session{Id: 7}})
}

func TestUsecase_UpdateProfile_PasswordNeedsCurrent(t *testing.T) {
	cases := []struct {
		name    string
		current *string
		wantErr error
	}{
		{name: "missing", current: nil, wantErr: errs.PasswordRequired},
		{name: "empty", current: new(""), wantErr: errs.PasswordRequired},
		{name: "wrong", current: new("guess"), wantErr: errs.PasswordMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var updated *model.Edit
			uc := newProfileUsecase(&updated)

			err := uc.UpdateProfile(context.Background(), &UpdateProfileReq{
				Password:        new("new-pass"),
				CurrentPassword: tc.current,
			})

			fullErr, ok := errors.AsType[errs.ErrFull](err)
			require.True(t, ok, "expected errs.ErrFull, got %v", err)
			assert.Equal(t, tc.wantErr, fullErr.Err)
			assert.Contains(t, fullErr.Fields, "current_password")
			assert.Nil(t, updated, "password must not be changed")
		})
	}
}

func TestUsecase_UpdateProfile_PasswordWithCurrent(t *testing.T) {
	var updated *model.Edit
	uc := newProfileUsecase(&updated)

	err := uc.UpdateProfile(context.Background(), &UpdateProfileReq{
		Password:        new("new-pass"),
		CurrentPassword: new("old-pass"),
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, new("new-pass"), updated.Password)
}

func TestUsecase_UpdateProfile_NameOnlyNeedsNoPassword(t *testing.T) {
	var updated *model.Edit
	uc := newProfileUsecase(&updated)

	err := uc.UpdateProfile(context.Background(), &UpdateProfileReq{Name: new("New Name")})

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, new("New Name"), updated.Name)
	assert.Nil(t, updated.Password)
}
