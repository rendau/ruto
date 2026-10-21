// Package oauth is the OAuth 2.1 authorization server of the built-in MCP
// server, so that MCP clients with connectors (Claude apps) connect by URL:
// the user signs in to the admin UI and approves the client, the client gets
// tokens that act like an api key of that admin.
package oauth

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	"github.com/rendau/ruto/internal/domain/oauth/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	"github.com/rendau/ruto/internal/errs"
)

const (
	// Scope is the only scope: access to the MCP server.
	Scope = "mcp"

	codeTTL = 2 * time.Minute

	// pendingGrantTTL: a grant without tokens older than this is an abandoned
	// consent or an expired connection.
	pendingGrantTTL = time.Hour

	// touchInterval limits last_used_at writes: an MCP client calls on every tool.
	touchInterval = time.Minute
)

type Config struct {
	Enabled bool
	// Issuer is the public URL of the MCP server: the OAuth issuer and the
	// protected resource at once.
	Issuer string
	// AdminURL is the public URL of the admin UI, which shows the consent page.
	AdminURL     string
	RedirectUris []string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
}

type Usecase struct {
	conf        Config
	svc         ServiceI
	clientCodec ClientCodecI
	usrSvc      UsrServiceI
	sessionSvc  SessionServiceI
	txm         TransactionManagerI

	touchMu   sync.Mutex
	lastTouch map[string]time.Time
}

func New(
	conf Config,
	svc ServiceI,
	clientCodec ClientCodecI,
	usrSvc UsrServiceI,
	sessionSvc SessionServiceI,
	txm TransactionManagerI,
) *Usecase {
	return &Usecase{
		conf:        conf,
		svc:         svc,
		clientCodec: clientCodec,
		usrSvc:      usrSvc,
		sessionSvc:  sessionSvc,
		txm:         txm,
		lastTouch:   map[string]time.Time{},
	}
}

func (u *Usecase) Config() Config {
	return u.conf
}

// ListGrants returns the caller's connected clients.
func (u *Usecase) ListGrants(ctx context.Context) ([]*model.Grant, error) {
	session, err := u.requireHumanAdmin(ctx)
	if err != nil {
		return nil, err
	}

	items, _, err := u.svc.ListGrants(ctx, &model.GrantListReq{
		ListParams: commonModel.ListParams{Sort: []string{"created_at"}},
		UsrId:      &session.Id,
		Connected:  new(true),
	})
	if err != nil {
		return nil, fmt.Errorf("svc.ListGrants: %w", err)
	}

	return items, nil
}

// DeleteGrant disconnects a client of the caller: its tokens stop working at
// once.
func (u *Usecase) DeleteGrant(ctx context.Context, id string) error {
	session, err := u.requireHumanAdmin(ctx)
	if err != nil {
		return err
	}
	if id == "" {
		return errs.IdRequired
	}

	item, _, err := u.svc.GetGrant(ctx, id, true)
	if err != nil {
		return fmt.Errorf("svc.GetGrant: %w", err)
	}
	if item.UsrId != session.Id {
		return errs.NoPermission
	}

	if err = u.svc.DeleteGrant(ctx, id); err != nil {
		return fmt.Errorf("svc.DeleteGrant: %w", err)
	}

	return nil
}

// SessionFromToken authenticates an MCP request by an access token. As with
// api keys, the session is built from the live user record, and the grant
// plays the role of the key (change requests are bound to it).
func (u *Usecase) SessionFromToken(ctx context.Context, token string) (*sessionModel.Session, error) {
	if !u.conf.Enabled {
		return nil, errs.NotAuthorized
	}

	item, found, err := u.svc.FindToken(ctx, token, model.TokenKindAccess)
	if err != nil {
		return nil, fmt.Errorf("svc.FindToken: %w", err)
	}
	if !found {
		return nil, errs.NotAuthorized
	}

	grant, found, err := u.svc.GetGrant(ctx, item.GrantId, false)
	if err != nil {
		return nil, fmt.Errorf("svc.GetGrant: %w", err)
	}
	if !found {
		return nil, errs.NotAuthorized
	}

	session, err := u.grantSession(ctx, grant)
	if err != nil {
		return nil, err
	}

	u.touchLastUsed(ctx, grant.Id)

	return session, nil
}

// RunCleanup calls Cleanup now and then every interval until ctx is done.
func (u *Usecase) RunCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		u.Cleanup(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Cleanup deletes expired tokens and grants left without tokens.
func (u *Usecase) Cleanup(ctx context.Context) {
	tokens, grants, err := u.svc.Cleanup(ctx, time.Now().Add(-pendingGrantTTL))
	if err != nil {
		slog.Error("oauth: cleanup", "error", err)
		return
	}
	if grants > 0 {
		slog.Info("oauth: cleanup", "tokens", tokens, "grants", grants)
	}
}

// grantSession is the session of the grant's owner; only an active admin may
// use MCP.
func (u *Usecase) grantSession(ctx context.Context, grant *model.Grant) (*sessionModel.Session, error) {
	usr, found, err := u.usrSvc.Get(ctx, grant.UsrId, false)
	if err != nil {
		return nil, fmt.Errorf("usrSvc.Get: %w", err)
	}
	if !found || !usr.Active || !usr.IsAdmin {
		return nil, errs.NotAuthorized
	}

	return &sessionModel.Session{
		Id:         usr.Id,
		Admin:      usr.IsAdmin,
		AllApps:    usr.AllApps,
		AppIds:     usr.AppIds,
		ApiKeyId:   grant.Id,
		ApiKeyName: grantDisplayName(grant),
	}, nil
}

func grantDisplayName(grant *model.Grant) string {
	name := grant.ClientName
	if name == "" {
		name = "OAuth client"
	}
	return name + " (OAuth)"
}

func (u *Usecase) requireHumanAdmin(ctx context.Context) (*sessionModel.Session, error) {
	session := u.sessionSvc.FromContext(ctx)
	if !session.IsAuthorized() {
		return nil, errs.NotAuthorized
	}
	if !session.IsHuman() || !session.IsAdmin() {
		return nil, errs.NoPermission
	}
	return session, nil
}

func (u *Usecase) touchLastUsed(ctx context.Context, id string) {
	u.touchMu.Lock()
	if last, ok := u.lastTouch[id]; ok && time.Since(last) < touchInterval {
		u.touchMu.Unlock()
		return
	}
	u.lastTouch[id] = time.Now()
	u.touchMu.Unlock()

	if err := u.svc.TouchGrantLastUsed(ctx, id); err != nil {
		slog.Warn("oauth: touch last_used_at", "error", err, "id", id)
	}
}
