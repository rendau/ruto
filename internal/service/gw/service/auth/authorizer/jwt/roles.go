package jwt

import "strings"

// hasAnyRole reports whether any role anywhere roles live in the token passes
// requiredRoleCheck: a flat "roles"/"role" claim, the realm roles and the
// roles of every client (Keycloak's realm_access / resource_access).
func hasAnyRole(claims map[string]any, requiredRoleCheck func(role string) bool) bool {
	return visitRoles(claims, requiredRoleCheck)
}

// hasClientRole reports whether any role of client (resource_access.<client>
// .roles) passes requiredRoleCheck. Roles of other clients do not count: with
// a shared login the token carries the roles of every app.
func hasClientRole(claims map[string]any, client string, requiredRoleCheck func(role string) bool) bool {
	resourceAccess, ok := claims["resource_access"].(map[string]any)
	if !ok {
		return false
	}
	clientRoles, ok := resourceAccess[client].(map[string]any)
	if !ok {
		return false
	}
	return visitAny(clientRoles["roles"], requiredRoleCheck)
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
		var clientRoles map[string]any
		for _, raw := range resourceAccess {
			clientRoles, ok = raw.(map[string]any)
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
		for value := range strings.FieldsSeq(typed) {
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
