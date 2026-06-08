package logmask

import (
	"strings"

	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

// defaultSensitiveKeys are always masked in logged headers, gRPC metadata and
// query params.
var defaultSensitiveKeys = []string{"authorization", "api-key"}

const maskedValue = "***"

// BuildSensitiveKeySet returns the lower-cased set of header/metadata/query keys
// whose values must be masked when logged: the always-sensitive defaults plus
// any API-key header names configured on the endpoint's auth.
func BuildSensitiveKeySet(ep *domEndpointModel.Endpoint) map[string]struct{} {
	set := make(map[string]struct{}, len(defaultSensitiveKeys)+len(ep.Auth.Methods))
	for _, k := range defaultSensitiveKeys {
		set[k] = struct{}{}
	}
	// Auth is normalized at this point: every API-key method has a non-empty
	// header (defaulted to X-Api-Key), so no nil/empty guards are needed.
	for _, m := range ep.Auth.Methods {
		if m.APIKey != nil {
			set[strings.ToLower(m.APIKey.Header)] = struct{}{}
		}
	}
	return set
}

// MaskValues copies a header/metadata/query-param map, replacing the values of
// sensitive keys with a mask. http.Header, url.Values and metadata.MD all share
// this underlying type.
func MaskValues(in map[string][]string, sensitive map[string]struct{}) map[string][]string {
	out := make(map[string][]string, len(in))
	for k, v := range in {
		if _, ok := sensitive[strings.ToLower(k)]; ok {
			out[k] = []string{maskedValue}
			continue
		}
		out[k] = v
	}
	return out
}
