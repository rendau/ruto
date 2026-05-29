package swagger

import (
	"slices"
	"strings"
)

type document struct {
	Paths map[string]map[string]any `json:"paths" yaml:"paths"`
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
