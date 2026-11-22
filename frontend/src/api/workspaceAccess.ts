/**
 * Whether the server refuses this session or workspace as a whole, and why.
 *
 * Some refusals are not about one request but about the whole session or
 * workspace. The API client records them here, and the App shows a page that
 * says what happened. A toast or a "Retry" would not help: no request can
 * succeed until the user signs in again or someone changes the workspace's
 * state.
 *
 * - `session_ended`: a 401 from the API. The edge answers an expired session
 *   with 401 and `{"error": "session_expired", "login_url": "..."}`. An older
 *   edge redirects the request to the sign-in page instead, which the browser
 *   refuses to follow for a data request; see {@link probeSessionEnded}.
 * - `no_identity`: a 401 that explains itself in a problem `detail` and names
 *   no sign-in page. A server with no identity source answers every request
 *   so, and names the settings that fix it; signing in again cannot help.
 * - `suspended` / `archived`: a multi-tenant server answers a suspended or
 *   archived workspace with 403 `tenant_suspended` or `tenant_archived`.
 * - `unknown_workspace`: 403 `unknown_tenant`, an organisation with no
 *   workspace on this server.
 */
import { ref } from 'vue'
import { apiUrl } from './base'
import { ApiError } from './errors'
import { problemCode } from '@/i18n/apiErrors'

/** The refusals, named once so other modules compare against these. */
export const REFUSAL = {
  sessionEnded: 'session_ended',
  noIdentity: 'no_identity',
  suspended: 'suspended',
  archived: 'archived',
  unknownWorkspace: 'unknown_workspace',
} as const

export type AccessRefusal = (typeof REFUSAL)[keyof typeof REFUSAL]

export const accessRefusal = ref<AccessRefusal | null>(null)

/** Where "Sign in again" goes, when the server named a sign-in page. */
export const loginUrl = ref<string | null>(null)

/** The server's explanation of a `no_identity` refusal. */
export const refusalDetail = ref<string | null>(null)

const PROBLEM_REFUSALS: Readonly<Record<string, AccessRefusal>> = {
  tenant_suspended: REFUSAL.suspended,
  tenant_archived: REFUSAL.archived,
  unknown_tenant: REFUSAL.unknownWorkspace,
}

/** The refusal a response stands for, or null for an ordinary answer. */
export function refusalOf(status: number, data: unknown): AccessRefusal | null {
  if (status === 401) {
    return detailOf(data) && !loginUrlOf(data) ? REFUSAL.noIdentity : REFUSAL.sessionEnded
  }
  if (status !== 403 || typeof data !== 'object' || data === null) return null
  const code = problemCode((data as { type?: string }).type)
  if (!code || !Object.prototype.hasOwnProperty.call(PROBLEM_REFUSALS, code)) return null
  return PROBLEM_REFUSALS[code]
}

function detailOf(data: unknown): string | null {
  if (typeof data !== 'object' || data === null) return null
  const detail = (data as { detail?: unknown }).detail
  return typeof detail === 'string' && detail.trim() ? detail.trim() : null
}

/**
 * The sign-in page a 401 body names, if it is a local path on this site. The
 * edge names its sign-in page so (`/auth/login?rd=...`). Anything else is
 * ignored, so a body cannot send the browser to another site: an absolute
 * URL, a `javascript:` URL, a scheme-relative `//host`, and a `/\host`,
 * which browsers also read as scheme-relative.
 */
export function loginUrlOf(data: unknown): string | null {
  if (typeof data !== 'object' || data === null) return null
  const raw = (data as { login_url?: unknown }).login_url
  if (typeof raw !== 'string' || !raw.startsWith('/') || /^\/[/\\]/.test(raw)) return null
  try {
    const url = new URL(raw, window.location.origin)
    // Check what is returned, not only the input: path normalisation turns
    // `/.//host` into the path `//host`. The absolute URL keeps it on this
    // site even so.
    if (url.origin !== window.location.origin || /^\/[/\\]/.test(url.pathname)) return null
    return url.href
  } catch {
    return null
  }
}

/** Records the refusal one API response stands for. The first one stays. */
export function trackAccess(status: number, data: unknown): void {
  const refusal = refusalOf(status, data)
  if (!refusal || accessRefusal.value) return
  loginUrl.value = refusal === REFUSAL.sessionEnded ? loginUrlOf(data) : null
  refusalDetail.value = refusal === REFUSAL.noIdentity ? detailOf(data) : null
  accessRefusal.value = refusal
}

/**
 * The error a request gets while a refusal is up. The client sends no more
 * requests then: none can succeed until the user signs in again or reloads,
 * and each would only fail and be logged. It is a cancellation, so callers
 * drop it as they drop an aborted request.
 */
export function refusedRequestError(): ApiError {
  return new ApiError('Request cancelled', { kind: 'cancelled', original: null })
}

/**
 * Whether a caught error is not worth logging: a cancelled request, or any
 * failure while the session or workspace screen is up, which already says
 * what happened.
 */
export function isQuietFailure(err: unknown): boolean {
  if (accessRefusal.value) return true
  return err instanceof ApiError && err.kind === 'cancelled'
}

/** How long after one probe the next may run, so an outage does not double the traffic. */
const PROBE_COOLDOWN_MS = 30_000

let probing: Promise<void> | null = null
let lastProbe = Number.NEGATIVE_INFINITY

/**
 * Asks whether a request that failed without a response did so because the
 * session ended.
 *
 * An edge that sends an expired session to its sign-in page answers a data
 * request with a cross-origin redirect. The browser refuses to follow it and
 * reports a network error, which looks the same as a server that is down. A
 * request with `redirect: 'manual'` sees the redirect itself, as an
 * `opaqueredirect` response, so one such request tells the two apart. rela
 * itself never redirects an API path.
 *
 * At most one probe runs at a time, and at most one per
 * {@link PROBE_COOLDOWN_MS}.
 */
export function probeSessionEnded(
  fetchFn: typeof fetch = (input, init) => fetch(input, init),
  now: () => number = Date.now
): Promise<void> {
  if (probing) return probing
  if (now() - lastProbe < PROBE_COOLDOWN_MS) return Promise.resolve()
  lastProbe = now()
  const run = (async () => {
    const res = await fetchFn(apiUrl('/api/v1/_tenant'), {
      credentials: 'same-origin',
      cache: 'no-store',
      redirect: 'manual',
    })
    if (res.type === 'opaqueredirect') {
      trackAccess(401, null)
      return
    }
    // Read the body whatever the status: an unread body keeps the request
    // open in the browser.
    const body: unknown = await res.json().catch(() => null)
    if (res.status === 401) trackAccess(401, body)
  })()
  probing = run
    .catch(() => {
      // The server is unreachable: the request's own error says so.
    })
    .finally(() => {
      probing = null
    })
  return probing
}

/** Resets module state between tests. */
export function resetAccessForTest(): void {
  accessRefusal.value = null
  loginUrl.value = null
  refusalDetail.value = null
  probing = null
  lastProbe = Number.NEGATIVE_INFINITY
}
