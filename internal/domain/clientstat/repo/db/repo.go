package db

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rendau/ruto/internal/domain/clientstat/model"
	commonRepoPg "github.com/rendau/ruto/internal/domain/common/repo/pg"
	"github.com/rendau/ruto/internal/errs"
)

const (
	addSQL = `
		INSERT INTO client_stat (bucket, ip, trusted, requests, blocked, limited, last_rule)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (bucket, ip) DO UPDATE SET
			trusted   = EXCLUDED.trusted,
			requests  = client_stat.requests + EXCLUDED.requests,
			blocked   = client_stat.blocked + EXCLUDED.blocked,
			limited   = client_stat.limited + EXCLUDED.limited,
			last_rule = CASE WHEN EXCLUDED.last_rule <> '' THEN EXCLUDED.last_rule ELSE client_stat.last_rule END`

	bucketSumsSQL = `
		SELECT bucket,
		       sum(requests)::bigint,
		       sum(blocked)::bigint,
		       sum(limited)::bigint,
		       coalesce(sum(requests) FILTER (WHERE trusted <> ''), 0)::bigint
		FROM client_stat
		WHERE bucket >= $1
		GROUP BY bucket
		ORDER BY bucket`

	ipBucketsSQL = `
		SELECT bucket, requests, blocked, limited, CASE WHEN trusted <> '' THEN requests ELSE 0 END
		FROM client_stat
		WHERE ip = $1 AND bucket >= $2
		ORDER BY bucket`

	trustedBucketSumsSQL = `
		SELECT trusted, bucket,
		       sum(requests)::bigint,
		       sum(blocked)::bigint,
		       sum(limited)::bigint
		FROM client_stat
		WHERE bucket >= $1 AND trusted <> ''
		GROUP BY trusted, bucket
		ORDER BY trusted, bucket`

	totalsSQL = `
		SELECT coalesce(sum(requests), 0)::bigint,
		       coalesce(sum(blocked), 0)::bigint,
		       coalesce(sum(limited), 0)::bigint,
		       coalesce(sum(requests) FILTER (WHERE trusted <> ''), 0)::bigint,
		       count(DISTINCT ip) FILTER (WHERE ip <> '')
		FROM client_stat
		WHERE bucket >= $1`

	topSQL = `
		SELECT ip, trusted,
		       sum(requests)::bigint AS requests,
		       sum(blocked)::bigint  AS blocked,
		       sum(limited)::bigint  AS limited,
		       coalesce((array_agg(last_rule ORDER BY bucket DESC) FILTER (WHERE last_rule <> ''))[1], ''),
		       min(bucket),
		       max(bucket),
		       max(requests)::bigint,
		       coalesce(sum(requests) FILTER (WHERE bucket = $3), 0)::bigint
		FROM client_stat
		WHERE bucket >= $1 AND %s
		GROUP BY ip, trusted
		HAVING %s
		ORDER BY %s DESC, ip, trusted
		LIMIT $2`

	deleteBeforeSQL = `DELETE FROM client_stat WHERE bucket < $1`
)

// topKinds are the fixed SQL parts of Top per kind: row filter, group filter
// and the sum to order by.
var topKinds = map[string][3]string{
	model.KindRequests: {"ip <> ''", "true", "requests"},
	model.KindBlocked:  {"true", "sum(blocked) > 0", "blocked"},
	model.KindLimited:  {"true", "sum(limited) > 0", "limited"},
	model.KindTrusted:  {"trusted <> ''", "true", "requests"},
}

type Repo struct {
	*commonRepoPg.Base
}

func New(con *pgxpool.Pool) *Repo {
	return &Repo{Base: commonRepoPg.NewBase(con)}
}

// Add merges a report into the bucket. Rows go in ip order, so concurrent
// reports of several gateways lock them in the same order and do not
// deadlock.
func (r *Repo) Add(ctx context.Context, bucket time.Time, items []*model.ClientStat) error {
	if len(items) == 0 {
		return nil
	}

	sorted := slices.SortedFunc(slices.Values(items), func(a, b *model.ClientStat) int {
		return cmp.Compare(a.Ip, b.Ip)
	})

	batch := &pgx.Batch{}
	for _, item := range sorted {
		batch.Queue(addSQL, bucket, item.Ip, item.Trusted, item.Requests, item.Blocked, item.Limited, item.LastRule)
	}

	err := pgx.BeginFunc(ctx, r.Con, func(tx pgx.Tx) error {
		return tx.SendBatch(ctx, batch).Close()
	})
	if err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	return nil
}

// BucketSums sums every stored bucket since the given one over all clients.
func (r *Repo) BucketSums(ctx context.Context, since time.Time) ([]*model.Point, error) {
	return r.queryPoints(ctx, bucketSumsSQL, since)
}

// IpBuckets returns the stored buckets of one client since the given one.
func (r *Repo) IpBuckets(ctx context.Context, ip string, since time.Time) ([]*model.Point, error) {
	return r.queryPoints(ctx, ipBucketsSQL, ip, since)
}

func (r *Repo) queryPoints(ctx context.Context, sql string, args ...any) ([]*model.Point, error) {
	rows, err := r.Txm.GetConnection(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*model.Point, error) {
		item := &model.Point{}
		scanErr := row.Scan(&item.Ts, &item.Requests, &item.Blocked, &item.Limited, &item.Trusted)
		return item, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("collect rows: %w", err)
	}

	return items, nil
}

func (r *Repo) TrustedBucketSums(ctx context.Context, since time.Time) ([]*model.TrustedPoint, error) {
	rows, err := r.Txm.GetConnection(ctx).Query(ctx, trustedBucketSumsSQL, since)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*model.TrustedPoint, error) {
		item := &model.TrustedPoint{}
		scanErr := row.Scan(&item.Name, &item.Ts, &item.Requests, &item.Blocked, &item.Limited)
		item.Trusted = item.Requests
		return item, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("collect rows: %w", err)
	}

	return items, nil
}

func (r *Repo) Totals(ctx context.Context, since time.Time) (*model.Totals, error) {
	result := &model.Totals{}
	err := r.Txm.GetConnection(ctx).QueryRow(ctx, totalsSQL, since).Scan(
		&result.Requests, &result.Blocked, &result.Limited, &result.Trusted, &result.Clients,
	)
	if err != nil {
		return nil, fmt.Errorf("query row: %w", err)
	}
	return result, nil
}

func (r *Repo) Top(ctx context.Context, req *model.TopReq) ([]*model.Item, error) {
	parts, ok := topKinds[req.Kind]
	if !ok {
		return nil, errs.InvalidRequest
	}

	rows, err := r.Txm.GetConnection(ctx).Query(ctx, fmt.Sprintf(topSQL, parts[0], parts[1], parts[2]), req.Since, req.Limit, req.Recent)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*model.Item, error) {
		item := &model.Item{}
		scanErr := row.Scan(
			&item.Ip, &item.Trusted, &item.Requests, &item.Blocked, &item.Limited,
			&item.LastRule, &item.FirstSeenAt, &item.LastSeenAt,
			&item.PeakRequests, &item.RecentRequests,
		)
		return item, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("collect rows: %w", err)
	}

	return items, nil
}

// DeleteBefore removes buckets older than the given time.
func (r *Repo) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.Txm.GetConnection(ctx).Exec(ctx, deleteBeforeSQL, before)
	if err != nil {
		return 0, fmt.Errorf("exec: %w", err)
	}
	return tag.RowsAffected(), nil
}
