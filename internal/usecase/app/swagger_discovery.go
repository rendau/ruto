package app

import (
	"context"
	"errors"
	"net"
	"net/url"
	"path"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/rendau/ruto/internal/errs"
	swaggerService "github.com/rendau/ruto/internal/service/swagger"
)

const swaggerProbeTimeout = time.Second
const swaggerProbeWorkers = 4

// systemSwaggerPort — порт, на котором backend-сервисы по соглашению отдают
// системные ручки (включая swagger-документацию). Discovery дополнительно
// опрашивает тот же хост на этом порту.
const systemSwaggerPort = "3003"

var errStopSwaggerDiscovery = errors.New("stop swagger discovery")

func (u *Usecase) GetSwaggerURLByBackendURL(ctx context.Context, backendURL string) (string, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return "", errs.NotAuthorized
	}

	normalizedBaseURL, err := normalizeBaseURL(backendURL)
	if err != nil {
		return "", errs.InvalidRequest
	}

	return u.discoverSwaggerURL(ctx, buildSwaggerCandidates(normalizedBaseURL)), nil
}

func (u *Usecase) discoverSwaggerURL(ctx context.Context, candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}

	workers := swaggerProbeWorkers
	if workers > len(candidates) {
		workers = len(candidates)
	}
	if workers < 1 {
		workers = 1
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(workers)
	result := make(chan string, 1)

	for _, candidateURL := range candidates {
		group.Go(func() error {
			if groupCtx.Err() != nil {
				return nil
			}

			probeCtx, cancel := context.WithTimeout(groupCtx, swaggerProbeTimeout)
			endpoints, loadErr := u.swaggerSvc.LoadEndpoints(probeCtx, candidateURL)
			cancel()
			if loadErr != nil {
				if swaggerService.IsDialError(loadErr) {
					return errStopSwaggerDiscovery
				}
				return nil
			}

			if len(endpoints) > 0 {
				select {
				case result <- candidateURL:
				default:
				}
				return errStopSwaggerDiscovery
			}
			return nil
		})
	}

	_ = group.Wait()
	close(result)

	for candidateURL := range result {
		return candidateURL
	}

	return ""
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
	result := make([]string, 0, 80)

	appendCandidates := func(base *url.URL, prefixes []string) {
		for _, prefix := range prefixes {
			for _, suffix := range suffixes {
				itemURL := *base
				itemURL.Path = joinURLPaths(prefix, suffix)
				item := itemURL.String()
				if _, ok := seen[item]; ok {
					continue
				}
				seen[item] = struct{}{}
				result = append(result, item)
			}
		}
	}

	appendCandidates(baseURL, pathPrefixes(baseURL.Path))

	for _, sysBase := range systemPortBaseURLs(baseURL) {
		appendCandidates(sysBase, []string{"/"})
	}

	return result
}

// systemPortBaseURLs возвращает варианты base URL, указывающие на тот же хост,
// но на системный порт. Системные ручки не наследуют backend-путь, поэтому
// опрашиваются только от корня. Если backend уже на системном порту — вариантов
// нет. https-backend дополнительно пробуется по http, т.к. системный порт обычно
// отдаётся без TLS.
func systemPortBaseURLs(baseURL *url.URL) []*url.URL {
	host := baseURL.Hostname()
	if host == "" || baseURL.Port() == systemSwaggerPort {
		return nil
	}

	hostPort := net.JoinHostPort(host, systemSwaggerPort)

	schemes := []string{baseURL.Scheme}
	if baseURL.Scheme == "https" {
		schemes = append(schemes, "http")
	}

	result := make([]*url.URL, 0, len(schemes))
	for _, scheme := range schemes {
		item := *baseURL
		item.Scheme = scheme
		item.Host = hostPort
		item.Path = "/"
		result = append(result, &item)
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
