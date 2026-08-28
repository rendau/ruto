package monitoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	prometheusModel "github.com/rendau/ruto/internal/service/prometheus/model"
)

func TestClampRangeSeconds(t *testing.T) {
	assert.Equal(t, int64(rangeSecondsDefault), clampRangeSeconds(0))
	assert.Equal(t, int64(rangeSecondsDefault), clampRangeSeconds(-5))
	assert.Equal(t, int64(rangeSecondsMin), clampRangeSeconds(10))
	assert.Equal(t, int64(3600), clampRangeSeconds(3600))
	assert.Equal(t, int64(rangeSecondsMax), clampRangeSeconds(rangeSecondsMax+1))
}

func TestRangePars(t *testing.T) {
	start, end, step, rateWindow := rangePars(3600)

	assert.Equal(t, 60*time.Second, step)
	assert.Equal(t, "240s", rateWindow)
	assert.Equal(t, 3600*time.Second, end.Sub(start))
	// end is aligned to the step so consecutive queries hit the same buckets
	assert.Zero(t, end.Unix()%60)

	// small ranges keep the minimal quantum and the 60s rate-window floor
	_, _, step, rateWindow = rangePars(300)
	assert.Equal(t, 15*time.Second, step)
	assert.Equal(t, "60s", rateWindow)
}

func TestMergeSeries(t *testing.T) {
	rps := []prometheusModel.Series{{Points: []prometheusModel.Point{
		{Ts: 100, Value: 10},
		{Ts: 160, Value: 20},
	}}}
	errRps := []prometheusModel.Series{{Points: []prometheusModel.Point{
		{Ts: 100, Value: 1},
		{Ts: 220, Value: 5}, // no rps at this ts → rate stays 0
	}}}
	dur := []prometheusModel.Series{{Points: []prometheusModel.Point{
		{Ts: 160, Value: 0.25},
	}}}

	result := mergeSeries(rps, errRps, dur, 60*time.Second)

	require.Len(t, result.Points, 3)
	assert.Equal(t, int64(60), result.StepSeconds)

	assert.Equal(t, int64(100), result.Points[0].Ts)
	assert.Equal(t, 10.0, result.Points[0].Rps)
	assert.Equal(t, 0.1, result.Points[0].ErrorRate)

	assert.Equal(t, int64(160), result.Points[1].Ts)
	assert.Equal(t, 0.25, result.Points[1].DurationAvgSeconds)
	assert.Zero(t, result.Points[1].ErrorRate)

	assert.Equal(t, int64(220), result.Points[2].Ts)
	assert.Zero(t, result.Points[2].ErrorRate)
}
