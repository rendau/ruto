package swagger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

type Service struct {
	httpClient *http.Client
}

type Endpoint struct {
	Method string
	Path   string
}

type document struct {
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

func New(timeout time.Duration) *Service {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Service{
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (s *Service) LoadEndpoints(ctx context.Context, swaggerURL string) ([]Endpoint, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, swaggerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequestWithContext: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.Client.Do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("bad status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var raw document
	if err = json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("json.Decode: %w", err)
	}

	return parseEndpoints(raw), nil
}

func parseEndpoints(doc document) []Endpoint {
	result := make([]Endpoint, 0, len(doc.Paths))
	seen := make(map[string]struct{}, len(doc.Paths)*2)

	for path, operations := range doc.Paths {
		normalizedPath := normalizePath(path)
		if normalizedPath == "" {
			continue
		}
		for method := range operations {
			normalizedMethod := normalizeMethod(method)
			if !isSupportedHTTPMethod(normalizedMethod) {
				continue
			}
			key := normalizedMethod + " " + normalizedPath
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, Endpoint{
				Method: normalizedMethod,
				Path:   normalizedPath,
			})
		}
	}

	slices.SortFunc(result, func(a, b Endpoint) int {
		if cmp := strings.Compare(a.Path, b.Path); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Method, b.Method)
	})
	return result
}

func normalizeMethod(method string) string {
	return strings.ToUpper(strings.TrimSpace(method))
}

func normalizePath(path string) string {
	p := strings.TrimSpace(path)
	p = strings.Trim(p, "/")
	if p == "" {
		return "/"
	}
	return "/" + p
}

func isSupportedHTTPMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "CONNECT", "TRACE":
		return true
	default:
		return false
	}
}
