package model

import (
	"fmt"
	"strings"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
)

func (m *Compression) Normalize() error {
	m.Backend = strings.ToLower(strings.TrimSpace(m.Backend))
	if m.Backend != "" && !constant.CompressionBackendIsValid(m.Backend) {
		return fmt.Errorf("backend: is invalid")
	}

	m.Response = strings.ToLower(strings.TrimSpace(m.Response))
	if m.Response != "" && !constant.CompressionResponseIsValid(m.Response) {
		return fmt.Errorf("response: is invalid")
	}

	m.Algorithms = lo.Uniq(lo.FilterMap(m.Algorithms, func(v string, _ int) (string, bool) {
		v = strings.ToLower(strings.TrimSpace(v))
		return v, v != ""
	}))
	for _, v := range m.Algorithms {
		if !constant.CompressionAlgorithmIsValid(v) {
			return fmt.Errorf("algorithms: %q is not supported", v)
		}
	}

	if m.MinSize < 0 {
		return fmt.Errorf("min_size: must be >= 0")
	}

	m.ContentTypes = lo.Uniq(lo.FilterMap(m.ContentTypes, func(v string, _ int) (string, bool) {
		v = strings.ToLower(strings.TrimSpace(v))
		return v, v != ""
	}))
	for _, v := range m.ContentTypes {
		if !strings.Contains(v, "/") || strings.ContainsAny(v, " ;,") {
			return fmt.Errorf("content_types: %q is not a media type", v)
		}
	}

	return nil
}
