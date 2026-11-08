import { afterEach, describe, expect, it, vi } from 'vitest'
import { createRutoAuth, loginError, statusOf } from '../src/index'
import { fakeFetch, json, stubLocation } from './helpers'

afterEach(() => vi.unstubAllGlobals())

describe('urls', () => {
  it('shared login: routes under /_auth/<provider>, me there too', () => {
    const auth = createRutoAuth({ gateway: 'https://api.mdev.kz/', provider: 'corp' })
    expect(auth.base).toBe('https://api.mdev.kz/_auth/corp')
    expect(auth.meUrl).toBe('https://api.mdev.kz/_auth/corp/me')
    expect(auth.refreshUrl).toBe('https://api.mdev.kz/_auth/corp/refresh')
    expect(auth.loginUrl('https://shop.example.com/x')).toBe(
      'https://api.mdev.kz/_auth/corp/login?redirect=https%3A%2F%2Fshop.example.com%2Fx',
    )
    expect(auth.logoutUrl('/')).toBe('https://api.mdev.kz/_auth/corp/logout?redirect=%2F')
  })

  it("app's own client: routes under /_auth/<provider>/<app>, me inside the app prefix", () => {
    const auth = createRutoAuth({ gateway: 'https://api.mdev.kz', provider: 'corp', app: 'orders/' })
    expect(auth.base).toBe('https://api.mdev.kz/_auth/corp/orders')
    expect(auth.meUrl).toBe('https://api.mdev.kz/orders/_auth/corp/me')
  })

  it('login and logout default to the current page and the site root', () => {
    const location = stubLocation()
    const auth = createRutoAuth({ gateway: 'https://api.mdev.kz', provider: 'corp' })
    auth.login()
    expect(location.assign).toHaveBeenLastCalledWith(
      'https://api.mdev.kz/_auth/corp/login?redirect=' + encodeURIComponent('https://shop.example.com/orders/7?x=1'),
    )
    auth.logout()
    expect(location.assign).toHaveBeenLastCalledWith(
      'https://api.mdev.kz/_auth/corp/logout?redirect=' + encodeURIComponent('https://shop.example.com/'),
    )
  })

  it('passes the OpenID Connect prompt through', () => {
    const location = stubLocation()
    const auth = createRutoAuth({ gateway: 'https://api.mdev.kz', provider: 'corp' })
    expect(auth.loginUrl('/x', { prompt: 'login' })).toBe('https://api.mdev.kz/_auth/corp/login?redirect=%2Fx&prompt=login')
    expect(auth.loginUrl('/x', { prompt: ['login', 'consent'] })).toBe(
      'https://api.mdev.kz/_auth/corp/login?redirect=%2Fx&prompt=login%20consent',
    )
    expect(auth.loginUrl('/x', { prompt: [] })).toBe('https://api.mdev.kz/_auth/corp/login?redirect=%2Fx')
    auth.login('/x', { prompt: 'select_account' })
    expect(location.assign).toHaveBeenLastCalledWith('https://api.mdev.kz/_auth/corp/login?redirect=%2Fx&prompt=select_account')
  })

  it('rejects an empty gateway or provider', () => {
    expect(() => createRutoAuth({ gateway: '', provider: 'corp' })).toThrow()
    expect(() => createRutoAuth({ gateway: 'https://x', provider: ' ' })).toThrow()
  })
})

describe('refresh', () => {
  it('maps 204 / 401 / 5xx / network failure to ok / expired / failed', async () => {
    for (const [status, want] of [
      [204, 'ok'],
      [401, 'expired'],
      [502, 'failed'],
      [0, 'failed'],
    ] as const) {
      const fetch = fakeFetch({ '/refresh': [status] })
      const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch })
      expect(await auth.refresh()).toBe(want)
      expect(fetch.calls[0]).toMatchObject({
        url: 'https://gw/_auth/corp/refresh',
        init: { method: 'POST', credentials: 'include' },
      })
    }
  })

  it('shares one request between concurrent callers, then refreshes again', async () => {
    const fetch = fakeFetch({ '/refresh': [204] })
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch })
    const results = await Promise.all([auth.refresh(), auth.refresh(), auth.refresh()])
    expect(results).toEqual(['ok', 'ok', 'ok'])
    expect(fetch.calls).toHaveLength(1)
    await auth.refresh()
    expect(fetch.calls).toHaveLength(2)
  })

  it('takes the Web Lock when the browser has one', async () => {
    const request = vi.fn((_name: string, fn: () => Promise<unknown>) => fn())
    vi.stubGlobal('navigator', { locks: { request } })
    const fetch = fakeFetch({ '/refresh': [204] })
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', app: '/orders', fetch })
    await auth.refresh()
    expect(request).toHaveBeenCalledWith('ruto-auth:corp/orders', expect.any(Function))
  })
})

describe('run', () => {
  it('returns a non-401 result as is, without touching refresh', async () => {
    const fetch = fakeFetch({})
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch })
    const result = await auth.run(async () => ({ status: 500, body: 'boom' }))
    expect(result.status).toBe(500)
    expect(fetch.calls).toHaveLength(0)
  })

  it('on 401 refreshes once and repeats the request', async () => {
    const fetch = fakeFetch({ '/refresh': [204] })
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch })
    let attempts = 0
    const result = await auth.run(async () => ({ status: ++attempts === 1 ? 401 : 200 }))
    expect(result.status).toBe(200)
    expect(attempts).toBe(2)
    expect(fetch.calls).toHaveLength(1)
  })

  it('a thrown 401 (axios style) is retried too, and re-thrown when the session is over', async () => {
    const fetch = fakeFetch({ '/refresh': [204, 401] })
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch })
    let attempts = 0
    const unauthorized = Object.assign(new Error('Request failed with status code 401'), { response: { status: 401 } })
    const value = await auth.run(async () => {
      if (++attempts === 1) throw unauthorized
      return 'second try'
    })
    expect(value).toBe('second try')

    await expect(auth.run(async () => Promise.reject(unauthorized))).rejects.toBe(unauthorized)
  })

  it('session over: onLoginRequired fires once for a burst of 401s, the originals are returned', async () => {
    const fetch = fakeFetch({ '/refresh': [401] })
    const onLoginRequired = vi.fn()
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch, onLoginRequired })
    const results = await Promise.all([1, 2, 3].map(() => auth.run(async () => ({ status: 401 }))))
    expect(results.map((r) => r.status)).toEqual([401, 401, 401])
    expect(fetch.calls).toHaveLength(1)
    expect(onLoginRequired).toHaveBeenCalledTimes(1)

    await auth.run(async () => ({ status: 401 }))
    expect(onLoginRequired).toHaveBeenCalledTimes(2)
  })

  it('provider down: no login prompt, the 401 is handed back', async () => {
    const fetch = fakeFetch({ '/refresh': [502] })
    const onLoginRequired = vi.fn()
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch, onLoginRequired })
    const result = await auth.run(async () => ({ status: 401 }))
    expect(result.status).toBe(401)
    expect(onLoginRequired).not.toHaveBeenCalled()
  })

  it('accepts a custom status reader', async () => {
    const fetch = fakeFetch({ '/refresh': [204] })
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch })
    let attempts = 0
    const result = await auth.run(async () => ({ code: ++attempts === 1 ? 'unauthenticated' : 'ok' }), {
      statusOf: (r) => ((r as { code: string }).code === 'unauthenticated' ? 401 : 200),
    })
    expect(result.code).toBe('ok')
  })
})

describe('fetch wrapper', () => {
  it('sends cookies and repeats after a successful refresh', async () => {
    const fetch = fakeFetch({ '/refresh': [204], '/shop/hello': [401, json(200, { hello: 'alice' })] })
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch })
    const response = await auth.fetch('https://gw/shop/hello', { headers: { 'x-a': '1' } })
    expect(response.status).toBe(200)
    expect(await response.json()).toEqual({ hello: 'alice' })
    expect(fetch.calls.map((c) => c.url)).toEqual([
      'https://gw/shop/hello',
      'https://gw/_auth/corp/refresh',
      'https://gw/shop/hello',
    ])
    expect(fetch.calls[0]!.init).toMatchObject({ credentials: 'include', headers: { 'x-a': '1' } })
  })
})

describe('me', () => {
  it('returns the claims', async () => {
    const fetch = fakeFetch({ '/me': [json(200, { sub: 'u1', preferred_username: 'alice', exp: 1 })] })
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch })
    expect(await auth.me()).toMatchObject({ preferred_username: 'alice' })
    expect(fetch.calls[0]).toMatchObject({ url: 'https://gw/_auth/corp/me', init: { credentials: 'include' } })
  })

  it('expired access cookie: refresh then claims', async () => {
    const fetch = fakeFetch({ '/me': [401, json(200, { sub: 'u1' })], '/refresh': [204] })
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch })
    expect(await auth.me()).toEqual({ sub: 'u1' })
    expect(fetch.calls).toHaveLength(3)
  })

  it('no session: null, and no login prompt', async () => {
    const fetch = fakeFetch({ '/me': [401], '/refresh': [401] })
    const onLoginRequired = vi.fn()
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch, onLoginRequired })
    expect(await auth.me()).toBeNull()
    expect(onLoginRequired).not.toHaveBeenCalled()
  })

  it('gateway failure throws', async () => {
    const fetch = fakeFetch({ '/me': [502] })
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch })
    await expect(auth.me()).rejects.toThrow(/502/)
  })
})

describe('loginError', () => {
  it('reads auth_error from an address, the current page by default', () => {
    expect(loginError('https://app.example.com/login?auth_error=access_denied&return_to=%2Fx')).toBe('access_denied')
    expect(loginError('https://app.example.com/?auth_error=login_required#/orders')).toBe('login_required')
    expect(loginError('https://app.example.com/login')).toBeNull()
    expect(loginError('/login?auth_error=state_mismatch')).toBe('state_mismatch')
    expect(loginError('not a url ::')).toBeNull()
    stubLocation('https://shop.example.com/?auth_error=invalid_grant')
    expect(loginError()).toBe('invalid_grant')
  })
})

describe('statusOf', () => {
  it('reads the status of the common clients', () => {
    expect(statusOf(new Response(null, { status: 401 }))).toBe(401)
    expect(statusOf({ status: 200, data: {} })).toBe(200)
    expect(statusOf({ response: { status: 401 }, message: 'axios' })).toBe(401)
    expect(statusOf({ statusCode: 401, status: 401 })).toBe(401)
    expect(statusOf({ status: 0, response: { status: 403 } })).toBe(403)
    expect(statusOf(new Error('x'))).toBeUndefined()
    expect(statusOf(undefined)).toBeUndefined()
    expect(statusOf('text')).toBeUndefined()
  })
})
