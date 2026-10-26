package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendau/mobone/v2"
	moboneTools "github.com/rendau/mobone/v2/tools"
	"github.com/samber/lo"

	domainModel "github.com/rendau/ruto/internal/domain/changerequest/model"
	repoModel "github.com/rendau/ruto/internal/domain/changerequest/repo/db/model"
	commonRepoPg "github.com/rendau/ruto/internal/domain/common/repo/pg"
)

var allowedSortFields = map[string]string{
	"id":         "id",
	"created_at": "created_at",
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
			TableName:          "change_request",
		},
	}
}

func (r *Repo) List(ctx context.Context, pars *domainModel.ListReq) ([]*domainModel.ChangeRequest, int64, error) {
	conditions := map[string]any{}
	if pars.Status != nil {
		conditions["status"] = *pars.Status
	}
	if pars.ApiKeyId != nil {
		conditions["api_key_id"] = *pars.ApiKeyId
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

func (r *Repo) Get(ctx context.Context, id int64) (*domainModel.ChangeRequest, bool, error) {
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

// Lock takes a row lock for the rest of the ctx transaction, so two admins
// cannot apply the same request concurrently.
func (r *Repo) Lock(ctx context.Context, id int64) error {
	_, err := r.ModelStore.GetConnection(ctx).Exec(ctx, "SELECT 1 FROM change_request WHERE id = $1 FOR UPDATE", id)
	if err != nil {
		return fmt.Errorf("Exec: %w", err)
	}
	return nil
}

func (r *Repo) Create(ctx context.Context, obj *domainModel.ChangeRequest) (int64, error) {
	m := repoModel.DecodeCreate(obj)

	if err := r.ModelStore.Create(ctx, m); err != nil {
		return 0, fmt.Errorf("ModelStore.Create: %w", err)
	}

	return m.PKId, nil
}

func (r *Repo) Update(ctx context.Context, id int64, obj *domainModel.Edit) error {
	if err := r.ModelStore.Update(ctx, &repoModel.Update{PKId: id, Edit: obj}); err != nil {
		return fmt.Errorf("ModelStore.Update: %w", err)
	}
	return nil
}

// ExpirePending closes pending requests created before the given time.
func (r *Repo) ExpirePending(ctx context.Context, createdBefore time.Time) (int64, error) {
	tag, err := r.ModelStore.GetConnection(ctx).Exec(
		ctx,
		"UPDATE change_request SET status = $1, decided_at = now() WHERE status = $2 AND created_at < $3",
		domainModel.StatusExpired, domainModel.StatusPending, createdBefore,
	)
	if err != nil {
		return 0, fmt.Errorf("Exec: %w", err)
	}
	return tag.RowsAffected(), nil
}

// PendingStats counts pending requests in one pass over the (status, created_at)
// index; a pending request keeps its status since creation.
func (r *Repo) PendingStats(ctx context.Context, stuckBefore time.Time) (*domainModel.PendingStats, error) {
	result := &domainModel.PendingStats{}

	err := r.ModelStore.GetConnection(ctx).QueryRow(
		ctx,
		"SELECT count(*), count(*) FILTER (WHERE created_at < $2), min(created_at) FROM change_request WHERE status = $1",
		domainModel.StatusPending, stuckBefore,
	).Scan(&result.Count, &result.Stuck, &result.OldestAt)
	if err != nil {
		return nil, fmt.Errorf("QueryRow: %w", err)
	}

	return result, nil
}

// DeleteClosed removes closed requests decided before the given time.
func (r *Repo) DeleteClosed(ctx context.Context, decidedBefore time.Time) (int64, error) {
	tag, err := r.ModelStore.GetConnection(ctx).Exec(
		ctx,
		"DELETE FROM change_request WHERE status <> $1 AND COALESCE(decided_at, created_at) < $2",
		domainModel.StatusPending, decidedBefore,
	)
	if err != nil {
		return 0, fmt.Errorf("Exec: %w", err)
	}
	return tag.RowsAffected(), nil
}
