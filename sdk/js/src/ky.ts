// Adapter for ky: spread `kyOptions(auth, ky)` into `ky.create` or
// `ky.extend`. Requests travel with the session cookies; after a 401 the
// session is refreshed and the request is repeated once. Typed structurally
// so that ky is not a dependency of this package; works with the hook
// signatures of ky 1.x (`(request, options, response)`) and 2.x (`(state)`).

import type { RutoAuth } from './index'

interface KyLike {
  (request: Request): Promise<Response>
}

type AfterResponseHook = (
  ...args: [state: { request: Request; response: Response }] | [request: Request, options: unknown, response: Response]
) => Promise<Response | undefined>

export interface KyAuthOptions {
  credentials: 'include'
  hooks: {
    afterResponse: AfterResponseHook[]
  }
}

const retriedMark = 'x-ruto-auth-retried'

/**
 * Options to merge into ky: `{ credentials, hooks.afterResponse }`. The hook
 * repeats the request with ky itself, so pass the instance the options are
 * given to (a function that calls it works too: `(req) => api(req)`).
 */
export function kyOptions(auth: RutoAuth, ky: KyLike): KyAuthOptions {
  return {
    credentials: 'include',
    hooks: {
      afterResponse: [
        async (...args) => {
          const [request, response] =
            args[0] instanceof Request ? [args[0], args[2] as Response] : [args[0].request, args[0].response]
          if (response.status !== 401 || request.headers.has(retriedMark)) return undefined
          if (!(await auth.recover())) return undefined
          const repeat = new Request(request, { headers: new Headers(request.headers) })
          repeat.headers.set(retriedMark, '1')
          return ky(repeat)
        },
      ],
    },
  }
}
