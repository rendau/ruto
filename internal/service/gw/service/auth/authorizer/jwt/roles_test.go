package jwt

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

type requiredRoleStoreT struct {
	roles map[string]bool
}

func newRequiredRoleStore(roles []string) *requiredRoleStoreT {
	return &requiredRoleStoreT{
		roles: lo.SliceToMap(roles, func(role string) (string, bool) {
			return role, true
		}),
	}
}

func (r requiredRoleStoreT) hasRole(role string) bool {
	return r.roles[role]
}

func TestHasAnyRole(t *testing.T) {
	tests := []struct {
		name          string
		claims        map[string]any
		requiredRoles []string
		want          bool
	}{
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
			rrStore := newRequiredRoleStore(tt.requiredRoles)
			got := hasAnyRole(tt.claims, rrStore.hasRole)
			require.Equal(t, tt.want, got)
		})
	}
}
