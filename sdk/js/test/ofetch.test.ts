import { createFetch } from 'ofetch'
import { describe, expect, it, vi } from 'vitest'
import { createRutoAuth } from '../src/index'
import { ofetchOptions } from '../src/ofetch'
import { fakeFetch, json } from './helpers'

function setup(plan: Parameters<typeof fakeFetch>[0], onLoginRequired?: () => void) {
  const fetch = fakeFetch(plan)
  const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch, onLoginRequired })
  const $fetch = createFetch({
    fetch: fetch as unknown as typeof globalThis.fetch,
    Headers,
    AbortController,
    defaults: { ...ofetchOptions(auth), baseURL: 'https://gw' },
  })
  return { fetch, auth, $fetch }
}

describe('ofetchOptions', () => {
  it('sends cookies, refreshes after a 401 and lets ofetch repeat once', async () => {
    const { fetch, $fetch } = setup({ '/refresh': [204], '/shop/hello': [401, json(200, { hello: 'alice' })] })
    const body = await $fetch<{ hello: string }>('/shop/hello', { method: 'POST', body: { a: 1 } })
    expect(body).toEqual({ hello: 'alice' })
    expect(fetch.calls.map((c) => c.url)).toEqual([
      'https://gw/shop/hello',
      'https://gw/_auth/corp/refresh',
      'https://gw/shop/hello',
    ])
    expect(fetch.calls[0]!.init).toMatchObject({ credentials: 'include', method: 'POST' })
    expect(fetch.calls[2]!.init?.body).toBe('{"a":1}')
  })

  it('session over: FetchError 401 reaches the caller, onLoginRequired fires', async () => {
    const onLoginRequired = vi.fn()
    const { fetch, $fetch } = setup({ '/refresh': [401], '/shop/hello': [401] }, onLoginRequired)
    await expect($fetch('/shop/hello')).rejects.toMatchObject({ statusCode: 401 })
    expect(fetch.calls).toHaveLength(2)
    expect(onLoginRequired).toHaveBeenCalledTimes(1)
  })

  it('a 401 on the repeat is final, other statuses are not touched', async () => {
    const { fetch, $fetch } = setup({ '/refresh': [204], '/a': [401], '/b': [403] })
    await expect($fetch('/a')).rejects.toMatchObject({ statusCode: 401 })
    expect(fetch.calls.filter((c) => c.url.endsWith('/a'))).toHaveLength(2)
    await expect($fetch('/b')).rejects.toMatchObject({ statusCode: 403 })
    expect(fetch.calls.filter((c) => c.url.endsWith('/refresh'))).toHaveLength(1)
  })
})
