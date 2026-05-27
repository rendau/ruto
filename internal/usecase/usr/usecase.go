package usr

import (
	"context"
	"fmt"
	"strings"

	"github.com/rendau/ruto/internal/domain/usr/model"
	"github.com/rendau/ruto/internal/errs"
)

type Usecase struct {
	svc        ServiceI
	sessionSvc SessionServiceI
}

func New(srv ServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        srv,
		sessionSvc: sessionSvc,
	}
}

func (u *Usecase) Login(ctx context.Context, username, password string) (string, error) {
	username = strings.TrimSpace(username)

	item, found, err := u.svc.AuthByUsernamePassword(ctx, username, password)
	if err != nil {
		return "", fmt.Errorf("svc.AuthByUsernamePassword: %w", err)
	}
	if !found || !item.Active {
		return "", errs.NotAuthorized
	}

	token, err := u.sessionSvc.CreateToken(item.Id, item.IsAdmin)
	if err != nil {
		return "", fmt.Errorf("sessionSvc.CreateToken: %w", err)
	}

	return token, nil
}

func (u *Usecase) BootstrapStatus(ctx context.Context) (bool, error) {
	hasAny, err := u.svc.HasAny(ctx)
	if err != nil {
		return false, fmt.Errorf("svc.HasAny: %w", err)
	}
	return !hasAny, nil
}

func (u *Usecase) GetProfile(ctx context.Context) (*model.Usr, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, errs.NotAuthorized
	}

	item, _, err := u.svc.Get(ctx, extractedSession.Id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	if !item.Active {
		return nil, errs.NotAuthorized
	}

	item.Password = ""

	return item, nil
}

func (u *Usecase) UpdateProfile(ctx context.Context, req *UpdateProfileReq) error {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return errs.NotAuthorized
	}
	if req == nil {
		return errs.InvalidRequest
	}

	item, _, err := u.svc.Get(ctx, extractedSession.Id, true)
	if err != nil {
		return fmt.Errorf("svc.Get: %w", err)
	}
	if !item.Active {
		return errs.NotAuthorized
	}
	if req.Name == nil && req.Password == nil {
		return errs.InvalidRequest
	}

	edit := &model.Edit{
		Name:     req.Name,
		Password: req.Password,
	}

	if err = u.validateEdit(edit, false); err != nil {
		return err
	}

	if err = u.svc.Update(ctx, extractedSession.Id, edit); err != nil {
		return fmt.Errorf("svc.Update: %w", err)
	}

	return nil
}

func (u *Usecase) List(ctx context.Context, pars *model.ListReq) ([]*model.Usr, int64, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, 0, errs.NotAuthorized
	}

	items, tCount, err := u.svc.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("svc.List: %w", err)
	}

	for i := range items {
		items[i].Password = ""
	}

	return items, tCount, nil
}

func (u *Usecase) Get(ctx context.Context, id int64) (*model.Usr, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, errs.NotAuthorized
	}

	if id == 0 {
		return nil, errs.IdRequired
	}

	item, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}

	item.Password = ""

	return item, nil
}

func (u *Usecase) Create(ctx context.Context, obj *model.Edit) (int64, error) {
	if obj == nil {
		obj = &model.Edit{}
	}

	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id > 0 && !extractedSession.IsAdmin {
		return 0, errs.NoPermission
	}
	if extractedSession.Id == 0 {
		hasAny, err := u.svc.HasAny(ctx)
		if err != nil {
			return 0, fmt.Errorf("svc.HasAny: %w", err)
		}
		if hasAny {
			return 0, errs.NotAuthorized
		}
		obj.IsAdmin = new(true)
		obj.Active = new(true)
	}

	if err := u.validateEdit(obj, true); err != nil {
		return 0, err
	}

	newId, err := u.svc.Create(ctx, obj)
	if err != nil {
		return 0, fmt.Errorf("svc.Create: %w", err)
	}

	return newId, nil
}

func (u *Usecase) Update(ctx context.Context, id int64, obj *model.Edit) error {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return errs.NotAuthorized
	}
	if !extractedSession.IsAdmin {
		return errs.NoPermission
	}

	if id == 0 {
		return errs.IdRequired
	}

	if err := u.validateEdit(obj, false); err != nil {
		return err
	}

	if err := u.svc.Update(ctx, id, obj); err != nil {
		return fmt.Errorf("svc.Update: %w", err)
	}

	return nil
}

func (u *Usecase) Delete(ctx context.Context, id int64) error {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return errs.NotAuthorized
	}
	if !extractedSession.IsAdmin {
		return errs.NoPermission
	}

	if id == 0 {
		return errs.IdRequired
	}

	if err := u.svc.Delete(ctx, id); err != nil {
		return fmt.Errorf("svc.Delete: %w", err)
	}
	return nil
}

func (u *Usecase) validateEdit(obj *model.Edit, forCreate bool) error {
	if obj == nil {
		return errs.InvalidRequest
	}

	// Name
	if forCreate && obj.Name == nil {
		return errs.NameRequired
	}
	if obj.Name != nil {
		*obj.Name = strings.TrimSpace(*obj.Name)
		if *obj.Name == "" {
			return errs.NameRequired
		}
	}

	// Username
	if forCreate && obj.Username == nil {
		return errs.UsernameRequired
	}
	if obj.Username != nil {
		*obj.Username = strings.TrimSpace(*obj.Username)
		if *obj.Username == "" {
			return errs.UsernameRequired
		}
	}

	// Password
	if forCreate && obj.Password == nil {
		return errs.PasswordRequired
	}
	if obj.Password != nil {
		*obj.Password = strings.TrimSpace(*obj.Password)
		if *obj.Password == "" {
			return errs.PasswordRequired
		}
	}
	return nil
}
