package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactedValues(t *testing.T) {
	src := Vars{"a": "1", "b": "2"}

	got := RedactedValues(src)

	require.Equal(t, Vars{"a": RedactedPlaceholder, "b": RedactedPlaceholder}, got)
	// original is left untouched
	require.Equal(t, Vars{"a": "1", "b": "2"}, src)
}

func TestRedactedValues_Empty(t *testing.T) {
	require.Empty(t, RedactedValues(nil))
	require.Empty(t, RedactedValues(Vars{}))
}
