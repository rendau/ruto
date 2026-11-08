// @rendau/ruto-auth — browser SDK for the ruto gateway login.
//
// The gateway keeps the session in httpOnly cookies on its own host; a front
// never sees a token. What a front does: send requests with cookies, and after
// a 401 ask the gateway to renew the session (POST …/refresh) and repeat the
// request once. This module does exactly that, for any HTTP client: `run`
// wraps a request of any library, `fetch` is a ready wrapper over fetch, and
// the ./axios and ./ofetch entries plug into those libraries' hooks.

export type { RolesOf } from './roles'
export { rolesOf, hasRole, usernameOf } from './roles'

export type FetchLike = (input: string | URL, init?: RequestInit) => Promise<Response>

/** Payload of the access token, as the gateway's `me` route returns it. */
export type Claims = Record<string, unknown> & {
  sub?: string
  iss?: string
  exp?: number
  preferred_username?: string
}

/**
 * Outcome of a session refresh:
 * - `ok` — cookies renewed, repeat the request;
 * - `expired` — the session is over (no refresh cookie, or the provider
 *   rejected the refresh token), time to log in;
 * - `failed` — the provider or the network is down; the session may still be
 *   alive, nothing to do but try later.
 */
export type RefreshResult = 'ok' | 'expired' | 'failed'

export interface RutoAuthOptions {
  /** Public address of the gateway, e.g. `https://api.mdev.kz`. */
  gateway: string
  /** Login provider name in the gateway root config (`Root.jwt[].name`). */
  provider: string
  /**
   * Path prefix of the app when it logs in with its own client (`App.jwt`),
   * e.g. `/orders`. Leave empty for the shared login of the provider.
   */
  app?: string
  /** fetch used for the gateway's own routes (`me`, `refresh`); default global fetch. */
  fetch?: FetchLike
  /**
   * Called once per refresh that found the session over: a request got 401,
   * the refresh was refused too. Typical handler: `() => auth.login()` or a
   * router push to the app's login page. Not called from `me()`, which just
   * resolves to null.
   */
  onLoginRequired?: (() => void) | undefined
  /** Name of the Web Lock that serializes refreshes across tabs; default derived from the unit. */
  lockName?: string
}

/** Values of the OpenID Connect `prompt` parameter (Core §3.1.2.1). */
export type Prompt = 'none' | 'login' | 'consent' | 'select_account'

export interface LoginOptions {
  /**
   * What the provider must ask even with a live SSO session: `login` — the
   * credentials again (another person logs in after an access denial),
   * `select_account` — an account picker, `consent` — the consent screen;
   * `none` — no interaction at all. Several values go as a list; `none` only
   * alone. The gateway passes it to the provider as it is.
   */
  prompt?: Prompt | Prompt[] | undefined
}

export interface RunOptions {
  /**
   * How to read the HTTP status from what the request resolved or rejected
   * with. The default understands fetch `Response`, axios responses and
   * errors, ofetch `FetchError`, ky `HTTPError` and `XMLHttpRequest`.
   */
  statusOf?: (resultOrError: unknown) => number | undefined
}

/** HTTP status of a result or error of the common browser HTTP clients. */
export function statusOf(value: unknown): number | undefined {
  if (!value || typeof value !== 'object') return undefined
  const v = value as { status?: unknown; statusCode?: unknown; response?: { status?: unknown } }
  for (const candidate of [v.status, v.statusCode, v.response?.status]) {
    if (typeof candidate === 'number' && candidate > 0) return candidate
  }
  return undefined
}

const trimSlash = (s: string): string => s.replace(/\/+$/, '')

export class RutoAuth {
  /** Base of the unit's routes: `<gateway>/_auth/<provider>[<app>]`. */
  readonly base: string
  /** `me` lives under the access cookie's path: `<gateway>[<app>]/_auth/<provider>/me`. */
  readonly meUrl: string
  readonly refreshUrl: string

  private readonly fetchImpl: FetchLike
  private readonly lockName: string
  private readonly onLoginRequired: (() => void) | undefined
  private refreshing: Promise<RefreshResult> | null = null
  private notified: Promise<RefreshResult> | null = null

  constructor(options: RutoAuthOptions) {
    const gateway = trimSlash(options.gateway.trim())
    const provider = options.provider.trim()
    if (!gateway || !provider) throw new Error('ruto-auth: gateway and provider are required')
    let app = trimSlash((options.app ?? '').trim())
    if (app && !app.startsWith('/')) app = '/' + app

    this.base = `${gateway}/_auth/${provider}${app}`
    this.meUrl = `${gateway}${app}/_auth/${provider}/me`
    this.refreshUrl = `${this.base}/refresh`
    this.fetchImpl = options.fetch ?? ((input, init) => globalThis.fetch(input, init))
    this.lockName = options.lockName ?? `ruto-auth:${provider}${app}`
    this.onLoginRequired = options.onLoginRequired
  }

  /**
   * Address that starts the login. `redirect` is where the gateway sends the
   * browser afterwards: a path on the gateway or an absolute URL on an origin
   * listed in the gateway's CORS config; default — the current page.
   */
  loginUrl(redirect?: string, options: LoginOptions = {}): string {
    let url = `${this.base}/login?redirect=${encodeURIComponent(redirect ?? currentHref())}`
    const prompt = Array.isArray(options.prompt) ? options.prompt.join(' ') : options.prompt
    if (prompt) url += `&prompt=${encodeURIComponent(prompt)}`
    return url
  }

  /** Address that ends the session (gateway cookies and the provider session). Default redirect — the site root. */
  logoutUrl(redirect?: string): string {
    return `${this.base}/logout?redirect=${encodeURIComponent(redirect ?? currentOrigin() + '/')}`
  }

  /** Sends the browser to the login. Comes back to `redirect` (default: the current page) logged in. */
  login(redirect?: string, options?: LoginOptions): void {
    globalThis.location.assign(this.loginUrl(redirect, options))
  }

  /** Sends the browser through the logout; comes back to `redirect` (default: the site root). */
  logout(redirect?: string): void {
    globalThis.location.assign(this.logoutUrl(redirect))
  }

  /**
   * Renews the session cookies with the refresh cookie. Concurrent calls share
   * one request; across tabs a Web Lock makes the refreshes take turns (the
   * gateway answers a repeated refresh with the same cookies for a while, so
   * the loser of a race still wins).
   */
  refresh(): Promise<RefreshResult> {
    if (!this.refreshing) {
      this.refreshing = this.withLock(() => this.doRefresh()).finally(() => {
        this.refreshing = null
      })
    }
    return this.refreshing
  }

  /**
   * Who is logged in: the claims of the access token, or null when there is no
   * session (after one refresh attempt). Throws on a gateway or network failure.
   */
  async me(): Promise<Claims | null> {
    const response = await this.run(() => this.fetchImpl(this.meUrl, { credentials: 'include' }), { notify: false })
    if (response.status === 401) return null
    if (!response.ok) throw new Error(`ruto-auth: me failed with status ${response.status}`)
    return (await response.json()) as Claims
  }

  /**
   * Runs a request of any HTTP client with the session: when it ends in a 401
   * (resolved or thrown), refreshes the session once and repeats it. If the
   * refresh says the session is over, `onLoginRequired` is called and the
   * original outcome is returned (or re-thrown) as it was.
   */
  async run<T>(request: () => Promise<T>, options: RunOptions & { notify?: boolean } = {}): Promise<T> {
    const status = options.statusOf ?? statusOf
    let outcome: { ok: true; value: T } | { ok: false; error: unknown }
    try {
      outcome = { ok: true, value: await request() }
    } catch (error) {
      outcome = { ok: false, error }
    }

    if (status(outcome.ok ? outcome.value : outcome.error) === 401 && (await this.recover(options.notify ?? true))) {
      return request()
    }
    if (outcome.ok) return outcome.value
    throw outcome.error
  }

  /**
   * What to do after a 401: refreshes the session and tells whether the
   * request should be repeated. Building block for adapters of HTTP clients
   * with interceptors (see ./axios, ./ofetch).
   */
  async recover(notify = true): Promise<boolean> {
    const attempt = this.refresh()
    const result = await attempt
    if (result === 'expired' && notify && this.notified !== attempt) {
      this.notified = attempt
      this.onLoginRequired?.()
    }
    return result === 'ok'
  }

  /** fetch with the session cookies and one refresh-and-repeat after a 401. */
  fetch: FetchLike = (input, init) => {
    const withCookies: RequestInit = { ...init, credentials: 'include' }
    return this.run(() => this.fetchImpl(input, withCookies))
  }

  private async doRefresh(): Promise<RefreshResult> {
    let response: Response
    try {
      response = await this.fetchImpl(this.refreshUrl, { method: 'POST', credentials: 'include' })
    } catch {
      return 'failed'
    }
    if (response.status === 204 || response.status === 200) return 'ok'
    if (response.status === 401) return 'expired'
    return 'failed'
  }

  private withLock<T>(fn: () => Promise<T>): Promise<T> {
    const locks = (globalThis.navigator as Navigator | undefined)?.locks
    return locks ? locks.request(this.lockName, fn) : fn()
  }
}

/**
 * The error code a failed login came back with: the gateway sends the browser
 * to the login's return address with `auth_error=<code>` in the query (the
 * provider's own code such as `access_denied` or `login_required`,
 * `invalid_grant`, `temporarily_unavailable`, or the gateway's `state_mismatch`
 * and `invalid_token`). Null when the address carries none. Reads the current
 * page by default.
 */
export function loginError(url?: string): string | null {
  const raw = url ?? currentHref()
  try {
    return new URL(raw, 'http://localhost/').searchParams.get('auth_error') || null
  } catch {
    return null
  }
}

/** Creates a client of one login unit (provider, or provider + app). */
export function createRutoAuth(options: RutoAuthOptions): RutoAuth {
  return new RutoAuth(options)
}

function currentHref(): string {
  return typeof globalThis.location === 'object' ? globalThis.location.href : '/'
}

function currentOrigin(): string {
  return typeof globalThis.location === 'object' ? globalThis.location.origin : ''
}
