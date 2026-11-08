import type { Claims } from './index'

export interface RolesOf {
  /** Roles outside any client: flat `roles`/`role` claims and `realm_access.roles`. */
  realm: string[]
  /** Roles per client from `resource_access.<client>.roles`. */
  clients: Record<string, string[]>
}

/**
 * Roles of the token, wherever providers put them: a flat `roles` or `role`
 * claim (string or list), Keycloak's `realm_access.roles` and
 * `resource_access.<client>.roles`. The same places the gateway's `jwt`
 * method looks at.
 */
export function rolesOf(claims: Claims | null | undefined): RolesOf {
  const result: RolesOf = { realm: [], clients: {} }
  if (!claims) return result

  const realm = new Set<string>()
  for (const value of [claims['roles'], claims['role'], record(claims['realm_access'])?.['roles']]) {
    for (const role of strings(value)) realm.add(role)
  }
  result.realm = [...realm]

  const resourceAccess = record(claims['resource_access'])
  if (resourceAccess) {
    for (const [client, raw] of Object.entries(resourceAccess)) {
      const roles = strings(record(raw)?.['roles'])
      if (roles.length) result.clients[client] = roles
    }
  }
  return result
}

/**
 * Whether the token carries one of the roles. `client:role` is a role of
 * that client only (`resource_access.<client>.roles`); a bare `role` matches
 * anywhere roles live, roles of any client included — the gateway's rule.
 */
export function hasRole(claims: Claims | null | undefined, ...wanted: string[]): boolean {
  if (!claims) return false
  const roles = rolesOf(claims)
  const anywhere = new Set([...roles.realm, ...Object.values(roles.clients).flat()])
  return wanted.some((role) => {
    const colon = role.indexOf(':')
    if (colon <= 0) return anywhere.has(role)
    return roles.clients[role.slice(0, colon)]?.includes(role.slice(colon + 1)) ?? false
  })
}

/** A display name for the user: the first of `preferred_username`, `upn`, `login`, `email`, `name`, `sub`. */
export function usernameOf(claims: Claims | null | undefined): string {
  if (!claims) return ''
  for (const key of ['preferred_username', 'upn', 'login', 'email', 'name', 'sub']) {
    const value = claims[key]
    if (typeof value === 'string' && value) return value
  }
  return ''
}

function record(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined
}

function strings(value: unknown): string[] {
  if (typeof value === 'string') return value.split(/\s+/).filter(Boolean)
  if (Array.isArray(value)) return value.filter((v): v is string => typeof v === 'string' && v !== '')
  return []
}
