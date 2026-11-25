package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClampRange(t *testing.T) {
	assert.Equal(t, DefaultRange, ClampRange(0))
	assert.Equal(t, DefaultRange, ClampRange(-time.Hour))
	assert.Equal(t, MinRange, ClampRange(time.Second))
	assert.Equal(t, MaxRange, ClampRange(30*24*time.Hour))
	assert.Equal(t, 6*time.Hour, ClampRange(6*time.Hour))
}

func TestStepFor(t *testing.T) {
	cases := map[time.Duration]time.Duration{
		5 * time.Minute:     5 * time.Minute,
		time.Hour:           5 * time.Minute,
		6 * time.Hour:       5 * time.Minute,
		8 * time.Hour:       5 * time.Minute,
		8*time.Hour + 1:     10 * time.Minute,
		12 * time.Hour:      10 * time.Minute,
		24 * time.Hour:      15 * time.Minute,
		3 * 24 * time.Hour:  45 * time.Minute,
		7 * 24 * time.Hour:  105 * time.Minute,
		90 * time.Second:    5 * time.Minute,
		299 * time.Minute:   5 * time.Minute,
		481 * time.Minute:   10 * time.Minute,
		1441 * time.Minute:  20 * time.Minute,
		10079 * time.Minute: 105 * time.Minute,
	}
	for rng, want := range cases {
		got := StepFor(rng)
		assert.Equal(t, want, got, rng)
		assert.Zero(t, got%BucketSize, rng)
	}
}

func TestBucketOf(t *testing.T) {
	at := time.Date(2026, 11, 25, 12, 34, 56, 789, time.FixedZone("Almaty", 5*3600))
	assert.Equal(t, time.Date(2026, 11, 25, 7, 30, 0, 0, time.UTC), BucketOf(at))
	assert.Equal(t, time.UTC, BucketOf(at).Location())
}

func TestNewWindow(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 1, 0, 0, time.UTC)

	for _, rng := range []time.Duration{0, time.Second, time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour, 3 * 24 * time.Hour, 7 * 24 * time.Hour, 365 * 24 * time.Hour} {
		w := NewWindow(now, rng)
		clamped := ClampRange(rng)

		assert.Equal(t, StepFor(clamped), w.Step, rng)
		assert.Zero(t, w.From.Unix()%int64(w.Step/time.Second), "steps start on multiples of the step: %v", rng)
		assert.LessOrEqual(t, w.Count, 100, rng)
		assert.False(t, w.From.After(now.Add(-clamped)), "the window covers the whole range: %v", rng)
		last := w.From.Add(time.Duration(w.Count-1) * w.Step)
		assert.False(t, last.After(now), rng)
		assert.True(t, last.Add(w.Step).After(now), "the last step holds now: %v", rng)
	}

	// 24h at 12:01 with 15m steps: from yesterday 12:00 to today 12:00
	w := NewWindow(now, 24*time.Hour)
	assert.Equal(t, time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC), w.From)
	assert.Equal(t, 97, w.Count)

	// 5m at 12:01: the previous bucket and the current one
	w = NewWindow(now, 5*time.Minute)
	assert.Equal(t, time.Date(2026, 11, 25, 11, 55, 0, 0, time.UTC), w.From)
	assert.Equal(t, 2, w.Count)
}

func TestWindowFill(t *testing.T) {
	from := time.Date(2026, 11, 25, 11, 0, 0, 0, time.UTC)
	w := &Window{From: from, Step: 15 * time.Minute, Count: 4}

	points := w.Fill([]*Point{
		{Ts: from.Add(-5 * time.Minute), Requests: 1000},
		{Ts: from, Requests: 1, Blocked: 2, Limited: 3, Trusted: 4},
		{Ts: from.Add(10 * time.Minute), Requests: 10},
		{Ts: from.Add(30 * time.Minute), Requests: 7, Trusted: 7},
		{Ts: from.Add(time.Hour), Requests: 1000},
	})
	require.Len(t, points, 4)

	for i, p := range points {
		assert.Equal(t, from.Add(time.Duration(i)*15*time.Minute), p.Ts)
	}
	assert.Equal(t, Point{Ts: from, Requests: 11, Blocked: 2, Limited: 3, Trusted: 4}, *points[0])
	assert.Equal(t, Point{Ts: from.Add(15 * time.Minute)}, *points[1], "a step without data is zero")
	assert.Equal(t, Point{Ts: from.Add(30 * time.Minute), Requests: 7, Trusted: 7}, *points[2])
	assert.Equal(t, Point{Ts: from.Add(45 * time.Minute)}, *points[3])

	assert.Len(t, w.Fill(nil), 4)
}
