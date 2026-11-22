/**
 * Whether the server reported that this workspace is read-only.
 *
 * A read-only tenant refuses every write with a 403 `tenant_read_only`
 * problem. The API client sets this flag on any such refusal, from whichever
 * write it came: the whole workspace is read-only, not one form. A create form
 * then says so and offers no Create button instead of a form that cannot be
 * saved (BUG-RO4WSP1). A write that succeeds clears the flag, so a workspace
 * made writable again does not need a reload.
 *
 * The flag is also read at start from the tenant status endpoint, so the
 * read-only banner shows and create buttons are hidden before anyone tries a
 * write. A single-tenant server answers the endpoint with `active` and no
 * `read_only`, so the flag stays false; any failure leaves it alone.
 *
 * The same answer says whether the workspace is suspended or archived, and a
 * refusal of the status request itself (an unknown workspace, an ended
 * session) is recorded like any other. The app reads it before it loads
 * anything else, so a workspace that refuses everything gets no requests it
 * would only refuse.
 */
import { ref } from 'vue'
import { apiUrl } from './base'
import { TENANT_STATUS_PATH } from './tenantProvisioning'
import { problemCode } from '@/i18n/apiErrors'
import { trackAccess, trackTenantStatus } from './workspaceAccess'

export const workspaceReadOnly = ref(false)

const SAFE_METHODS = new Set(['get', 'head', 'options'])

/** Whether a response is the server's "this workspace is read-only". */
export function isReadOnlyResponse(status: number, data: unknown): boolean {
  if (status !== 403 || typeof data !== 'object' || data === null) return false
  return problemCode((data as { type?: string }).type) === 'tenant_read_only'
}

/** Updates {@link workspaceReadOnly} from one API response. */
export function trackWorkspaceReadOnly(
  req: { method?: string; url?: string },
  status: number,
  data: unknown
): void {
  if (isReadOnlyResponse(status, data)) {
    workspaceReadOnly.value = true
    return
  }
  const write = !SAFE_METHODS.has((req.method ?? 'get').toLowerCase())
  // A read-only workspace still accepts the live-update subscriptions PUT,
  // so its success says nothing about writes.
  const exempt = (req.url ?? '').includes('/_events/')
  if (status >= 200 && status < 300 && write && !exempt) {
    workspaceReadOnly.value = false
  }
}

/** How long the start-up read waits before the app loads without it. */
export const STATUS_TIMEOUT_MS = 2000

/**
 * Reads the tenant status endpoint: sets {@link workspaceReadOnly}, and
 * records a suspended, archived or refused workspace (see
 * `workspaceAccess.ts`). Any failure leaves the flag as it is: a refused
 * write still sets it. The read gives up after `timeoutMs`, so a status that
 * does not answer cannot keep the app from starting. A browser without
 * `AbortSignal.timeout` skips the read.
 */
export async function loadWorkspaceStatus(
  fetchFn: typeof fetch = (input, init) => fetch(input, init),
  timeoutMs = STATUS_TIMEOUT_MS
): Promise<void> {
  try {
    const res = await fetchFn(apiUrl(TENANT_STATUS_PATH), {
      credentials: 'same-origin',
      cache: 'no-store',
      signal: AbortSignal.timeout(timeoutMs),
    })
    // Read the body even when it is not used: an unread body keeps the
    // request open in the browser.
    const body = (await res.json().catch(() => null)) as {
      status?: unknown
      read_only?: unknown
    } | null
    if (!res.ok) {
      trackAccess(res.status, body)
      return
    }
    if (!body) return
    trackTenantStatus(body.status)
    if (body.status === 'active') workspaceReadOnly.value = body.read_only === true
  } catch {
    // No status to read: keep what the responses said.
  }
}
