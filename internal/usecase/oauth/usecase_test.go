package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/domain/oauth/model"
	oauthService "github.com/rendau/ruto/internal/domain/oauth/service"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	usrModel "github.com/rendau/ruto/internal/domain/usr/model"
	"github.com/rendau/ruto/internal/errs"
)

const (
	testIssuer   = "https://mcp.example.com"
	testRedirect = "https://claude.ai/api/mcp/auth_callback"
	testVerifier = "verifier-verifier-verifier-verifier-verifier"
)

// memRepo is an in-memory oauth repo for running the real domain service.
type memRepo struct {
	mu     sync.Mutex
	grants map[string]*model.Grant
	tokens map[string]*model.Token
	seq    int
}

func newMemRepo() *memRepo {
	return &memRepo{grants: map[string]*model.Grant{}, tokens: map[string]*model.Token{}}
}

func (r *memRepo) ListGrants(_ context.Context, pars *model.GrantListReq) ([]*model.Grant, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]*model.Grant, 0)
	for _, g := range r.grants {
		if pars.UsrId != nil && g.UsrId != *pars.UsrId {
			continue
		}
		if pars.Connected != nil && (g.ConnectedAt != nil) != *pars.Connected {
			continue
		}
		result = append(result, new(*g))
	}
	return result, int64(len(result)), nil
}

func (r *memRepo) GetGrant(_ context.Context, id string) (*model.Grant, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.grants[id]
	if !ok {
		return nil, false, nil
	}
	return new(*g), true, nil
}

func (r *memRepo) CreateGrant(_ context.Context, obj *model.Grant) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	g := new(*obj)
	g.Id = "grant-" + string(rune('0'+r.seq))
	g.CreatedAt = time.Now()
	r.grants[g.Id] = g
	return g.Id, nil
}

func (r *memRepo) DeleteGrant(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.grants, id)
	for hash, t := range r.tokens {
		if t.GrantId == id {
			delete(r.tokens, hash)
		}
	}
	return nil
}

func (r *memRepo) MarkGrantConnected(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.grants[id]; ok && g.ConnectedAt == nil {
		g.ConnectedAt = new(time.Now())
	}
	return nil
}

func (r *memRepo) TouchGrantLastUsed(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.grants[id]; ok {
		g.LastUsedAt = new(time.Now())
	}
	return nil
}

func (r *memRepo) CreateTokens(_ context.Context, tokens []*model.Token) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range tokens {
		r.tokens[t.Hash] = new(*t)
	}
	return nil
}

func (r *memRepo) GetToken(_ context.Context, hash string) (*model.Token, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tokens[hash]
	if !ok {
		return nil, false, nil
	}
	return new(*t), true, nil
}

func (r *memRepo) TakeToken(_ context.Context, hash, kind string) (*model.Token, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tokens[hash]
	if !ok || t.Kind != kind {
		return nil, false, nil
	}
	delete(r.tokens, hash)
	return t, true, nil
}

func (r *memRepo) Cleanup(_ context.Context, _ time.Time) (int64, int64, error) {
	return 0, 0, nil
}

type testUsrService struct {
	usrs map[int64]*usrModel.Usr
}

func (s *testUsrService) Get(_ context.Context, id int64, _ bool) (*usrModel.Usr, bool, error) {
	u, ok := s.usrs[id]
	return u, ok, nil
}

type ctxSessionKey struct{}

type testSessionService struct{}

func (s *testSessionService) FromContext(ctx context.Context) *sessionModel.Session {
	if v, ok := ctx.Value(ctxSessionKey{}).(*sessionModel.Session); ok {
		return v
	}
	return sessionModel.New(0)
}

type testTxm struct{}

func (t *testTxm) TxFn(ctx context.Context, f func(context.Context) error) error {
	return f(ctx)
}

type testEnv struct {
	uc    *Usecase
	repo  *memRepo
	usrs  *testUsrService
	admin context.Context
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	repo := newMemRepo()
	usrs := &testUsrService{usrs: map[int64]*usrModel.Usr{
		1: {Id: 1, Active: true, IsAdmin: true, AllApps: true},
		2: {Id: 2, Active: true},
	}}
	uc := New(
		Config{
			Enabled:      true,
			Issuer:       testIssuer,
			AdminURL:     "https://ruto.example.com",
			RedirectUris: []string{testRedirect, "http://localhost"},
			AccessTTL:    time.Hour,
			RefreshTTL:   24 * time.Hour,
		},
		oauthService.New(repo),
		oauthService.NewClientCodec("secret"),
		usrs,
		&testSessionService{},
		&testTxm{},
	)
	admin := context.WithValue(context.Background(), ctxSessionKey{}, &sessionModel.Session{Id: 1, Admin: true})
	return &testEnv{uc: uc, repo: repo, usrs: usrs, admin: admin}
}

func assertErrCode(t *testing.T, err error, code errs.Err) {
	t.Helper()
	full, ok := errors.AsType[errs.ErrFull](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, code, full.Err)
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (e *testEnv) register(t *testing.T, authMethod string) *RegisterRep {
	t.Helper()
	rep, err := e.uc.Register(context.Background(), &RegisterReq{
		RedirectUris:            []string{testRedirect},
		ClientName:              "Claude",
		TokenEndpointAuthMethod: authMethod,
	})
	require.NoError(t, err)
	return rep
}

func (e *testEnv) authorizeReq(clientId string) *AuthorizeReq {
	return &AuthorizeReq{
		ResponseType:        "code",
		ClientId:            clientId,
		RedirectUri:         testRedirect,
		State:               "st",
		CodeChallenge:       pkceChallenge(testVerifier),
		CodeChallengeMethod: "S256",
		Resource:            testIssuer + "/",
	}
}

// approve runs the consent page and returns the code from the redirect.
func (e *testEnv) approve(t *testing.T, clientId string) string {
	t.Helper()
	redirect, err := e.uc.Authorize(e.admin, e.authorizeReq(clientId), true)
	require.NoError(t, err)
	parsed, err := url.Parse(redirect)
	require.NoError(t, err)
	assert.Equal(t, "st", parsed.Query().Get("state"))
	code := parsed.Query().Get("code")
	require.NotEmpty(t, code)
	return code
}

func TestRegister(t *testing.T) {
	env := newTestEnv(t)

	t.Run("public client", func(t *testing.T) {
		rep := env.register(t, "none")
		assert.Empty(t, rep.ClientSecret)
		assert.Equal(t, "Claude", rep.Client.Name)
	})

	t.Run("confidential by default", func(t *testing.T) {
		rep := env.register(t, "")
		assert.Equal(t, model.ClientAuthSecretBasic, rep.Client.AuthMethod)
		assert.NotEmpty(t, rep.ClientSecret)
	})

	t.Run("foreign redirect", func(t *testing.T) {
		_, err := env.uc.Register(context.Background(), &RegisterReq{
			RedirectUris: []string{"https://evil.example/cb"},
		})
		oauthErr, ok := err.(*Error)
		require.True(t, ok)
		assert.Equal(t, ErrInvalidRedirectUri, oauthErr.Code)
	})

	t.Run("loopback any port", func(t *testing.T) {
		_, err := env.uc.Register(context.Background(), &RegisterReq{
			RedirectUris:            []string{"http://localhost:53682/callback"},
			TokenEndpointAuthMethod: "none",
		})
		require.NoError(t, err)
	})
}

func TestAuthorize(t *testing.T) {
	env := newTestEnv(t)
	client := env.register(t, "none")

	t.Run("describe", func(t *testing.T) {
		info, err := env.uc.DescribeAuthorization(env.admin, env.authorizeReq(client.ClientId))
		require.NoError(t, err)
		assert.Equal(t, "Claude", info.ClientName)
		assert.Equal(t, "claude.ai", info.RedirectHost)
	})

	t.Run("not an admin", func(t *testing.T) {
		usr := context.WithValue(context.Background(), ctxSessionKey{}, &sessionModel.Session{Id: 2})
		_, err := env.uc.DescribeAuthorization(usr, env.authorizeReq(client.ClientId))
		assert.ErrorIs(t, err, errs.NoPermission)
	})

	t.Run("agent session", func(t *testing.T) {
		agent := context.WithValue(context.Background(), ctxSessionKey{}, &sessionModel.Session{Id: 1, Admin: true, ApiKeyId: "k"})
		_, err := env.uc.Authorize(agent, env.authorizeReq(client.ClientId), true)
		assert.ErrorIs(t, err, errs.NoPermission)
	})

	t.Run("redirect not registered", func(t *testing.T) {
		req := env.authorizeReq(client.ClientId)
		req.RedirectUri = "http://localhost:1234/cb"
		_, err := env.uc.DescribeAuthorization(env.admin, req)
		assertErrCode(t, err, errs.OAuthInvalidRequest)
	})

	t.Run("forged client", func(t *testing.T) {
		_, err := env.uc.DescribeAuthorization(env.admin, env.authorizeReq("rtcl_forged.sig"))
		assertErrCode(t, err, errs.OAuthInvalidRequest)
	})

	t.Run("pkce required", func(t *testing.T) {
		req := env.authorizeReq(client.ClientId)
		req.CodeChallengeMethod = "plain"
		_, err := env.uc.DescribeAuthorization(env.admin, req)
		assertErrCode(t, err, errs.OAuthInvalidRequest)
	})

	t.Run("other resource", func(t *testing.T) {
		req := env.authorizeReq(client.ClientId)
		req.Resource = "https://other.example.com"
		_, err := env.uc.DescribeAuthorization(env.admin, req)
		assertErrCode(t, err, errs.OAuthInvalidRequest)
	})

	t.Run("deny", func(t *testing.T) {
		redirect, err := env.uc.Authorize(env.admin, env.authorizeReq(client.ClientId), false)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(redirect, testRedirect+"?"))
		assert.Contains(t, redirect, "error=access_denied")
		assert.Contains(t, redirect, "state=st")
	})
}

func TestTokenFlow(t *testing.T) {
	env := newTestEnv(t)
	client := env.register(t, "none")
	code := env.approve(t, client.ClientId)

	exchange := func(code, verifier string) (*TokenRep, error) {
		return env.uc.Token(context.Background(), &TokenReq{
			GrantType:    "authorization_code",
			Code:         code,
			RedirectUri:  testRedirect,
			CodeVerifier: verifier,
			ClientId:     client.ClientId,
			Resource:     testIssuer,
		})
	}

	tokens, err := exchange(code, testVerifier)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(tokens.AccessToken, oauthService.AccessTokenPrefix))
	assert.Equal(t, int64(3600), tokens.ExpiresIn)

	t.Run("code is single-use", func(t *testing.T) {
		_, err := exchange(code, testVerifier)
		oauthErr, ok := err.(*Error)
		require.True(t, ok)
		assert.Equal(t, ErrInvalidGrant, oauthErr.Code)
	})

	t.Run("wrong verifier", func(t *testing.T) {
		_, err := exchange(env.approve(t, client.ClientId), strings.Repeat("x", 43))
		oauthErr, ok := err.(*Error)
		require.True(t, ok)
		assert.Equal(t, ErrInvalidGrant, oauthErr.Code)
	})

	session, err := env.uc.SessionFromToken(context.Background(), tokens.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, int64(1), session.Id)
	assert.True(t, session.IsAdmin())
	assert.False(t, session.IsHuman())
	assert.Equal(t, "Claude (OAuth)", session.ApiKeyName)
	grantId := session.ApiKeyId

	grants, err := env.uc.ListGrants(env.admin)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, grantId, grants[0].Id)

	t.Run("refresh rotates", func(t *testing.T) {
		refreshed, err := env.uc.Token(context.Background(), &TokenReq{
			GrantType:    "refresh_token",
			RefreshToken: tokens.RefreshToken,
			ClientId:     client.ClientId,
		})
		require.NoError(t, err)

		session, err := env.uc.SessionFromToken(context.Background(), refreshed.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, grantId, session.ApiKeyId)

		_, err = env.uc.Token(context.Background(), &TokenReq{
			GrantType:    "refresh_token",
			RefreshToken: tokens.RefreshToken,
			ClientId:     client.ClientId,
		})
		oauthErr, ok := err.(*Error)
		require.True(t, ok)
		assert.Equal(t, ErrInvalidGrant, oauthErr.Code)

		tokens = refreshed
	})

	t.Run("refresh by another client", func(t *testing.T) {
		other := env.register(t, "none")
		_, err := env.uc.Token(context.Background(), &TokenReq{
			GrantType:    "refresh_token",
			RefreshToken: tokens.RefreshToken,
			ClientId:     other.ClientId,
		})
		oauthErr, ok := err.(*Error)
		require.True(t, ok)
		assert.Equal(t, ErrInvalidGrant, oauthErr.Code)
	})

	t.Run("admin rights revoked", func(t *testing.T) {
		env.usrs.usrs[1].IsAdmin = false
		defer func() { env.usrs.usrs[1].IsAdmin = true }()

		_, err := env.uc.SessionFromToken(context.Background(), tokens.AccessToken)
		assert.ErrorIs(t, err, errs.NotAuthorized)
	})

	t.Run("disconnect", func(t *testing.T) {
		require.NoError(t, env.uc.DeleteGrant(env.admin, grantId))

		_, err := env.uc.SessionFromToken(context.Background(), tokens.AccessToken)
		assert.ErrorIs(t, err, errs.NotAuthorized)
	})
}

func TestConfidentialClient(t *testing.T) {
	env := newTestEnv(t)
	client := env.register(t, model.ClientAuthSecretPost)
	code := env.approve(t, client.ClientId)

	req := &TokenReq{
		GrantType:    "authorization_code",
		Code:         code,
		RedirectUri:  testRedirect,
		CodeVerifier: testVerifier,
		ClientId:     client.ClientId,
	}

	_, err := env.uc.Token(context.Background(), req)
	oauthErr, ok := err.(*Error)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidClient, oauthErr.Code)

	req.ClientSecret = client.ClientSecret
	_, err = env.uc.Token(context.Background(), req)
	require.NoError(t, err)
}

func TestRevoke(t *testing.T) {
	env := newTestEnv(t)
	client := env.register(t, "none")
	tokens, err := env.uc.Token(context.Background(), &TokenReq{
		GrantType:    "authorization_code",
		Code:         env.approve(t, client.ClientId),
		RedirectUri:  testRedirect,
		CodeVerifier: testVerifier,
		ClientId:     client.ClientId,
	})
	require.NoError(t, err)

	require.NoError(t, env.uc.Revoke(context.Background(), "rtr_unknown", client.ClientId, ""))
	require.NoError(t, env.uc.Revoke(context.Background(), tokens.RefreshToken, client.ClientId, ""))

	_, err = env.uc.SessionFromToken(context.Background(), tokens.AccessToken)
	assert.ErrorIs(t, err, errs.NotAuthorized)
}

func TestDisabled(t *testing.T) {
	env := newTestEnv(t)
	env.uc.conf.Enabled = false

	_, err := env.uc.Register(context.Background(), &RegisterReq{RedirectUris: []string{testRedirect}})
	assert.Equal(t, errNotEnabled, err)

	_, err = env.uc.SessionFromToken(context.Background(), "rto_x")
	assert.ErrorIs(t, err, errs.NotAuthorized)
}
