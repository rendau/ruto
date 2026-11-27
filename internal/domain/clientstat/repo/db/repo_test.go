package db

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/rendau/ruto/internal/domain/clientstat/model"
)

// The queries run against a real Postgres when RUTO_TEST_PG_DSN is set, e.g.
// the test stand: postgres://ruto:ruto@localhost:55432/ruto?sslmode=disable.
// The table is created in a schema of its own, dropped afterwards.
func newTestRepo(t *testing.T) *Repo {
	dsn := os.Getenv("RUTO_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("RUTO_TEST_PG_DSN is not set")
	}
	ctx := context.Background()

	schema := "client_stat_test_" + strconv.FormatInt(time.Now().UnixNano(), 36)

	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	con, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(con.Close)

	up, err := os.ReadFile("../../../../../migrations/20261125120000_client_stat.up.sql")
	require.NoError(t, err)
	_, err = con.Exec(ctx, string(up))
	require.NoError(t, err)

	return New(con)
}

func TestRepo_Postgres(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	b1 := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	b2 := b1.Add(5 * time.Minute)
	old := b1.Add(-8 * 24 * time.Hour)

	require.NoError(t, r.Add(ctx, b1, []*model.ClientStat{
		{Ip: "203.0.113.7", Requests: 5, Blocked: 2},
		{Ip: "", Requests: 100, Blocked: 1},
		{Ip: "10.0.0.0/8", Trusted: "1c", Requests: 50},
		{Ip: "198.51.100.1", Requests: 4, Limited: 3, LastRule: "r1"},
	}))
	// a second gateway in the same bucket: counters add up, a newer rule wins,
	// an empty one keeps the stored rule
	require.NoError(t, r.Add(ctx, b1, []*model.ClientStat{
		{Ip: "203.0.113.7", Requests: 3, Blocked: 1},
		{Ip: "198.51.100.1", Requests: 1, Limited: 1, LastRule: "r2"},
	}))
	require.NoError(t, r.Add(ctx, b1, []*model.ClientStat{
		{Ip: "198.51.100.1", Requests: 1},
	}))
	require.NoError(t, r.Add(ctx, b2, []*model.ClientStat{
		{Ip: "203.0.113.7", Requests: 1},
		{Ip: "198.51.100.1", Requests: 2, Limited: 2},
		{Ip: "2001:db8:1:2::/64", Requests: 7},
	}))
	require.NoError(t, r.Add(ctx, old, []*model.ClientStat{{Ip: "192.0.2.1", Requests: 1}}))
	require.NoError(t, r.Add(ctx, b1, nil))

	// per-bucket sums
	sums, err := r.BucketSums(ctx, b1)
	require.NoError(t, err)
	require.Len(t, sums, 2)
	assert.Equal(t, model.Point{Ts: b1, Requests: 164, Blocked: 4, Limited: 4, Trusted: 50}, *norm(sums[0]))
	assert.Equal(t, model.Point{Ts: b2, Requests: 10, Limited: 2}, *norm(sums[1]))

	// one client
	ipPoints, err := r.IpBuckets(ctx, "203.0.113.7", b1)
	require.NoError(t, err)
	require.Len(t, ipPoints, 2)
	assert.Equal(t, model.Point{Ts: b1, Requests: 8, Blocked: 3}, *norm(ipPoints[0]))
	assert.Equal(t, model.Point{Ts: b2, Requests: 1}, *norm(ipPoints[1]))

	ipPoints, err = r.IpBuckets(ctx, "10.0.0.0/8", b1)
	require.NoError(t, err)
	require.Len(t, ipPoints, 1)
	assert.Equal(t, int64(50), ipPoints[0].Trusted)

	// trusted
	trusted, err := r.TrustedBucketSums(ctx, b1)
	require.NoError(t, err)
	require.Len(t, trusted, 1)
	assert.Equal(t, "1c", trusted[0].Name)
	assert.Equal(t, model.Point{Ts: b1, Requests: 50, Trusted: 50}, *norm(&trusted[0].Point))

	// totals: the "" row counts in the sums, not as a client
	totals, err := r.Totals(ctx, b1)
	require.NoError(t, err)
	assert.Equal(t, model.Totals{Requests: 174, Blocked: 4, Limited: 6, Trusted: 50, Clients: 4}, *totals)

	totals, err = r.Totals(ctx, b2.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, model.Totals{}, *totals)

	// top
	top := func(kind string, limit int64) []*model.Item {
		items, err := r.Top(ctx, &model.TopReq{Since: b1, Kind: kind, Limit: limit, Recent: b2})
		require.NoError(t, err)
		return items
	}
	ips := func(items []*model.Item) []string {
		result := make([]string, len(items))
		for i, v := range items {
			result[i] = v.Ip
		}
		return result
	}

	assert.Equal(t, []string{"10.0.0.0/8", "203.0.113.7", "198.51.100.1", "2001:db8:1:2::/64"}, ips(top(model.KindRequests, 50)))
	assert.Equal(t, []string{"10.0.0.0/8", "203.0.113.7"}, ips(top(model.KindRequests, 2)))
	assert.Equal(t, []string{"203.0.113.7", ""}, ips(top(model.KindBlocked, 50)))
	assert.Equal(t, []string{"10.0.0.0/8"}, ips(top(model.KindTrusted, 50)))

	limited := top(model.KindLimited, 50)
	require.Len(t, limited, 1)
	assert.Equal(t, "198.51.100.1", limited[0].Ip)
	assert.Equal(t, int64(8), limited[0].Requests)
	assert.Equal(t, int64(6), limited[0].Limited)
	assert.Equal(t, "r2", limited[0].LastRule, "the rule of the latest bucket that has one")
	assert.True(t, b1.Equal(limited[0].FirstSeenAt))
	assert.True(t, b2.Equal(limited[0].LastSeenAt))
	assert.Equal(t, int64(6), limited[0].PeakRequests, "the busiest bucket, all gateways summed")
	assert.Equal(t, int64(2), limited[0].RecentRequests)

	trustedTop := top(model.KindTrusted, 50)
	require.Len(t, trustedTop, 1)
	assert.Equal(t, int64(50), trustedTop[0].PeakRequests)
	assert.Equal(t, int64(0), trustedTop[0].RecentRequests, "no requests in the recent bucket")

	_, err = r.Top(ctx, &model.TopReq{Since: b1, Kind: "bytes", Limit: 1})
	require.Error(t, err)

	// retention
	n, err := r.DeleteBefore(ctx, b1.Add(-model.Retention))
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	sums, err = r.BucketSums(ctx, old)
	require.NoError(t, err)
	assert.Len(t, sums, 2)
}

// Several gateways report the same clients at once: the rows are written in
// one order, so the transactions wait for each other instead of deadlocking.
func TestRepo_Postgres_ConcurrentReports(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	bucket := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

	const (
		gateways = 4
		rounds   = 5
		clients  = 300
	)

	eg, egCtx := errgroup.WithContext(ctx)
	for g := range gateways {
		eg.Go(func() error {
			for range rounds {
				items := make([]*model.ClientStat, clients)
				for i := range items {
					items[i] = &model.ClientStat{Ip: fmt.Sprintf("198.51.%d.%d", i/250, i%250), Requests: 1}
				}
				rand.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
				if err := r.Add(egCtx, bucket, items); err != nil {
					return fmt.Errorf("gateway %d: %w", g, err)
				}
			}
			return nil
		})
	}
	require.NoError(t, eg.Wait())

	totals, err := r.Totals(ctx, bucket)
	require.NoError(t, err)
	assert.Equal(t, int64(gateways*rounds*clients), totals.Requests)
	assert.Equal(t, int64(clients), totals.Clients)
}

func norm(p *model.Point) *model.Point {
	p.Ts = p.Ts.UTC()
	return p
}
