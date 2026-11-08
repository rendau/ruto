package model

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"

	compressionModel "github.com/rendau/ruto/internal/domain/compression/model"
)

func (m *Root) Normalize() error {
	m.BaseUrl = strings.TrimRight(strings.TrimSpace(m.BaseUrl), "/")
	if err := m.Cors.Normalize(); err != nil {
		return fmt.Errorf("cors: %w", err)
	}
	providerNames := make(map[string]struct{}, len(m.Jwt))
	for i := range m.Jwt {
		if err := m.Jwt[i].Normalize(); err != nil {
			return fmt.Errorf("jwt[%d]: %w", i, err)
		}
		if m.Jwt[i].LoginEnabled() && m.Jwt[i].PublicUrl == "" && m.BaseUrl == "" {
			return fmt.Errorf("jwt[%d]: public_url: empty and root base_url is empty", i)
		}
		if name := m.Jwt[i].Name; name != "" {
			if _, ok := providerNames[name]; ok {
				return fmt.Errorf("jwt[%d]: name: duplicate %q", i, name)
			}
			providerNames[name] = struct{}{}
		}
	}
	if err := m.Auth.Normalize(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if err := m.Logging.Normalize(); err != nil {
		return fmt.Errorf("logging: %w", err)
	}
	if err := m.Variables.Normalize(); err != nil {
		return fmt.Errorf("variables: %w", err)
	}
	if m.Transform.MaxWorkers < 0 {
		m.Transform.MaxWorkers = 0
	}
	if err := m.Compression.Normalize(); err != nil {
		return fmt.Errorf("compression: %w", err)
	}
	m.Compression = compressionModel.Merge(compressionModel.Default(), m.Compression)
	for i := range m.Apps {
		if err := m.Apps[i].Normalize(); err != nil {
			return fmt.Errorf("apps[%d]: %w", i, err)
		}
	}
	return nil
}

func (m *RootCors) Normalize() error {
	m.MaxAge = strings.TrimSpace(m.MaxAge)
	if _, err := parseMaxAge(m.MaxAge); err != nil {
		return fmt.Errorf("max_age: %w", err)
	}
	m.AllowOrigins = lo.Filter(m.AllowOrigins, func(v string, _ int) bool {
		return strings.TrimSpace(v) != ""
	})
	for i, v := range m.AllowOrigins {
		origin, err := normalizeOrigin(v)
		if err != nil {
			return fmt.Errorf("allow_origins[%d]: %w", i, err)
		}
		m.AllowOrigins[i] = origin
	}
	m.AllowOrigins = lo.Uniq(m.AllowOrigins)
	m.AllowMethods = lo.FilterMap(m.AllowMethods, func(v string, _ int) (string, bool) {
		v = strings.ToUpper(strings.TrimSpace(v))
		return v, v != ""
	})
	m.AllowHeaders = lo.FilterMap(m.AllowHeaders, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	return nil
}

func (m *RootJwt) Normalize() error {
	var err error

	m.Name = strings.TrimSpace(m.Name)
	m.JwkUrl = strings.TrimSpace(m.JwkUrl)
	if m.Issuer, err = normalizeBaseUrl(m.Issuer); err != nil {
		return fmt.Errorf("issuer: %w", err)
	}
	if m.InternalUrl, err = normalizeBaseUrl(m.InternalUrl); err != nil {
		return fmt.Errorf("internal_url: %w", err)
	}
	if m.PublicUrl, err = normalizeBaseUrl(m.PublicUrl); err != nil {
		return fmt.Errorf("public_url: %w", err)
	}

	if m.Issuer == "" {
		if m.JwkUrl == "" {
			return fmt.Errorf("jwk_url: empty")
		}
		if m.Client != nil || m.InternalUrl != "" || m.PublicUrl != "" {
			return fmt.Errorf("issuer: empty, but login settings are set")
		}
		return nil
	}

	// Login routes and cookies are named after the provider.
	if m.Name == "" {
		return fmt.Errorf("name: empty, required with issuer")
	}
	if m.Client != nil {
		if err = m.Client.Normalize(); err != nil {
			return fmt.Errorf("client: %w", err)
		}
	}
	return nil
}

// normalizeBaseUrl accepts an empty value or an absolute http(s) URL without
// query and fragment, returned without the trailing slash.
func normalizeBaseUrl(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("host cannot be empty")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("query and fragment are not allowed")
	}
	return raw, nil
}

// MaxAgeSeconds is the Access-Control-Max-Age value: the header takes
// seconds only, while the config keeps max_age as the admin wrote it ("120h"
// reads better than "432000"). Empty when max_age is empty or malformed.
func (m *RootCors) MaxAgeSeconds() string {
	d, err := parseMaxAge(m.MaxAge)
	if err != nil || m.MaxAge == "" {
		return ""
	}
	return strconv.Itoa(int(d / time.Second))
}

// parseMaxAge accepts seconds ("86400") or a duration ("24h").
func parseMaxAge(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		if seconds < 0 {
			return 0, fmt.Errorf("must not be negative")
		}
		return time.Duration(seconds) * time.Second, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("must be seconds or a duration like 24h")
	}
	return d, nil
}
