package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendau/mobone/v2"

	commonRepoPg "github.com/rendau/ruto/internal/domain/common/repo/pg"
	"github.com/rendau/ruto/internal/domain/snapshot/model"
	repoModel "github.com/rendau/ruto/internal/domain/snapshot/repo/db/model"
)

const snapshotId = "snapshot"

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
			TableName: "snapshot",
		},
	}
}

func (r *Repo) GetVersion(ctx context.Context) (string, error) {
	var result string

	err := r.Con.QueryRow(ctx, `
		select hash
		from `+r.ModelStore.TableName+`
		where id = $1
	`, snapshotId).Scan(&result)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("QueryRow.Scan: %w", err)
	}

	return result, nil
}

func (r *Repo) Get(ctx context.Context) (*model.Snapshot, error) {
	m := &repoModel.Select{PKId: snapshotId}
	found, err := r.ModelStore.Get(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("ModelStore.Get: %w", err)
	}
	if !found {
		return nil, nil
	}
	return repoModel.EncodeSelect(m, 0), nil
}

func (r *Repo) Set(ctx context.Context, obj *model.Snapshot) error {
	m := repoModel.DecodeUpsert(obj)
	m.PKId = snapshotId
	if err := r.ModelStore.UpdateOrCreate(ctx, m); err != nil {
		return fmt.Errorf("ModelStore.UpdateOrCreate: %w", err)
	}
	return nil
}
