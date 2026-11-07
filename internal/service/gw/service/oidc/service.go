// Package oidc talks to OpenID Connect providers on the gateway's behalf:
// reads their discovery documents and exchanges or refreshes tokens at their
// token endpoint. It knows nothing about cookies, apps or requests.
package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/rendau/ruto/internal/infra/httpx"
)

const (
	// MetadataTTL is how long a discovery document is trusted before it is
	// read again; providers change their endpoints about never.
	MetadataTTL      = time.Hour
	discoveryPath    = "/.well-known/openid-configuration"
	requestTimeout   = 10 * time.Second
	maxResponseBytes = 1 << 20
)

var instance = New()

func Ins() *Service {
	return instance
}

type Service struct {
	httpClient *http.Client
	mu         sync.Mutex
	entries    map[string]*entry // by discovery url
	refreshes  singleflight.Group
}

type entry struct {
	mu       sync.Mutex
	meta     *Metadata
	loadedAt time.Time
}

func New() *Service {
	return &Service{
		httpClient: httpx.New(httpx.Config{Timeout: requestTimeout}),
		entries:    make(map[string]*entry),
	}
}

// Metadata returns the provider's endpoints. issuer is the public address the
// provider names itself by; internalUrl, when not empty, stands for the same
// address from inside the cluster: the discovery document is read from it and
// the endpoints the gateway calls are moved onto it. A document is cached for
// MetadataTTL; when a reload fails the previous document is kept.
func (s *Service) Metadata(ctx context.Context, issuer, internalUrl string) (*Metadata, error) {
	base := issuer
	if internalUrl != "" {
		base = internalUrl
	}
	discoveryUrl := base + discoveryPath

	e := s.entry(discoveryUrl)
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.meta != nil && time.Since(e.loadedAt) < MetadataTTL {
		return e.meta, nil
	}

	meta, err := s.load(ctx, discoveryUrl, issuer, internalUrl)
	if err != nil {
		if e.meta != nil {
			slog.Warn("oidc: discovery reload failed, keeping the previous document",
				"error", err, "url", discoveryUrl)
			e.loadedAt = time.Now() // do not hammer a failing provider on every call
			return e.meta, nil
		}
		return nil, err
	}

	e.meta = meta
	e.loadedAt = time.Now()
	return meta, nil
}

func (s *Service) entry(discoveryUrl string) *entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[discoveryUrl]
	if !ok {
		e = &entry{}
		s.entries[discoveryUrl] = e
	}
	return e
}

func (s *Service) load(ctx context.Context, discoveryUrl, issuer, internalUrl string) (*Metadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	body, status, err := s.do(req)
	if err != nil {
		return nil, fmt.Errorf("discovery %s: %w", discoveryUrl, err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("discovery %s: status %d", discoveryUrl, status)
	}

	meta := new(Metadata)
	if err = json.Unmarshal(body, meta); err != nil {
		return nil, fmt.Errorf("discovery %s: json.Unmarshal: %w", discoveryUrl, err)
	}

	// The document must describe the provider we were configured with: a
	// wrong address or a proxy to another realm would otherwise go unnoticed.
	if strings.TrimRight(meta.Issuer, "/") != issuer {
		return nil, fmt.Errorf("discovery %s: issuer %q does not match configured %q", discoveryUrl, meta.Issuer, issuer)
	}
	if meta.AuthorizationEndpoint == "" || meta.TokenEndpoint == "" || meta.JwksUri == "" {
		return nil, fmt.Errorf("discovery %s: authorization_endpoint, token_endpoint and jwks_uri are required", discoveryUrl)
	}

	meta.TokenEndpoint = rebase(meta.TokenEndpoint, issuer, internalUrl)
	meta.JwksUri = rebase(meta.JwksUri, issuer, internalUrl)

	return meta, nil
}

// rebase moves an endpoint under issuer onto internalUrl, so the gateway
// reaches the provider the short way. Endpoints elsewhere are left alone.
func rebase(endpoint, issuer, internalUrl string) string {
	if internalUrl == "" {
		return endpoint
	}
	if rest, ok := strings.CutPrefix(endpoint, issuer); ok && (rest == "" || rest[0] == '/') {
		return internalUrl + rest
	}
	return endpoint
}

func (s *Service) do(req *http.Request) ([]byte, int, error) {
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http.Client.Do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("io.ReadAll: %w", err)
	}
	return body, resp.StatusCode, nil
}
