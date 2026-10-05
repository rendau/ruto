package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rendau/ruto/internal/domain/seenpath/model"
)

const (
	// maxRowsPerEndpoint stops the table from growing forever under URL scans:
	// once an endpoint has this many paths, only the known ones keep counting.
	maxRowsPerEndpoint = 5000

	listLimit = 5000

	addSQL = `
		INSERT INTO seen_path (endpoint_id, method, path, app_id, hits, hits_not_found, first_seen_at, last_seen_at, sample)
		SELECT $1, $2, $3, $4, $5, $6, $7, $7, $8
		WHERE EXISTS (SELECT 1 FROM seen_path WHERE endpoint_id = $1 AND method = $2 AND path = $3)
		   OR (SELECT count(*) FROM seen_path WHERE endpoint_id = $1) < $9
		ON CONFLICT (endpoint_id, method, path) DO UPDATE SET
			app_id         = EXCLUDED.app_id,
			hits           = seen_path.hits + EXCLUDED.hits,
			hits_not_found = seen_path.hits_not_found + EXCLUDED.hits_not_found,
			sample         = CASE WHEN EXCLUDED.last_seen_at >= seen_path.last_seen_at THEN EXCLUDED.sample ELSE seen_path.sample END,
			last_seen_at   = GREATEST(seen_path.last_seen_at, EXCLUDED.last_seen_at)`

	listSQL = `
		SELECT app_id, endpoint_id, method, path, hits, hits_not_found, first_seen_at, last_seen_at, sample
		FROM seen_path
		WHERE app_id = $1
		ORDER BY hits DESC, path, method
		LIMIT $2`

	deleteByAppSQL = `DELETE FROM seen_path WHERE app_id = $1`
)

type Repo struct {
	con *pgxpool.Pool
}

func New(con *pgxpool.Pool) *Repo {
	return &Repo{con: con}
}

// Add merges the counters into the stored ones. It is all-or-nothing, so a
// gateway may safely resend the whole report after a failure.
func (r *Repo) Add(ctx context.Context, items []*model.SeenPath) error {
	if len(items) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, item := range items {
		batch.Queue(addSQL,
			item.EndpointId, item.Method, item.Path, item.AppId,
			item.Hits, item.HitsNotFound, item.LastSeenAt, item.Sample,
			maxRowsPerEndpoint,
		)
	}

	err := pgx.BeginFunc(ctx, r.con, func(tx pgx.Tx) error {
		return tx.SendBatch(ctx, batch).Close()
	})
	if err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	return nil
}

func (r *Repo) ListByApp(ctx context.Context, appId string) ([]*model.SeenPath, error) {
	rows, err := r.con.Query(ctx, listSQL, appId, listLimit)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*model.SeenPath, error) {
		item := &model.SeenPath{}
		scanErr := row.Scan(
			&item.AppId, &item.EndpointId, &item.Method, &item.Path,
			&item.Hits, &item.HitsNotFound, &item.FirstSeenAt, &item.LastSeenAt, &item.Sample,
		)
		return item, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("collect rows: %w", err)
	}

	return items, nil
}

func (r *Repo) DeleteByApp(ctx context.Context, appId string) error {
	if _, err := r.con.Exec(ctx, deleteByAppSQL, appId); err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	return nil
}
