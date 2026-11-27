package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	commonRepoPg "github.com/rendau/ruto/internal/domain/common/repo/pg"
	"github.com/rendau/ruto/internal/domain/seenorigin/model"
)

const (
	// maxRows stops the table from growing under a scan that forges Origin
	// headers: once full, only the known origins keep counting.
	maxRows = 2000

	addOriginSQL = `
		INSERT INTO seen_origin (origin, first_seen_at, last_seen_at)
		SELECT $1, $2, $2
		WHERE EXISTS (SELECT 1 FROM seen_origin WHERE origin = $1)
		   OR (SELECT count(*) FROM seen_origin) < $3
		ON CONFLICT (origin) DO UPDATE SET
			last_seen_at = GREATEST(seen_origin.last_seen_at, EXCLUDED.last_seen_at)`

	// addDaySQL runs after addOriginSQL in the same batch: an origin left out
	// by the row cap gets no day either.
	addDaySQL = `
		INSERT INTO seen_origin_day (origin, day, hits)
		SELECT $1, $2::date, $3
		WHERE EXISTS (SELECT 1 FROM seen_origin WHERE origin = $1)
		ON CONFLICT (origin, day) DO UPDATE SET
			hits = seen_origin_day.hits + EXCLUDED.hits`

	listSQL = `
		SELECT o.origin, COALESCE(d.hits, 0), o.first_seen_at, o.last_seen_at
		FROM seen_origin o
		LEFT JOIN (
			SELECT origin, sum(hits)::bigint AS hits
			FROM seen_origin_day
			WHERE day >= $1::date
			GROUP BY origin
		) d ON d.origin = o.origin
		ORDER BY 2 DESC, o.last_seen_at DESC, o.origin`

	deleteItemsSQL = `DELETE FROM seen_origin WHERE origin = ANY($1::text[])`

	deleteAllSQL = `DELETE FROM seen_origin`

	deleteDaysBeforeSQL = `DELETE FROM seen_origin_day WHERE day < $1::date`
)

type Repo struct {
	*commonRepoPg.Base
}

func New(con *pgxpool.Pool) *Repo {
	return &Repo{Base: commonRepoPg.NewBase(con)}
}

// Add merges the counters into the stored ones, the hits into the given day.
// It is all-or-nothing, so a gateway may safely resend the whole report after
// a failure.
func (r *Repo) Add(ctx context.Context, day time.Time, items []*model.SeenOrigin) error {
	if len(items) == 0 {
		return nil
	}

	dayStr := day.Format(time.DateOnly)

	batch := &pgx.Batch{}
	for _, item := range items {
		batch.Queue(addOriginSQL, item.Origin, item.LastSeenAt, maxRows)
		batch.Queue(addDaySQL, item.Origin, dayStr, item.Hits)
	}

	err := pgx.BeginFunc(ctx, r.Con, func(tx pgx.Tx) error {
		return tx.SendBatch(ctx, batch).Close()
	})
	if err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	return nil
}

// List gives every stored origin with its hits since the day since.
func (r *Repo) List(ctx context.Context, since time.Time) ([]*model.SeenOrigin, error) {
	rows, err := r.Txm.GetConnection(ctx).Query(ctx, listSQL, since.Format(time.DateOnly))
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

func (r *Repo) DeleteDaysBefore(ctx context.Context, day time.Time) (int64, error) {
	tag, err := r.Txm.GetConnection(ctx).Exec(ctx, deleteDaysBeforeSQL, day.Format(time.DateOnly))
	if err != nil {
		return 0, fmt.Errorf("exec: %w", err)
	}
	return tag.RowsAffected(), nil
}
