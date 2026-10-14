package apikey

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rendau/ruto/internal/domain/apikey/model"
	apikeyService "github.com/rendau/ruto/internal/domain/apikey/service"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	"github.com/rendau/ruto/internal/errs"
)

// touchInterval limits last_used_at writes: an MCP client calls on every tool.
const touchInterval = time.Minute

type Usecase struct {
	svc        ServiceI
	usrSvc     UsrServiceI
	sessionSvc SessionServiceI

	touchMu   sync.Mutex
	lastTouch map[string]time.Time
}

func New(svc ServiceI, usrSvc UsrServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        svc,
		usrSvc:     usrSvc,
		sessionSvc: sessionSvc,
		lastTouch:  map[string]time.Time{},
	}
}

// List returns the caller's own keys. Keys are managed only from an admin
// login: an agent must not be able to mint or list keys with its own key.
func (u *Usecase) List(ctx context.Context) ([]*model.ApiKey, error) {
	session, err := u.requireHumanAdmin(ctx)
	if err != nil {
		return nil, err
	}

	items, _, err := u.svc.List(ctx, &model.ListReq{UsrId: &session.Id})
	if err != nil {
		return nil, fmt.Errorf("svc.List: %w", err)
	}

	return items, nil
}

// Create issues a key for the caller and returns (id, key); the key is not
// retrievable afterwards.
func (u *Usecase) Create(ctx context.Context, name string) (string, string, error) {
	session, err := u.requireHumanAdmin(ctx)
	if err != nil {
		return "", "", err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", errs.NameRequired
	}

	key, hash, prefix, err := apikeyService.GenerateKey()
	if err != nil {
		return "", "", fmt.Errorf("GenerateKey: %w", err)
	}

	newId, err := u.svc.Create(ctx, &model.ApiKey{
		UsrId:     session.Id,
		Name:      name,
		KeyHash:   hash,
		KeyPrefix: prefix,
	})
	if err != nil {
		return "", "", fmt.Errorf("svc.Create: %w", err)
	}

	return newId, key, nil
}

func (u *Usecase) Delete(ctx context.Context, id string) error {
	session, err := u.requireHumanAdmin(ctx)
	if err != nil {
		return err
	}
	if id == "" {
		return errs.IdRequired
	}

	item, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return fmt.Errorf("svc.Get: %w", err)
	}
	if item.UsrId != session.Id {
		return errs.NoPermission
	}

	if err = u.svc.Delete(ctx, id); err != nil {
		return fmt.Errorf("svc.Delete: %w", err)
	}

	return nil
}

// SessionFromKey authenticates an MCP request. The session is built from the
// live user record, so revoking admin rights or deactivating the user disables
// the key at once.
func (u *Usecase) SessionFromKey(ctx context.Context, key string) (*sessionModel.Session, error) {
	item, found, err := u.svc.GetByKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("svc.GetByKey: %w", err)
	}
	if !found {
		return nil, errs.NotAuthorized
	}

	usr, found, err := u.usrSvc.Get(ctx, item.UsrId, false)
	if err != nil {
		return nil, fmt.Errorf("usrSvc.Get: %w", err)
	}
	if !found || !usr.Active || !usr.IsAdmin {
		return nil, errs.NotAuthorized
	}

	u.touchLastUsed(ctx, item.Id)

	return &sessionModel.Session{
		Id:         usr.Id,
		Admin:      usr.IsAdmin,
		AllApps:    usr.AllApps,
		AppIds:     usr.AppIds,
		ApiKeyId:   item.Id,
		ApiKeyName: item.Name,
	}, nil
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

	if err := u.svc.TouchLastUsed(ctx, id); err != nil {
		slog.Warn("apikey: touch last_used_at", "error", err, "id", id)
	}
}
