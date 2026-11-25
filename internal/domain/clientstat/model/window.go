package model

import "time"

const (
	BucketSize = 5 * time.Minute
	Retention  = 7 * 24 * time.Hour

	MinRange     = 5 * time.Minute
	MaxRange     = Retention
	DefaultRange = 24 * time.Hour

	// maxSteps keeps a timeline at about 100 points: a range that does not
	// start on a step boundary gets one more.
	maxSteps = 96
)

// BucketOf is the bucket a report received at t is summed into.
func BucketOf(t time.Time) time.Time {
	return floorTo(t, BucketSize)
}

// ClampRange applies the default to a missing range and keeps it within
// MinRange..MaxRange.
func ClampRange(rng time.Duration) time.Duration {
	switch {
	case rng <= 0:
		return DefaultRange
	case rng < MinRange:
		return MinRange
	case rng > MaxRange:
		return MaxRange
	}
	return rng
}

// StepFor is the smallest multiple of BucketSize that splits rng into at most
// maxSteps steps.
func StepFor(rng time.Duration) time.Duration {
	buckets := (rng + BucketSize - 1) / BucketSize
	return (buckets + maxSteps - 1) / maxSteps * BucketSize
}

// Window is the regular timeline of a range: Count steps of Step from From,
// the last one holding now. Steps start on multiples of Step in unix time, so
// the points stay put between refreshes.
type Window struct {
	From  time.Time
	Step  time.Duration
	Count int
}

func NewWindow(now time.Time, rng time.Duration) *Window {
	rng = ClampRange(rng)
	step := StepFor(rng)
	first := floorTo(now.Add(-rng), step)
	last := floorTo(now, step)
	return &Window{
		From:  first,
		Step:  step,
		Count: int(last.Sub(first)/step) + 1,
	}
}

// Fill sums per-bucket points into the steps of the window; steps without
// data are zero, points outside the window are dropped.
func (w *Window) Fill(points []*Point) []*Point {
	result := make([]*Point, w.Count)
	for i := range result {
		result[i] = &Point{Ts: w.From.Add(time.Duration(i) * w.Step)}
	}

	for _, p := range points {
		if p.Ts.Before(w.From) {
			continue
		}
		i := int(p.Ts.Sub(w.From) / w.Step)
		if i >= w.Count {
			continue
		}
		dst := result[i]
		dst.Requests += p.Requests
		dst.Blocked += p.Blocked
		dst.Limited += p.Limited
		dst.Trusted += p.Trusted
	}

	return result
}

func floorTo(t time.Time, step time.Duration) time.Time {
	sec := t.Unix()
	s := int64(step / time.Second)
	return time.Unix(sec-sec%s, 0).UTC()
}
