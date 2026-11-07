package model

import (
	"fmt"
	"net/url"
	"strings"
)

// AnyOrigin is the allow_origins entry that admits every origin.
const AnyOrigin = "*"

const originWildcard = "*."

// normalizeOrigin brings one allow_origins entry to its canonical form: "*",
// an exact origin "https://app.example.com[:port]" or a subdomain pattern
// "https://*.example.com[:port]". Scheme and host are lower case, a path is
// not allowed.
func normalizeOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == AnyOrigin {
		return raw, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("must be an origin: scheme://host[:port]")
	}
	hostname := parsed.Hostname()
	if strings.Contains(hostname, "*") && (!strings.HasPrefix(hostname, originWildcard) ||
		strings.Contains(hostname[len(originWildcard):], "*") || len(hostname) == len(originWildcard)) {
		return "", fmt.Errorf("a wildcard may only replace the leftmost label: scheme://*.domain")
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host), nil
}

// OriginMatcher answers whether a browser origin is admitted by a normalized
// allow_origins list. Built once per snapshot; Matches runs on the hot path.
type OriginMatcher struct {
	any   bool
	exact map[string]struct{}
	// patterns are the subdomain entries split into "scheme://" and
	// ".domain[:port]": an origin matches when it has the scheme and ends
	// with the suffix with at least one more label in front.
	patterns []originPattern
}

type originPattern struct {
	scheme string
	suffix string
}

func NewOriginMatcher(origins []string) *OriginMatcher {
	m := &OriginMatcher{exact: make(map[string]struct{}, len(origins))}
	for _, origin := range origins {
		scheme, host, _ := strings.Cut(origin, "://")
		switch {
		case origin == AnyOrigin:
			m.any = true
		case strings.HasPrefix(host, originWildcard):
			m.patterns = append(m.patterns, originPattern{scheme: scheme + "://", suffix: host[len(originWildcard)-1:]})
		default:
			m.exact[origin] = struct{}{}
		}
	}
	return m
}

// Any reports whether "*" is among the entries.
func (m *OriginMatcher) Any() bool {
	return m.any
}

// Listed reports whether origin is named by an exact entry or a subdomain
// pattern; "*" is ignored. This is what a login return address must satisfy,
// since a wildcard there would make it an open redirect.
func (m *OriginMatcher) Listed(origin string) bool {
	origin = strings.ToLower(origin) // no allocation for an already lower-case origin
	if _, ok := m.exact[origin]; ok {
		return true
	}
	for _, p := range m.patterns {
		host, ok := strings.CutPrefix(origin, p.scheme)
		if ok && len(host) > len(p.suffix) && strings.HasSuffix(host, p.suffix) {
			return true
		}
	}
	return false
}
