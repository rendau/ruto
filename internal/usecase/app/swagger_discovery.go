package app

import (
	"context"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rendau/ruto/internal/errs"
)

const swaggerProbeTimeout = 1500 * time.Millisecond

func (u *Usecase) GetSwaggerURLByBackendURL(ctx context.Context, backendURL string) (string, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return "", errs.NotAuthorized
	}

	normalizedBaseURL, err := normalizeBaseURL(backendURL)
	if err != nil {
		return "", errs.InvalidRequest
	}

	for _, candidateURL := range buildSwaggerCandidates(normalizedBaseURL) {
		probeCtx, cancel := context.WithTimeout(ctx, swaggerProbeTimeout)
		endpoints, loadErr := u.swaggerSvc.LoadEndpoints(probeCtx, candidateURL)
		cancel()
		if loadErr == nil && len(endpoints) > 0 {
			return candidateURL, nil
		}
	}

	return "", nil
}

func normalizeBaseURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errs.InvalidRequest
	}

	result, err := url.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	if result.Scheme == "" || result.Host == "" {
		return nil, errs.InvalidRequest
	}

	result.RawQuery = ""
	result.Fragment = ""
	if result.Path == "" {
		result.Path = "/"
	}

	return result, nil
}

func buildSwaggerCandidates(baseURL *url.URL) []string {
	if baseURL == nil {
		return nil
	}

	suffixes := []string{
		"",
		"/swagger.json",
		"/api.swagger.json",
		"/swagger.yaml",
		"/swagger.yml",
		"/api.swagger.yaml",
		"/api.swagger.yml",
		"/openapi.json",
		"/openapi.yaml",
		"/openapi.yml",
		"/doc",
		"/docs",
		"/swagger",
		"/openapi",
		"/api-docs",
		"/v3/api-docs",
		"/doc/swagger.json",
		"/doc/api.swagger.json",
		"/doc/swagger.yaml",
		"/doc/swagger.yml",
		"/doc/api.swagger.yaml",
		"/doc/api.swagger.yml",
		"/docs/swagger.json",
		"/docs/api.swagger.json",
		"/docs/swagger.yaml",
		"/docs/swagger.yml",
		"/docs/api.swagger.yaml",
		"/docs/api.swagger.yml",
		"/swagger/swagger.json",
		"/swagger/api.swagger.json",
		"/swagger/swagger.yaml",
		"/swagger/swagger.yml",
		"/swagger/api.swagger.yaml",
		"/swagger/api.swagger.yml",
		"/openapi/openapi.json",
		"/openapi/openapi.yaml",
		"/openapi/openapi.yml",
		"/api-docs/swagger.json",
		"/api-docs/swagger.yaml",
		"/api-docs/swagger.yml",
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, 40)
	for _, prefix := range pathPrefixes(baseURL.Path) {
		for _, suffix := range suffixes {
			itemURL := *baseURL
			itemURL.Path = joinURLPaths(prefix, suffix)
			item := itemURL.String()
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}

	return result
}

func pathPrefixes(rawPath string) []string {
	normalized := "/" + strings.Trim(rawPath, "/")
	if normalized == "/" {
		return []string{"/"}
	}
	return []string{normalized, "/"}
}

func joinURLPaths(prefix, suffix string) string {
	left := "/" + strings.Trim(prefix, "/")
	right := strings.TrimSpace(suffix)
	if right == "" {
		return left
	}
	return path.Clean(strings.TrimSuffix(left, "/") + "/" + strings.TrimPrefix(right, "/"))
}
