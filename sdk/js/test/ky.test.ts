import ky from 'ky'
import { describe, expect, it, vi } from 'vitest'
import { createRutoAuth } from '../src/index'
import { kyOptions } from '../src/ky'
import { fakeFetch, json } from './helpers'

function setup(plan: Parameters<typeof fakeFetch>[0], onLoginRequired?: () => void) {
  const fetch = fakeFetch(plan)
  const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch, onLoginRequired })
  const api: typeof ky = ky.create({
    prefix: 'https://gw',
    fetch: fetch as unknown as typeof globalThis.fetch,
    retry: 0,
    ...kyOptions(auth, (request) => api(request)),
  })
  return { fetch, auth, api }
}

describe('kyOptions', () => {
  it('sends cookies, refreshes after a 401 and repeats once', async () => {
    const { fetch, api } = setup({ '/refresh': [204], '/shop/hello': [401, json(200, { hello: 'alice' })] })
    const body = await api.post('shop/hello', { json: { a: 1 } }).json<{ hello: string }>()
    expect(body).toEqual({ hello: 'alice' })
    expect(fetch.calls.map((c) => c.url)).toEqual([
      'https://gw/shop/hello',
      'https://gw/_auth/corp/refresh',
      'https://gw/shop/hello',
    ])
  })

  it('session over: HTTPError 401 reaches the caller, onLoginRequired fires', async () => {
    const onLoginRequired = vi.fn()
    const { fetch, api } = setup({ '/refresh': [401], '/shop/hello': [401] }, onLoginRequired)
    await expect(api.get('shop/hello')).rejects.toMatchObject({ response: { status: 401 } })
    expect(fetch.calls).toHaveLength(2)
    expect(onLoginRequired).toHaveBeenCalledTimes(1)
  })

  it('a 401 on the repeat is final', async () => {
    const { fetch, api } = setup({ '/refresh': [204], '/a': [401] })
    await expect(api.get('a')).rejects.toMatchObject({ response: { status: 401 } })
    expect(fetch.calls.filter((c) => c.url.endsWith('/a'))).toHaveLength(2)
    expect(fetch.calls.filter((c) => c.url.endsWith('/refresh'))).toHaveLength(1)
  })
})
