package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/domain/clientstat/model"
)

type testRepo struct {
	RepoDbI

	since   time.Time
	sums    []*model.Point
	trusted []*model.TrustedPoint
}

func (r *testRepo) BucketSums(_ context.Context, since time.Time) ([]*model.Point, error) {
	r.since = since
	return r.sums, nil
}

func (r *testRepo) TrustedBucketSums(_ context.Context, _ time.Time) ([]*model.TrustedPoint, error) {
	return r.trusted, nil
}

func (r *testRepo) Totals(_ context.Context, _ time.Time) (*model.Totals, error) {
	return &model.Totals{Requests: 1}, nil
}

func TestService_Overview(t *testing.T) {
	from := time.Date(2026, 11, 25, 11, 0, 0, 0, time.UTC)
	w := &model.Window{From: from, Step: 10 * time.Minute, Count: 3}

	repo := &testRepo{
		sums: []*model.Point{{Ts: from.Add(5 * time.Minute), Requests: 5}},
		trusted: []*model.TrustedPoint{
			{Name: "1c", Point: model.Point{Ts: from, Requests: 2, Trusted: 2}},
			{Name: "erp", Point: model.Point{Ts: from, Requests: 3, Trusted: 3}},
			{Name: "1c", Point: model.Point{Ts: from.Add(20 * time.Minute), Requests: 4, Trusted: 4}},
			{Name: "bi", Point: model.Point{Ts: from, Requests: 6, Trusted: 6}},
		},
	}

	result, err := New(repo).Overview(context.Background(), w)
	require.NoError(t, err)

	assert.Equal(t, from, repo.since)
	assert.Equal(t, w.Step, result.Step)
	require.Len(t, result.Points, 3)
	assert.Equal(t, int64(5), result.Points[0].Requests)
	assert.Equal(t, int64(1), result.Totals.Requests)

	// busiest first, a tie by name
	require.Len(t, result.Trusted, 3)
	assert.Equal(t, []string{"1c", "bi", "erp"}, []string{result.Trusted[0].Name, result.Trusted[1].Name, result.Trusted[2].Name})
	assert.Equal(t, int64(6), result.Trusted[0].Requests)
	require.Len(t, result.Trusted[0].Points, 3)
	assert.Equal(t, int64(2), result.Trusted[0].Points[0].Requests)
	assert.Zero(t, result.Trusted[0].Points[1].Requests)
	assert.Equal(t, int64(4), result.Trusted[0].Points[2].Trusted)
}
