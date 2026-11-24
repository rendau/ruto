package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendau/mobone/v2"
	moboneTools "github.com/rendau/mobone/v2/tools"
	"github.com/samber/lo"

	domainModel "github.com/rendau/ruto/internal/domain/blockedclient/model"
	repoModel "github.com/rendau/ruto/internal/domain/blockedclient/repo/db/model"
	commonRepoPg "github.com/rendau/ruto/internal/domain/common/repo/pg"
)

// lockKey is the advisory lock that serializes writes to the block list, so
// the caps and the one-block-per-network rule hold under concurrent calls.
const lockKey int64 = 0x626c6f636b6c7374 // "blocklst"

const (
	activeCond   = "removed_at IS NULL AND (expires_at IS NULL OR expires_at > ?)"
	inactiveCond = "(removed_at IS NOT NULL OR expires_at <= ?)"

	activeRulesSQL = `
		SELECT ip, expires_at
		FROM blocked_client
		WHERE removed_at IS NULL AND (expires_at IS NULL OR expires_at > $1)
		ORDER BY ip`

	closeExpiredSQL = `
		UPDATE blocked_client SET removed_at = expires_at
		WHERE ip = $1 AND removed_at IS NULL AND expires_at <= $2`

	statsSQL = `
		SELECT count(*) FILTER (WHERE removed_at IS NULL AND (expires_at IS NULL OR expires_at > $1)),
		       count(*) FILTER (WHERE source = $3 AND created_at >= $2)
		FROM blocked_client`

	removeManySQL = `
		UPDATE blocked_client
		SET removed_at = $1, removed_by = $2, remove_reason = $3, removed_source = $4
		WHERE source = $5 AND removed_at IS NULL AND (expires_at IS NULL OR expires_at > $1)
		  AND ($6::text IS NULL OR api_key_id = $6)`

	pruneSQL = `
		DELETE FROM blocked_client
		WHERE (removed_at IS NOT NULL AND removed_at < $1)
		   OR (removed_at IS NULL AND expires_at < $1)`
)

var allowedSortFields = map[string]string{
	"id":         "id",
	"ip":         "ip",
	"created_at": "created_at",
	"updated_at": "updated_at",
	"removed_at": "removed_at",
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
			TableName:          "blocked_client",
		},
	}
}

func (r *Repo) List(ctx context.Context, pars *domainModel.ListReq) ([]*domainModel.BlockedClient, int64, error) {
	conditions := map[string]any{}
	if pars.Ip != nil {
		conditions["ip"] = *pars.Ip
	}
	if pars.Source != nil {
		conditions["source"] = *pars.Source
	}
	if pars.ApiKeyId != nil {
		conditions["api_key_id"] = *pars.ApiKeyId
	}
	if pars.RemovedSource != nil {
		conditions["removed_source"] = *pars.RemovedSource
	}

	conditionExps := map[string][]any{}
	if pars.ActiveAt != nil {
		conditionExps[activeCond] = []any{*pars.ActiveAt}
	}
	if pars.InactiveAt != nil {
		conditionExps[inactiveCond] = []any{*pars.InactiveAt}
	}
	if pars.NotRemoved {
		conditionExps["removed_at IS NULL"] = nil
	}
	if pars.CreatedAfter != nil {
		conditionExps["created_at >= ?"] = []any{*pars.CreatedAfter}
	}
	if pars.RemovedAfter != nil {
		conditionExps["removed_at > ?"] = []any{*pars.RemovedAfter}
	}

	items := make([]*repoModel.Select, 0)

	totalCount, err := r.ModelStore.List(ctx, mobone.ListParams{
		Conditions:           conditions,
		ConditionExpressions: conditionExps,
		Page:                 pars.Page,
		PageSize:             pars.PageSize,
		WithTotalCount:       pars.WithTotalCount,
		OnlyCount:            pars.OnlyCount,
		Sort:                 moboneTools.ConstructSortColumns(allowedSortFields, pars.Sort),
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

func (r *Repo) Get(ctx context.Context, id string) (*domainModel.BlockedClient, bool, error) {
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

func (r *Repo) Create(ctx context.Context, obj *domainModel.BlockedClient) (string, error) {
	m := repoModel.DecodeCreate(obj)

	if err := r.ModelStore.Create(ctx, m); err != nil {
		return "", fmt.Errorf("ModelStore.Create: %w", err)
	}

	return m.PKId, nil
}

func (r *Repo) Update(ctx context.Context, id string, obj *domainModel.Edit) error {
	if err := r.ModelStore.Update(ctx, &repoModel.Update{PKId: id, Edit: obj}); err != nil {
		return fmt.Errorf("ModelStore.Update: %w", err)
	}
	return nil
}

// Lock takes the block list write lock for the rest of the ctx transaction.
func (r *Repo) Lock(ctx context.Context) error {
	if _, err := r.ModelStore.GetConnection(ctx).Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
		return fmt.Errorf("Exec: %w", err)
	}
	return nil
}

// CloseExpired marks an expired, not lifted block of the network as ended at
// its expiry, freeing the network for a new block.
func (r *Repo) CloseExpired(ctx context.Context, ip string, now time.Time) error {
	if _, err := r.ModelStore.GetConnection(ctx).Exec(ctx, closeExpiredSQL, ip, now); err != nil {
		return fmt.Errorf("Exec: %w", err)
	}
	return nil
}

func (r *Repo) ActiveRules(ctx context.Context, now time.Time) ([]*domainModel.Rule, error) {
	rows, err := r.ModelStore.GetConnection(ctx).Query(ctx, activeRulesSQL, now)
	if err != nil {
		return nil, fmt.Errorf("Query: %w", err)
	}

	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*domainModel.Rule, error) {
		item := &domainModel.Rule{}
		scanErr := row.Scan(&item.Ip, &item.ExpiresAt)
		return item, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("CollectRows: %w", err)
	}

	return items, nil
}

func (r *Repo) Stats(ctx context.Context, now, agentsSince time.Time) (*domainModel.Stats, error) {
	result := &domainModel.Stats{}

	err := r.ModelStore.GetConnection(ctx).QueryRow(ctx, statsSQL, now, agentsSince, domainModel.SourceAgent).
		Scan(&result.Active, &result.ByAgentsSince)
	if err != nil {
		return nil, fmt.Errorf("QueryRow: %w", err)
	}

	return result, nil
}

// RemoveMany lifts the active agent blocks (of one api key, if given) in one
// statement.
func (r *Repo) RemoveMany(ctx context.Context, req *domainModel.RemoveMany) (int64, error) {
	tag, err := r.ModelStore.GetConnection(ctx).Exec(ctx, removeManySQL,
		req.Now, req.RemovedBy, req.RemoveReason, req.RemovedSource, domainModel.SourceAgent, req.ApiKeyId)
	if err != nil {
		return 0, fmt.Errorf("Exec: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Prune deletes the history of blocks that ended before the given time.
func (r *Repo) Prune(ctx context.Context, endedBefore time.Time) (int64, error) {
	tag, err := r.ModelStore.GetConnection(ctx).Exec(ctx, pruneSQL, endedBefore)
	if err != nil {
		return 0, fmt.Errorf("Exec: %w", err)
	}
	return tag.RowsAffected(), nil
}
