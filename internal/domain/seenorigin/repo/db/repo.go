package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	commonRepoPg "github.com/rendau/ruto/internal/domain/common/repo/pg"
	"github.com/rendau/ruto/internal/domain/seenorigin/model"
)

const (
	// maxRows stops the table from growing under a scan that forges Origin
	// headers: once full, only the known origins keep counting.
	maxRows = 2000

	addSQL = `
		INSERT INTO seen_origin (origin, hits, first_seen_at, last_seen_at)
		SELECT $1, $2, $3, $3
		WHERE EXISTS (SELECT 1 FROM seen_origin WHERE origin = $1)
		   OR (SELECT count(*) FROM seen_origin) < $4
		ON CONFLICT (origin) DO UPDATE SET
			hits         = seen_origin.hits + EXCLUDED.hits,
			last_seen_at = GREATEST(seen_origin.last_seen_at, EXCLUDED.last_seen_at)`

	listSQL = `
		SELECT origin, hits, first_seen_at, last_seen_at
		FROM seen_origin
		ORDER BY hits DESC, origin`

	deleteItemsSQL = `DELETE FROM seen_origin WHERE origin = ANY($1::text[])`

	deleteAllSQL = `DELETE FROM seen_origin`
)

type Repo struct {
	*commonRepoPg.Base
}

func New(con *pgxpool.Pool) *Repo {
	return &Repo{Base: commonRepoPg.NewBase(con)}
}

// Add merges the counters into the stored ones. It is all-or-nothing, so a
// gateway may safely resend the whole report after a failure.
func (r *Repo) Add(ctx context.Context, items []*model.SeenOrigin) error {
	if len(items) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, item := range items {
		batch.Queue(addSQL, item.Origin, item.Hits, item.LastSeenAt, maxRows)
	}

	err := pgx.BeginFunc(ctx, r.Con, func(tx pgx.Tx) error {
		return tx.SendBatch(ctx, batch).Close()
	})
	if err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	return nil
}

func (r *Repo) List(ctx context.Context) ([]*model.SeenOrigin, error) {
	rows, err := r.Txm.GetConnection(ctx).Query(ctx, listSQL)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*model.SeenOrigin, error) {
		item := &model.SeenOrigin{}
		scanErr := row.Scan(&item.Origin, &item.Hits, &item.FirstSeenAt, &item.LastSeenAt)
		return item, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("collect rows: %w", err)
	}

	return items, nil
}

func (r *Repo) DeleteItems(ctx context.Context, origins []string) error {
	if len(origins) == 0 {
		return nil
	}

	if _, err := r.Txm.GetConnection(ctx).Exec(ctx, deleteItemsSQL, origins); err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	return nil
}

func (r *Repo) DeleteAll(ctx context.Context) error {
	if _, err := r.Txm.GetConnection(ctx).Exec(ctx, deleteAllSQL); err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	return nil
}
