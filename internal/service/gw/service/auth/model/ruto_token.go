package model

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync/atomic"
)

// rutoKids are the key ids the gateway signs its own tokens with (the OpenID
// Connect broker for fronts). Empty until the gateway gets signing keys.
var rutoKids atomic.Pointer[map[string]struct{}]

// SetRutoKids replaces the key ids of the gateway's own tokens.
func SetRutoKids(kids []string) {
	set := make(map[string]struct{}, len(kids))
	for _, kid := range kids {
		if kid != "" {
			set[kid] = struct{}{}
		}
	}
	rutoKids.Store(&set)
}

// IsRutoToken tells a token of the gateway itself by the kid in its header.
// The signature is not checked: such a token is never taken as credentials,
// only recognized to be set aside.
func IsRutoToken(token string) bool {
	set := rutoKids.Load()
	if set == nil || len(*set) == 0 {
		return false
	}
	header, _, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(header)
	if err != nil {
		return false
	}
	var parsed struct {
		Kid string `json:"kid"`
	}
	if json.Unmarshal(raw, &parsed) != nil || parsed.Kid == "" {
		return false
	}
	_, found := (*set)[parsed.Kid]
	return found
}
