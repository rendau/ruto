// Package oidc reads the discovery documents of OpenID Connect providers: the
// gateway takes the address of a provider's keys (jwks_uri) from there when the
// provider is configured by its issuer.
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

	"github.com/rendau/ruto/internal/infra/httpx"
)

const (
	// MetadataTTL is how long a discovery document is trusted before it is
	// read again. A provider's setting can move jwks_uri (an IdP realm that
	// stops sharing the root JWKS), and the old address may keep answering
	// with an empty set, so the document is not trusted for longer than the
	// gateway polls the keys.
	MetadataTTL      = 10 * time.Minute
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
}

type entry struct {
	mu       sync.Mutex
	meta     *Metadata
	loadedAt time.Time
}

func New() *Service {
	return &Service{
		// Read every few minutes: an idle connection would be closed by then anyway.
		httpClient: httpx.New(httpx.Config{Timeout: requestTimeout, DisableKeepAlives: true}),
		entries:    make(map[string]*entry),
	}
}

// Metadata returns the provider's discovery document. issuer is the public
// address the provider names itself by; internalUrl, when not empty, stands
// for the same address from inside the cluster: the document is read from it
// and jwks_uri is moved onto it. A document is cached for MetadataTTL; when a
// reload fails the previous document is kept.
func (s *Service) Metadata(ctx context.Context, issuer, internalUrl string) (*Metadata, error) {
	discoveryUrl := discoveryUrlOf(issuer, internalUrl)

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

// Forget drops the cached document of the provider: the next Metadata call
// reads it again, whatever its age.
func (s *Service) Forget(issuer, internalUrl string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, discoveryUrlOf(issuer, internalUrl))
}

func discoveryUrlOf(issuer, internalUrl string) string {
	if internalUrl != "" {
		return internalUrl + discoveryPath
	}
	return issuer + discoveryPath
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

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discovery %s: %w", discoveryUrl, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery %s: status %d", discoveryUrl, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("discovery %s: io.ReadAll: %w", discoveryUrl, err)
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
	if meta.JwksUri == "" {
		return nil, fmt.Errorf("discovery %s: jwks_uri is required", discoveryUrl)
	}

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
