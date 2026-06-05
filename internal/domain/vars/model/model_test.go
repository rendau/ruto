package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVarsFillMissing_InitializeNilReceiver(t *testing.T) {
	var item Vars

	item.FillMissing(Vars{
		"env":  "prod",
		"zone": "kz",
	})

	require.NotNil(t, item, "FillMissing() must initialize nil receiver")
	assert.Equal(t, "prod", item["env"])
	assert.Equal(t, "kz", item["zone"])
}

func TestVarsFillMissing_DoesNotOverrideExistingAndRespectsInputOrder(t *testing.T) {
	item := Vars{
		"env": "local",
	}

	item.FillMissing(
		Vars{
			"env":  "dev",
			"host": "first-host",
		},
		Vars{
			"host": "second-host",
			"port": "5050",
		},
	)

	assert.Len(t, item, 3, "all keys must be present")
	assert.Equal(t, "local", item["env"], "existing key must not be overridden")
	assert.Equal(t, "first-host", item["host"], "first input vars must win on conflict")
	assert.Equal(t, "5050", item["port"], "missing key must be filled")
}

func TestVarsFillMissing_AllowsNilInputVars(t *testing.T) {
	item := Vars{
		"env": "prod",
	}

	item.FillMissing(nil, Vars{}, Vars{"region": "ap-south"})

	require.Len(t, item, 2)
	assert.Equal(t, "prod", item["env"])
	assert.Equal(t, "ap-south", item["region"])
}

func TestVars_Clone(t *testing.T) {
	original := Vars{
		"env":  "prod",
		"zone": "kz",
	}

	cloned := original.Clone()
	require.Equal(t, original, cloned)

	cloned["env"] = "dev"
	assert.Equal(t, "prod", original["env"])
	assert.Equal(t, "dev", cloned["env"])
}

func TestVars_InterpolateString(t *testing.T) {
	tests := []struct {
		name     string
		vars     Vars
		input    string
		expected string
	}{
		{
			name:     "no interpolation",
			vars:     Vars{"foo": "bar"},
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "single interpolation",
			vars:     Vars{"name": "Alice"},
			input:    "hello {{name}}",
			expected: "hello Alice",
		},
		{
			name:     "multiple interpolation",
			vars:     Vars{"first": "Alice", "last": "Smith"},
			input:    "{{first}} {{last}}",
			expected: "Alice Smith",
		},
		{
			name:     "missing variable",
			vars:     Vars{"known": "value"},
			input:    "{{known}} and {{unknown}}",
			expected: "value and {{unknown}}",
		},
		{
			name:     "complex patterns",
			vars:     Vars{"a.b-c_1": "success"},
			input:    "result: {{a.b-c_1}}",
			expected: "result: success",
		},
		{
			name:     "empty input",
			vars:     Vars{"foo": "bar"},
			input:    "",
			expected: "",
		},
		{
			name:     "repeated variables",
			vars:     Vars{"val": "1"},
			input:    "{{val}}+{{val}}={{val}}{{val}}",
			expected: "1+1=11",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.vars.InterpolateString(tt.input))
		})
	}
}

func TestVars_InterpolateVars(t *testing.T) {
	m := Vars{
		"base_url": "https://api.example.com",
		"version":  "v1",
		"user_id":  "123",
	}

	input := Vars{
		"endpoint":         "{{base_url}}/{{version}}/users",
		"user_{{user_id}}": "profile",
	}

	expected := Vars{
		"endpoint": "https://api.example.com/v1/users",
		"user_123": "profile",
	}

	result := m.InterpolateVars(input)
	assert.Equal(t, expected, result)
}

func TestVars_InterpolateStrings(t *testing.T) {
	m := Vars{
		"env": "prod",
		"id":  "42",
	}

	input := []string{
		"app-{{env}}",
		"container-{{id}}",
		"no-change",
	}

	expected := []string{
		"app-prod",
		"container-42",
		"no-change",
	}

	result := m.InterpolateStrings(input)
	assert.Equal(t, expected, result)
}
