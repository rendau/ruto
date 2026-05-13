package jwt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasAnyRole(t *testing.T) {
	tests := []struct {
		name          string
		claims        map[string]any
		requiredRoles []string
		want          bool
	}{
		{
			name:          "no required roles",
			claims:        map[string]any{},
			requiredRoles: nil,
			want:          true,
		},
		{
			name: "top-level roles array",
			claims: map[string]any{
				"roles": []any{"user", "admin"},
			},
			requiredRoles: []string{"admin"},
			want:          true,
		},
		{
			name: "top-level role string",
			claims: map[string]any{
				"role": "user manager",
			},
			requiredRoles: []string{"manager"},
			want:          true,
		},
		{
			name: "realm access roles",
			claims: map[string]any{
				"realm_access": map[string]any{
					"roles": []any{"viewer", "operator"},
				},
			},
			requiredRoles: []string{"operator"},
			want:          true,
		},
		{
			name: "resource access roles",
			claims: map[string]any{
				"resource_access": map[string]any{
					"client-a": map[string]any{"roles": []any{"reader"}},
					"client-b": map[string]any{"roles": []any{"writer"}},
				},
			},
			requiredRoles: []string{"writer"},
			want:          true,
		},
		{
			name: "role not found",
			claims: map[string]any{
				"roles": []any{"user"},
			},
			requiredRoles: []string{"admin"},
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAnyRole(tt.claims, tt.requiredRoles)
			require.Equal(t, tt.want, got)
		})
	}
}
