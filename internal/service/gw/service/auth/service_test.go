package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/service/gw/service/auth/authorizer"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

type stubAuthorizer struct {
	result model.Result
}

func (a *stubAuthorizer) Authorize(*model.AuthRequest) model.Result {
	return a.result
}

func newStubMethod(results ...model.Result) *method {
	authorizers := make([]authorizer.Authorizer, 0, len(results))
	for _, r := range results {
		authorizers = append(authorizers, &stubAuthorizer{result: r})
	}
	return &method{authorizers: authorizers}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		methods []*method
		want    model.Result
	}{
		{
			name:    "any method allows",
			methods: []*method{newStubMethod(model.InvalidCredentials), newStubMethod(model.Allowed)},
			want:    model.Allowed,
		},
		{
			name:    "no credentials",
			methods: []*method{newStubMethod(model.NoCredentials)},
			want:    model.NoCredentials,
		},
		{
			name:    "ip-only method denies",
			methods: []*method{newStubMethod(model.Denied)},
			want:    model.Denied,
		},
		{
			name:    "valid credentials but ip denied in the same method",
			methods: []*method{newStubMethod(model.Allowed, model.Denied)},
			want:    model.DeniedAuthenticated,
		},
		{
			name:    "missing credentials of another method beat ip denial",
			methods: []*method{newStubMethod(model.Denied), newStubMethod(model.NoCredentials)},
			want:    model.NoCredentials,
		},
		{
			name:    "authenticated denial beats missing credentials of another method",
			methods: []*method{newStubMethod(model.DeniedAuthenticated), newStubMethod(model.NoCredentials)},
			want:    model.DeniedAuthenticated,
		},
		{
			name:    "invalid credentials beat ip denial",
			methods: []*method{newStubMethod(model.Denied), newStubMethod(model.InvalidCredentials)},
			want:    model.InvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{methods: tt.methods}
			require.Equal(t, tt.want, s.Check(model.NewAuthRequest()))
		})
	}
}
