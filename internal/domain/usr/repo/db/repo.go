package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendau/mobone/v2"
	moboneTools "github.com/rendau/mobone/v2/tools"
	"github.com/samber/lo"

	commonRepoPg "github.com/rendau/ruto/internal/domain/common/repo/pg"
	domainModel "github.com/rendau/ruto/internal/domain/usr/model"
	repoModel "github.com/rendau/ruto/internal/domain/usr/repo/db/model"
)

type Repo struct {
	*commonRepoPg.Base
	ModelStore *mobone.ModelStore
}

func New(con *pgxpool.Pool) *Repo {
	base := commonRepoPg.NewBase(con)
	return &Repo{
		Base: base,
		ModelStore: &mobone.ModelStore{
			Con:       base.Con,
			QB:        base.QB,
			TableName: "usr",
		},
	}
}

func (r *Repo) List(ctx context.Context, pars *domainModel.ListReq) ([]*domainModel.Usr, int64, error) {
	if pars == nil {
		pars = &domainModel.ListReq{}
	}

	conditions, conditionExps := r.getConditions(pars)
	sort := moboneTools.ConstructSortColumns(allowedSortFields, pars.Sort)

	items := make([]*repoModel.Select, 0)

	totalCount, err := r.ModelStore.List(ctx, mobone.ListParams{
		Conditions:           conditions,
		ConditionExpressions: conditionExps,
		Page:                 pars.Page,
		PageSize:             pars.PageSize,
		WithTotalCount:       pars.WithTotalCount,
		OnlyCount:            pars.OnlyCount,
		Sort:                 sort,
	}, func(add bool) mobone.ListModelI {
		item := &repoModel.Select{}
		if add {
			items = append(items, item)
		}
		return item
	})
	if err != nil {
		return nil, 0, fmt.Errorf("ModelStore.List: %w", err)
	}

	return lo.Map(items, repoModel.EncodeSelect), totalCount, nil
}

func (r *Repo) Get(ctx context.Context, id int64) (*domainModel.Usr, bool, error) {
	m := &repoModel.Select{PKId: id}

	found, err := r.ModelStore.Get(ctx, m)
	if err != nil {
		return nil, false, fmt.Errorf("ModelStore.Get: %w", err)
	}
	if !found {
		return nil, false, nil
	}

	return repoModel.EncodeSelect(m, 0), true, nil
}

func (r *Repo) GetByUsernamePassword(ctx context.Context, username, password string) (*domainModel.Usr, bool, error) {
	m := &repoModel.GetByUsernameAndPassword{}
	m.Username = username
	m.Password = password

	found, err := r.ModelStore.Get(ctx, m)
	if err != nil {
		return nil, false, fmt.Errorf("ModelStore.Get: %w", err)
	}
	if !found {
		return nil, false, nil
	}

	return repoModel.EncodeSelect(&m.Select, 0), true, nil
}

func (r *Repo) Create(ctx context.Context, obj *domainModel.Usr) (int64, error) {
	m := repoModel.DecodeUpsert(obj)

	err := r.ModelStore.Create(ctx, m)
	if err != nil {
		return 0, fmt.Errorf("ModelStore.Create: %w", err)
	}

	return m.PKId, nil
}

func (r *Repo) Update(ctx context.Context, id int64, obj *domainModel.Usr) error {
	m := repoModel.DecodeUpsert(obj)
	m.PKId = id

	err := r.ModelStore.Update(ctx, m)
	if err != nil {
		return fmt.Errorf("ModelStore.Update: %w", err)
	}

	return nil
}

func (r *Repo) Delete(ctx context.Context, id int64) error {
	m := &repoModel.Upsert{PKId: id}

	err := r.ModelStore.Delete(ctx, m)
	if err != nil {
		return fmt.Errorf("ModelStore.Delete: %w", err)
	}

	return nil
}
