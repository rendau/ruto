package usr

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	"github.com/rendau/ruto/internal/domain/usr/model"
	"github.com/rendau/ruto/internal/errs"
)

type testSessionService struct {
	session *sessionModel.Session
}

// test tokens are "<usr id>:<password version>"
func (s *testSessionService) CreateToken(usrId int64, passwordVersion string) (string, error) {
	return fmt.Sprintf("%d:%s", usrId, passwordVersion), nil
}

func (s *testSessionService) ParseToken(tokenStr string) (int64, string, error) {
	var usrId int64
	var passwordVersion string
	if _, err := fmt.Sscanf(tokenStr, "%d:%s", &usrId, &passwordVersion); err != nil {
		return 0, "", err
	}
	return usrId, passwordVersion, nil
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

type testUsrService struct {
	get             func(ctx context.Context, id int64) (*model.Usr, error)
	auth            func(ctx context.Context, username, password string) (*model.Usr, bool, error)
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

func (s *testUsrService) AuthByUsernamePassword(ctx context.Context, username, password string) (*model.Usr, bool, error) {
	if s.auth == nil {
		panic("unexpected call")
	}
	return s.auth(ctx, username, password)
}

func (s *testUsrService) PasswordVersion(item *model.Usr) string {
	return "v-" + item.Password
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

func newSessionUsecase(usr *model.Usr) *Usecase {
	usrSvc := &testUsrService{
		get: func(_ context.Context, id int64) (*model.Usr, error) {
			if usr == nil || usr.Id != id {
				return nil, errs.ObjectNotFound
			}
			return usr, nil
		},
	}
	return New(usrSvc, &testSessionService{})
}

func TestUsecase_SessionFromToken(t *testing.T) {
	usr := &model.Usr{Id: 7, Active: true, IsAdmin: true, AllApps: true, Password: "hash-1"}
	uc := newSessionUsecase(usr)

	session, err := uc.SessionFromToken(context.Background(), "7:v-hash-1")
	require.NoError(t, err)
	assert.Equal(t, int64(7), session.Id)
	assert.True(t, session.IsAdmin())
	assert.True(t, session.IsHuman())

	// rights come from the live record, not from the token
	usr.IsAdmin = false
	session, err = uc.SessionFromToken(context.Background(), "7:v-hash-1")
	require.NoError(t, err)
	assert.False(t, session.IsAdmin())

	// a new password ends the sessions issued under the old one
	usr.Password = "hash-2"
	_, err = uc.SessionFromToken(context.Background(), "7:v-hash-1")
	require.ErrorIs(t, err, errs.NotAuthorized)
	_, err = uc.SessionFromToken(context.Background(), "7:v-hash-2")
	require.NoError(t, err)

	usr.Active = false
	_, err = uc.SessionFromToken(context.Background(), "7:v-hash-2")
	require.ErrorIs(t, err, errs.NotAuthorized)

	_, err = uc.SessionFromToken(context.Background(), "garbage")
	require.ErrorIs(t, err, errs.NotAuthorized)
}

func TestUsecase_SessionFromToken_LookupFails(t *testing.T) {
	usrSvc := &testUsrService{
		get: func(_ context.Context, _ int64) (*model.Usr, error) { return nil, errors.New("db down") },
	}
	uc := New(usrSvc, &testSessionService{})

	_, err := uc.SessionFromToken(context.Background(), "7:v-hash-1")
	require.Error(t, err)
	assert.NotErrorIs(t, err, errs.NotAuthorized, "a failed lookup is not a bad token")
}

func TestUsecase_Login_Throttled(t *testing.T) {
	usr := &model.Usr{Id: 7, Active: true, Password: "hash-1"}
	authCalls := 0
	usrSvc := &testUsrService{
		auth: func(_ context.Context, username, password string) (*model.Usr, bool, error) {
			authCalls++
			if username == "admin" && password == "right" {
				return usr, true, nil
			}
			return nil, false, nil
		},
	}
	uc := New(usrSvc, &testSessionService{})
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	uc.throttle = newLoginThrottle(func() time.Time { return now })

	attacker := netip.MustParseAddr("203.0.113.9")
	owner := netip.MustParseAddr("198.51.100.1")

	for range throttlePairMax {
		_, err := uc.Login(context.Background(), "admin", "guess", attacker)
		require.ErrorIs(t, err, errs.NotAuthorized)
	}

	// locked for the attacker's address, the right password included, and
	// the password is not even checked
	authCalls = 0
	_, err := uc.Login(context.Background(), "Admin", "right", attacker)
	require.ErrorIs(t, err, errs.TooManyAttempts)
	assert.Zero(t, authCalls)

	// the owner signs in from elsewhere
	token, err := uc.Login(context.Background(), "admin", "right", owner)
	require.NoError(t, err)
	assert.Equal(t, "7:v-hash-1", token)

	now = now.Add(throttleLockout + time.Second)
	_, err = uc.Login(context.Background(), "admin", "right", attacker)
	require.NoError(t, err)
}

func TestLoginThrottle(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	th := newLoginThrottle(func() time.Time { return now })
	ip := netip.MustParseAddr("203.0.113.9")

	t.Run("one address guessing many usernames", func(t *testing.T) {
		for i := range throttleAddrMax {
			require.False(t, th.Blocked(fmt.Sprintf("user%d", i), ip))
			th.Failed(fmt.Sprintf("user%d", i), ip)
		}
		assert.True(t, th.Blocked("someone-else", ip))
		assert.False(t, th.Blocked("someone-else", netip.MustParseAddr("203.0.113.10")))
	})

	now = now.Add(throttleLockout + throttleWindow + time.Second)

	t.Run("failures spread over time do not add up", func(t *testing.T) {
		for range throttlePairMax * 3 {
			th.Failed("admin", ip)
			now = now.Add(throttleWindow/throttlePairMax + time.Minute)
		}
		assert.False(t, th.Blocked("admin", ip))
	})

	t.Run("success forgets the username's failures", func(t *testing.T) {
		other := netip.MustParseAddr("192.0.2.50")
		for range throttlePairMax - 1 {
			th.Failed("bob", other)
		}
		th.Succeeded("bob", other)
		th.Failed("bob", other)
		assert.False(t, th.Blocked("bob", other))
	})

	t.Run("sweep drops stale entries", func(t *testing.T) {
		now = now.Add(throttleLockout + throttleWindow + time.Second)
		th.Blocked("anyone", ip)
		assert.Empty(t, th.entries)
	})
}
