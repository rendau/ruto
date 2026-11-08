import { describe, expect, it } from 'vitest'
import { hasRole, rolesOf, usernameOf } from '../src/index'

const keycloak = {
  sub: 'u1',
  preferred_username: 'alice',
  realm_access: { roles: ['auditor', 'offline_access'] },
  resource_access: { shop: { roles: ['manager'] }, account: { roles: ['view-profile'] } },
}

describe('rolesOf', () => {
  it('collects realm and client roles of a Keycloak token', () => {
    expect(rolesOf(keycloak)).toEqual({
      realm: ['auditor', 'offline_access'],
      clients: { shop: ['manager'], account: ['view-profile'] },
    })
  })

  it('reads flat roles as a list or a space separated string', () => {
    expect(rolesOf({ roles: ['a', 'b'], role: 'c d' }).realm).toEqual(['a', 'b', 'c', 'd'])
    expect(rolesOf({ roles: 'x' })).toEqual({ realm: ['x'], clients: {} })
  })

  it('ignores junk', () => {
    expect(rolesOf(null)).toEqual({ realm: [], clients: {} })
    expect(rolesOf({ roles: 5, realm_access: 'no', resource_access: { shop: 'no', x: { roles: [1, ''] } } })).toEqual({
      realm: [],
      clients: {},
    })
  })
})

describe('hasRole', () => {
  it('a bare role matches anywhere, client:role only in that client', () => {
    expect(hasRole(keycloak, 'auditor')).toBe(true)
    expect(hasRole(keycloak, 'manager')).toBe(true)
    expect(hasRole(keycloak, 'shop:manager')).toBe(true)
    expect(hasRole(keycloak, 'account:manager')).toBe(false)
    expect(hasRole(keycloak, 'admin')).toBe(false)
    expect(hasRole(keycloak, 'admin', 'shop:manager')).toBe(true)
    expect(hasRole(null, 'auditor')).toBe(false)
  })
})

describe('usernameOf', () => {
  it('prefers preferred_username, falls back down to sub', () => {
    expect(usernameOf(keycloak)).toBe('alice')
    expect(usernameOf({ upn: 'bob@corp', sub: 'u2' })).toBe('bob@corp')
    expect(usernameOf({ sub: 'u3' })).toBe('u3')
    expect(usernameOf(null)).toBe('')
  })
})
