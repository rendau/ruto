package jwt

import "strings"

func hasAnyRole(claims map[string]any, requiredRoles []string) bool {
	if len(requiredRoles) == 0 {
		return true
	}

	required := make(map[string]struct{}, len(requiredRoles))
	for _, role := range requiredRoles {
		required[role] = struct{}{}
	}

	return visitRoles(claims, func(role string) bool {
		_, ok := required[role]
		return ok
	})
}

func visitRoles(claims map[string]any, fn func(role string) bool) bool {
	if visitAny(claims["roles"], fn) {
		return true
	}

	if visitAny(claims["role"], fn) {
		return true
	}

	if realmAccess, ok := claims["realm_access"].(map[string]any); ok {
		if visitAny(realmAccess["roles"], fn) {
			return true
		}
	}

	if resourceAccess, ok := claims["resource_access"].(map[string]any); ok {
		for _, raw := range resourceAccess {
			clientRoles, ok := raw.(map[string]any)
			if !ok {
				continue
			}

			if visitAny(clientRoles["roles"], fn) {
				return true
			}
		}
	}

	return false
}

func visitAny(values any, fn func(role string) bool) bool {
	switch typed := values.(type) {
	case string:
		for _, value := range strings.Fields(typed) {
			if fn(value) {
				return true
			}
		}

	case []string:
		for _, value := range typed {
			if fn(value) {
				return true
			}
		}

	case []any:
		for _, value := range typed {
			str, ok := value.(string)
			if !ok {
				continue
			}

			if fn(str) {
				return true
			}
		}
	}

	return false
}
