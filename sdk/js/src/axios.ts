// Adapter for axios: `installAxios(auth, instance)` makes every request of the
// instance travel with the session cookies and repeats it once after a 401
// when the session could be refreshed. Typed structurally so that axios is
// not a dependency of this package.

import type { RutoAuth } from './index'

interface AxiosRequestConfigLike {
  withCredentials?: boolean
}

interface AxiosErrorLike {
  config?: AxiosRequestConfigLike
  response?: { status?: number }
}

/* eslint-disable @typescript-eslint/no-explicit-any -- axios' interceptor
   generics are not reproducible structurally; `any` keeps every axios 0.x/1.x
   instance assignable here without a dependency on its types. */
export interface AxiosLike {
  interceptors: {
    request: {
      use(onFulfilled: (config: any) => any): number
      eject(id: number): void
    }
    response: {
      use(onFulfilled: ((value: any) => any) | null | undefined, onRejected: (error: any) => any): number
      eject(id: number): void
    }
  }
  request(config: any): Promise<any>
}
/* eslint-enable @typescript-eslint/no-explicit-any */

const retriedMark = '__rutoAuthRetried'

/** Installs the interceptors; the returned function removes them. */
export function installAxios(auth: RutoAuth, axios: AxiosLike): () => void {
  const requestId = axios.interceptors.request.use((config: AxiosRequestConfigLike) => {
    config.withCredentials = true
    return config
  })

  const responseId = axios.interceptors.response.use(undefined, async (error: unknown) => {
    const e = error as AxiosErrorLike | null
    const config = e?.config
    if (!config || e?.response?.status !== 401 || (config as Record<string, unknown>)[retriedMark]) throw error
    if (!(await auth.recover())) throw error
    // The mark survives axios' own merge of the config, so the repeat is not repeated.
    return axios.request({ ...config, [retriedMark]: true })
  })

  return () => {
    axios.interceptors.request.eject(requestId)
    axios.interceptors.response.eject(responseId)
  }
}
