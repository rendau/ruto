package logmask

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskValues(t *testing.T) {
	sensitive := map[string]struct{}{"authorization": {}, "x-api-key": {}}
	in := map[string][]string{
		"Authorization": {"Bearer abc"},
		"X-Api-Key":     {"key"},
		"Cookie":        {"session=abc"},
		"Set-Cookie":    {"a=b"},
		"Accept":        {"application/json"},
	}

	assert.Equal(t, map[string][]string{
		"Authorization": {maskedValue},
		"X-Api-Key":     {maskedValue},
		"Accept":        {"application/json"},
	}, MaskValues(in, sensitive))
}
