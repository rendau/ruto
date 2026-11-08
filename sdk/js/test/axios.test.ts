import axios, { AxiosError, type AxiosRequestConfig } from 'axios'
import { describe, expect, it, vi } from 'vitest'
import { installAxios } from '../src/axios'
import { createRutoAuth } from '../src/index'
import { fakeFetch } from './helpers'

/** axios instance whose adapter answers from a queue of statuses, like a server would. */
function fakeAxios(statuses: number[]) {
  const seen: AxiosRequestConfig[] = []
  const instance = axios.create({
    adapter: async (config) => {
      seen.push(config)
      const status = statuses.length > 1 ? statuses.shift()! : statuses[0]!
      const response = { data: { status }, status, statusText: '', headers: {}, config }
      if (status >= 400) {
        throw new AxiosError(`Request failed with status code ${status}`, 'ERR_BAD_REQUEST', config, null, response)
      }
      return response
    },
  })
  return { instance, seen }
}

describe('installAxios', () => {
  it('sends cookies, repeats once after a refresh', async () => {
    const fetch = fakeFetch({ '/refresh': [204] })
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch })
    const { instance, seen } = fakeAxios([401, 200])
    installAxios(auth, instance)

    const response = await instance.get('/shop/hello', { headers: { 'x-a': '1' } })
    expect(response.status).toBe(200)
    expect(seen).toHaveLength(2)
    expect(seen[0]!.withCredentials).toBe(true)
    expect(seen[1]!.headers?.['x-a']).toBe('1')
    expect(fetch.calls).toHaveLength(1)
  })

  it('session over: the 401 error reaches the caller, onLoginRequired fires', async () => {
    const fetch = fakeFetch({ '/refresh': [401] })
    const onLoginRequired = vi.fn()
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch, onLoginRequired })
    const { instance, seen } = fakeAxios([401])
    installAxios(auth, instance)

    await expect(instance.get('/shop/hello')).rejects.toMatchObject({ response: { status: 401 } })
    expect(seen).toHaveLength(1)
    expect(onLoginRequired).toHaveBeenCalledTimes(1)
  })

  it('a 401 on the repeat is not retried again; other errors pass through; uninstall works', async () => {
    const fetch = fakeFetch({ '/refresh': [204] })
    const auth = createRutoAuth({ gateway: 'https://gw', provider: 'corp', fetch })
    const { instance, seen } = fakeAxios([401, 401, 500, 401])
    const uninstall = installAxios(auth, instance)

    await expect(instance.get('/a')).rejects.toMatchObject({ response: { status: 401 } })
    expect(seen).toHaveLength(2)
    await expect(instance.get('/b')).rejects.toMatchObject({ response: { status: 500 } })
    expect(fetch.calls).toHaveLength(1)

    uninstall()
    await expect(instance.get('/c')).rejects.toMatchObject({ response: { status: 401 } })
    expect(seen).toHaveLength(4)
    expect(fetch.calls).toHaveLength(1)
  })
})
