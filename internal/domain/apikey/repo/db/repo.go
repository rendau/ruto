package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendau/mobone/v2"
	moboneTools "github.com/rendau/mobone/v2/tools"
	"github.com/samber/lo"

	domainModel "github.com/rendau/ruto/internal/domain/apikey/model"
	repoModel "github.com/rendau/ruto/internal/domain/apikey/repo/db/model"
	commonRepoPg "github.com/rendau/ruto/internal/domain/common/repo/pg"
)

var allowedSortFields = map[string]string{
	"created_at": "created_at",
	"name":       "name",
}

type Repo struct {
	*commonRepoPg.Base
	ModelStore *mobone.ModelStore
}

func New(con *pgxpool.Pool) *Repo {
	base := commonRepoPg.NewBase(con)
	return &Repo{
		Base: base,
		ModelStore: &mobone.ModelStore{
			Con:                base.Con,
			TransactionManager: base.Txm,
			QB:                 base.QB,
			TableName:          "api_key",
		},
	}
}

func (r *Repo) List(ctx context.Context, pars *domainModel.ListReq) ([]*domainModel.ApiKey, int64, error) {
	conditions := map[string]any{}
	if pars.UsrId != nil {
		conditions["usr_id"] = *pars.UsrId
	}
	if pars.KeyHash != nil {
		conditions["key_hash"] = *pars.KeyHash
	}

	items := make([]*repoModel.Select, 0)

	totalCount, err := r.ModelStore.List(ctx, mobone.ListParams{
		Conditions:     conditions,
		Page:           pars.Page,
		PageSize:       pars.PageSize,
		WithTotalCount: pars.WithTotalCount,
		OnlyCount:      pars.OnlyCount,
		Sort:           moboneTools.ConstructSortColumns(allowedSortFields, pars.Sort),
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

func (r *Repo) Get(ctx context.Context, id string) (*domainModel.ApiKey, bool, error) {
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

func (r *Repo) Create(ctx context.Context, obj *domainModel.ApiKey) (string, error) {
	m := repoModel.DecodeUpsert(obj)

	if err := r.ModelStore.Create(ctx, m); err != nil {
		return "", fmt.Errorf("ModelStore.Create: %w", err)
	}

	return m.PKId, nil
}

func (r *Repo) Delete(ctx context.Context, id string) error {
	if err := r.ModelStore.Delete(ctx, &repoModel.Upsert{PKId: id}); err != nil {
		return fmt.Errorf("ModelStore.Delete: %w", err)
	}

	return nil
}

func (r *Repo) TouchLastUsed(ctx context.Context, id string) error {
	_, err := r.ModelStore.GetConnection(ctx).Exec(ctx, "UPDATE api_key SET last_used_at = now() WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("Exec: %w", err)
	}

	return nil
}
