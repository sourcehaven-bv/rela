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
 * The flag is also read once at start from the tenant status endpoint, so the
 * read-only banner shows and create buttons are hidden before anyone tries a
 * write. A server that is not multi-tenant has no such endpoint; its 404
 * leaves the flag alone.
 */
import { ref } from 'vue'
import { apiUrl } from './base'
import { TENANT_STATUS_PATH } from './tenantProvisioning'
import { problemCode } from '@/i18n/apiErrors'

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

/**
 * Sets {@link workspaceReadOnly} from the tenant status endpoint. Any failure
 * leaves the flag as it is: a refused write still sets it.
 */
export async function loadWorkspaceReadOnly(
  fetchFn: typeof fetch = (input, init) => fetch(input, init)
): Promise<void> {
  try {
    const res = await fetchFn(apiUrl(TENANT_STATUS_PATH), {
      credentials: 'same-origin',
      cache: 'no-store',
    })
    if (!res.ok) return
    const body = (await res.json()) as { status?: unknown; read_only?: unknown }
    if (body.status === 'active') workspaceReadOnly.value = body.read_only === true
  } catch {
    // No status to read: keep what the responses said.
  }
}
