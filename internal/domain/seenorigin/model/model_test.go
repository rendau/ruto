package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWindowStart(t *testing.T) {
	loc := time.FixedZone("Almaty", 5*3600)

	// the window is today and the 6 days before, by the local day
	assert.Equal(t,
		time.Date(2026, 11, 21, 0, 0, 0, 0, loc),
		WindowStart(time.Date(2026, 11, 27, 0, 30, 0, 0, loc)))
	assert.Equal(t,
		time.Date(2026, 11, 26, 0, 0, 0, 0, loc),
		WindowStart(time.Date(2026, 12, 2, 23, 59, 0, 0, loc)))
}
