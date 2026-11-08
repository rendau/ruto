// Adapter for ofetch (Nuxt's $fetch): spread `ofetchOptions(auth)` into the
// options of `$fetch.create` or of a single call. Requests travel with the
// session cookies; after a 401 the session is refreshed and ofetch's own retry
// repeats the request once. Typed structurally so that ofetch is not a
// dependency of this package.

import type { RutoAuth } from './index'

interface OfetchContextLike {
  response?: { status: number }
  options: {
    retry?: number | false
    retryDelay?: unknown
    retryStatusCodes?: number[]
  }
}

export interface OfetchAuthOptions {
  credentials: 'include'
  onResponseError: (context: OfetchContextLike) => Promise<void>
}

const retriedMark = '__rutoAuthRetried'

/**
 * Options to merge into ofetch: `{ credentials, onResponseError }`. Note that
 * ofetch replaces hooks instead of chaining them: an `onResponseError` of the
 * call overrides this one.
 */
export function ofetchOptions(auth: RutoAuth): OfetchAuthOptions {
  return {
    credentials: 'include',
    async onResponseError(context) {
      const { options } = context
      const marks = options as Record<string, unknown>
      if (context.response?.status !== 401 || marks[retriedMark]) return
      if (!(await auth.recover())) return
      // ofetch reads these right after the hook and copies the options into the
      // repeat: one repeat, right away, on 401 only, and never a third attempt.
      marks[retriedMark] = true
      options.retry = 1
      options.retryDelay = 0
      options.retryStatusCodes = [401]
    },
  }
}
