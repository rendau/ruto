package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendau/mobone/v2"

	commonRepoPg "github.com/rendau/ruto/internal/domain/common/repo/pg"
	domModel "github.com/rendau/ruto/internal/domain/root/model"
	repoModel "github.com/rendau/ruto/internal/domain/root/repo/db/model"
)

const rootId = "root"

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
			TableName: "root",
		},
	}
}

func (r *Repo) Get(ctx context.Context) (*domModel.Main, error) {
	m := &repoModel.Select{PKId: rootId}
	found, err := r.ModelStore.Get(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("ModelStore.Get: %w", err)
	}
	if !found {
		return nil, nil
	}
	return repoModel.EncodeSelect(m, 0), nil
}

func (r *Repo) Set(ctx context.Context, obj *domModel.Edit) error {
	m := repoModel.DecodeUpsert(obj)
	m.PKId = rootId
	if err := r.ModelStore.UpdateOrCreate(ctx, m); err != nil {
		return fmt.Errorf("ModelStore.UpdateOrCreate: %w", err)
	}
	return nil
}
