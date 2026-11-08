import { vi } from 'vitest'
import type { FetchLike } from '../src/index'

export interface Call {
  url: string
  init: RequestInit | undefined
}

/**
 * A fetch that answers from a queue of status codes per URL suffix and records
 * the calls. The last entry of a queue repeats; 0 means a network failure.
 */
export function fakeFetch(plan: Record<string, Array<number | (() => Response)>>): FetchLike & { calls: Call[] } {
  const calls: Call[] = []
  const fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = input instanceof Request ? input.url : String(input)
    calls.push({ url, init })
    const key = Object.keys(plan).find((suffix) => url.endsWith(suffix))
    const queue = key ? plan[key] : undefined
    if (!queue || !queue.length) throw new Error(`unexpected fetch ${init?.method ?? 'GET'} ${url}`)
    const next = queue.length > 1 ? queue.shift()! : queue[0]!
    if (typeof next === 'function') return next()
    if (next === 0) throw new TypeError('connection refused')
    return new Response(next === 204 ? null : '{}', { status: next })
  }) as FetchLike & { calls: Call[] }
  fetch.calls = calls
  return fetch
}

export const json = (status: number, body: unknown) => () =>
  new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })

export function stubLocation(href = 'https://shop.example.com/orders/7?x=1') {
  const url = new URL(href)
  const location = { href, origin: url.origin, assign: vi.fn() }
  vi.stubGlobal('location', location)
  return location
}
