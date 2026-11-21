/**
 * Whether the server reported that this workspace is read-only.
 *
 * A read-only tenant refuses every write with a 403 `tenant_read_only`
 * problem. The API client sets this flag on any such refusal, from whichever
 * write it came: the whole workspace is read-only, not one form. A create form
 * then says so and offers no Create button instead of a form that cannot be
 * saved (BUG-RO4WSP1). A write that succeeds clears the flag, so a workspace
 * made writable again does not need a reload.
 */
import { ref } from 'vue'
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
