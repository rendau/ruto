package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/domain/seenorigin/model"
)

// The queries run against a real Postgres when RUTO_TEST_PG_DSN is set, e.g.
// the test stand: postgres://ruto:ruto@localhost:55432/ruto?sslmode=disable.
// The tables are created in a schema of their own, dropped afterwards.
func newTestRepo(t *testing.T) (*Repo, *pgxpool.Pool) {
	dsn := os.Getenv("RUTO_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("RUTO_TEST_PG_DSN is not set")
	}
	ctx := context.Background()

	schema := "seen_origin_test_" + strconv.FormatInt(time.Now().UnixNano(), 36)

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

	for _, name := range []string{"20261109120000_seen_origin", "20261127130000_seen_origin_day"} {
		up, err := os.ReadFile("../../../../../migrations/" + name + ".up.sql")
		require.NoError(t, err)
		_, err = con.Exec(ctx, string(up))
		require.NoError(t, err, name)
	}

	return New(con), con
}

func TestRepo_Postgres(t *testing.T) {
	r, con := newTestRepo(t)
	ctx := context.Background()

	loc := time.FixedZone("Almaty", 5*3600)
	day := func(d int) time.Time { return time.Date(2026, 11, d, 12, 0, 0, 0, loc) }
	hits := func(items []*model.SeenOrigin) map[string]int64 {
		return lo.SliceToMap(items, func(v *model.SeenOrigin) (string, int64) { return v.Origin, v.Hits })
	}

	const a, b = "https://a.example", "https://b.example"

	require.NoError(t, r.Add(ctx, day(18), []*model.SeenOrigin{{Origin: a, Hits: 1000, LastSeenAt: day(18)}}))
	require.NoError(t, r.Add(ctx, day(26), []*model.SeenOrigin{{Origin: a, Hits: 5, LastSeenAt: day(26)}}))
	require.NoError(t, r.Add(ctx, day(27), []*model.SeenOrigin{
		{Origin: a, Hits: 3, LastSeenAt: day(27)},
		{Origin: b, Hits: 7, LastSeenAt: day(27)},
	}))
	// a resent report lands in the same day row
	require.NoError(t, r.Add(ctx, day(27), []*model.SeenOrigin{{Origin: a, Hits: 2, LastSeenAt: day(27)}}))

	// the window 21..27 leaves the 18th out
	items, err := r.List(ctx, model.WindowStart(day(27)))
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{a: 10, b: 7}, hits(items))
	assert.Equal(t, a, items[0].Origin, "most requested first")
	assert.True(t, items[0].FirstSeenAt.Equal(day(18)))
	assert.True(t, items[0].LastSeenAt.Equal(day(27)))

	// a week later nobody called: the origins stay, with zero hits
	items, err = r.List(ctx, model.WindowStart(day(27).AddDate(0, 0, 7)))
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{a: 0, b: 0}, hits(items))

	n, err := r.DeleteDaysBefore(ctx, model.WindowStart(day(27)))
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	n, err = r.DeleteDaysBefore(ctx, model.WindowStart(day(27)))
	require.NoError(t, err)
	assert.Zero(t, n)

	// deleting an origin takes its days along
	require.NoError(t, r.DeleteItems(ctx, []string{a}))
	var days int
	require.NoError(t, con.QueryRow(ctx, "SELECT count(*) FROM seen_origin_day").Scan(&days))
	assert.Equal(t, 1, days)

	require.NoError(t, r.DeleteAll(ctx))
	require.NoError(t, con.QueryRow(ctx, "SELECT count(*) FROM seen_origin_day").Scan(&days))
	assert.Zero(t, days)
}

func TestRepo_Postgres_RowCap(t *testing.T) {
	r, con := newTestRepo(t)
	ctx := context.Background()
	now := time.Now()

	items := lo.Times(maxRows+10, func(i int) *model.SeenOrigin {
		return &model.SeenOrigin{Origin: fmt.Sprintf("https://o%d.example", i), Hits: 1, LastSeenAt: now}
	})
	require.NoError(t, r.Add(ctx, now, items))

	var origins, days int
	require.NoError(t, con.QueryRow(ctx, "SELECT count(*) FROM seen_origin").Scan(&origins))
	require.NoError(t, con.QueryRow(ctx, "SELECT count(*) FROM seen_origin_day").Scan(&days))
	assert.Equal(t, maxRows, origins)
	assert.Equal(t, maxRows, days, "an origin left out by the cap gets no day")

	// the known ones keep counting
	require.NoError(t, r.Add(ctx, now, items[:1]))
	list, err := r.List(ctx, model.WindowStart(now))
	require.NoError(t, err)
	assert.Equal(t, items[0].Origin, list[0].Origin)
	assert.EqualValues(t, 2, list[0].Hits)
}
