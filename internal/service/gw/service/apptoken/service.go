// Package apptoken gets app tokens: the shared access token of a provider
// exchanged (RFC 8693) for a token with an app's audience, on behalf of the
// provider's default client. It knows the providers of the snapshot, not apps,
// cookies or requests.
package apptoken

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/service/oidc"
)

var (
	// ErrTokenRejected — the provider does not take the shared token any more
	// (the session is over): the client must refresh it or log in.
	ErrTokenRejected = errors.New("apptoken: the shared token is rejected")
	// ErrRefused — the provider refused the app token on the merits: the user
	// has no access to the audience, or the audience is not allowed.
	ErrRefused = errors.New("apptoken: the app token is refused")
	// ErrUnavailable — the provider cannot be reached, answered with an error
	// of its own or is not configured for exchanges.
	ErrUnavailable = errors.New("apptoken: the provider is unavailable")
)

var instance = New()

func Ins() *Service {
	return instance
}

type provider struct {
	issuer      string
	internalUrl string
	client      *authModel.JwtClient
	scopes      []string
	secure      bool
}

type Service struct {
	mu        sync.RWMutex
	providers map[string]*provider // by name
}

func New() *Service {
	return &Service{providers: map[string]*provider{}}
}

// SetProviders takes the login providers of a normalized and interpolated
// snapshot (the client secret may come from variables).
func (s *Service) SetProviders(snapshot *rootModel.Root) {
	providers := make(map[string]*provider, len(snapshot.Jwt))
	for i := range snapshot.Jwt {
		p := &snapshot.Jwt[i]
		if !p.LoginEnabled() || p.Client == nil {
			continue
		}
		publicUrl := p.PublicUrl
		if publicUrl == "" {
			publicUrl = snapshot.BaseUrl
		}
		providers[p.Name] = &provider{
			issuer:      p.Issuer,
			internalUrl: p.InternalUrl,
			client:      p.Client.Clone(),
			scopes:      append([]string(nil), p.ExchangeScopes...),
			secure:      strings.HasPrefix(publicUrl, "https://"),
		}
	}
	s.mu.Lock()
	s.providers = providers
	s.mu.Unlock()
}

func (s *Service) provider(name string) *provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.providers[name]
}

// CookieSecure tells whether the cookies of the provider's tokens are Secure:
// the gateway serves its login over https. The same rule as the login cookies.
func (s *Service) CookieSecure(name string) bool {
	if p := s.provider(name); p != nil {
		return p.secure
	}
	return false
}

// Exchange trades subjectToken, the shared access token of the provider
// called name, for an access token meant for audience. The error is one of
// ErrTokenRejected, ErrRefused, ErrUnavailable, wrapped with the details.
func (s *Service) Exchange(ctx context.Context, name, subjectToken, audience string) (string, error) {
	p := s.provider(name)
	if p == nil {
		return "", fmt.Errorf("%w: %q is not a login provider with a client", ErrUnavailable, name)
	}
	meta, err := oidc.Ins().Metadata(ctx, p.issuer, p.internalUrl)
	if err != nil {
		return "", fmt.Errorf("%w: discovery: %w", ErrUnavailable, err)
	}
	rep, err := oidc.Ins().TokenExchange(ctx, meta.TokenEndpoint, p.client, subjectToken, audience, p.scopes)
	if err != nil {
		return "", classify(name, audience, err)
	}
	return rep.AccessToken, nil
}

// classify sorts a failed exchange by what the client should do. A refusal
// about the gateway's own request (its client, its grant) is a configuration
// problem, not the user's: it is logged as an error and answered as an
// unavailable provider.
func classify(name, audience string, err error) error {
	refusal, ok := errors.AsType[*oidc.Error](err)
	if !ok || refusal.Status >= http.StatusInternalServerError {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	switch refusal.Code {
	case "invalid_grant", "invalid_token":
		return fmt.Errorf("%w: %w", ErrTokenRejected, err)
	case "invalid_client", "unauthorized_client", "unsupported_grant_type":
		slog.Error("apptoken: the provider does not let the gateway exchange tokens, check its client",
			"provider", name, "audience", audience, "error", err)
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return fmt.Errorf("%w: %w", ErrRefused, err)
}
