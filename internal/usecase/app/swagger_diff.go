package app

import (
	"slices"
	"strings"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	swaggerService "github.com/rendau/ruto/internal/service/swagger"
)

func buildSwaggerEndpointsDiff(swaggerEndpoints []swaggerService.Endpoint, registeredEndpoints []*endpointModel.Endpoint) *SwaggerEndpointsDiff {
	swaggerSet := make(map[string]SwaggerEndpoint, len(swaggerEndpoints))
	for _, item := range swaggerEndpoints {
		method := normalizeComparableMethod(item.Method)
		path := normalizeComparablePath(item.Path)
		if method == "" || path == "" {
			continue
		}
		swaggerSet[swaggerEndpointKey(method, normalizeComparablePathPattern(path))] = SwaggerEndpoint{
			Method: method,
			Path:   path,
		}
	}

	registeredSet := make(map[string]SwaggerEndpoint, len(registeredEndpoints))
	for _, item := range registeredEndpoints {
		method := normalizeComparableMethod(item.Http.Method)
		path := normalizeComparablePath(item.Http.Path)
		if method == "" || path == "" {
			continue
		}
		registeredSet[swaggerEndpointKey(method, normalizeComparablePathPattern(path))] = SwaggerEndpoint{
			Method: method,
			Path:   path,
		}
	}

	unregistered := make([]SwaggerEndpoint, 0)
	for key, item := range swaggerSet {
		if _, ok := registeredSet[key]; ok {
			continue
		}
		unregistered = append(unregistered, item)
	}
	slices.SortFunc(unregistered, func(a, b SwaggerEndpoint) int {
		if cmp := strings.Compare(a.Path, b.Path); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Method, b.Method)
	})

	registeredInvalid := make([]SwaggerEndpoint, 0)
	for key, item := range registeredSet {
		if _, ok := swaggerSet[key]; ok {
			continue
		}
		registeredInvalid = append(registeredInvalid, item)
	}
	slices.SortFunc(registeredInvalid, func(a, b SwaggerEndpoint) int {
		if cmp := strings.Compare(a.Path, b.Path); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Method, b.Method)
	})

	return &SwaggerEndpointsDiff{
		Unregistered:      unregistered,
		RegisteredInvalid: registeredInvalid,
	}
}

func swaggerEndpointKey(method, path string) string {
	return method + " " + path
}

func normalizeComparableMethod(method string) string {
	return strings.ToUpper(strings.TrimSpace(method))
}

func normalizeComparablePath(path string) string {
	p := strings.TrimSpace(path)
	p = strings.Trim(p, "/")
	if p == "" {
		return "/"
	}
	return "/" + p
}

func normalizeComparablePathPattern(path string) string {
	p := normalizeComparablePath(path)
	if p == "/" {
		return p
	}

	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	for i, part := range parts {
		parts[i] = normalizeComparablePathSegment(part)
	}

	return "/" + strings.Join(parts, "/")
}

func normalizeComparablePathSegment(segment string) string {
	if isPathVariableSegment(segment) {
		return "{}"
	}
	return segment
}

func isPathVariableSegment(segment string) bool {
	s := strings.TrimSpace(segment)
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, ":") && len(s) > 1 {
		return true
	}
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && len(s) > 2 {
		return true
	}
	return false
}
