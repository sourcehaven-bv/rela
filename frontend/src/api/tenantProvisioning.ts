/**
 * Tenant provisioning (TKT-TNPRV8).
 *
 * In a multi-tenant rela-server, a request of an organization whose tenant
 * is still being created does not reach the tenant. A read is answered `202`
 * with `{"status": "provisioning"}`; a write is a `503` `tenant_provisioning`
 * problem. A create that is throttled, failed or needs an operator is a `503`
 * `provisioning_busy`, `provisioning_failed` or `provisioning_needs_operator`
 * problem. On any of these the SPA shows a "setting up" screen, polls the
 * status endpoint, and reloads once it reports `active`, or `suspended`, which
 * the reloaded app then reports as a refused organization.
 */
import { ref } from 'vue'
import { apiUrl } from './base'
import { problemCode } from '@/i18n/apiErrors'

/** Where the SPA's view of provisioning stands. */
export type ProvisioningState = 'idle' | 'provisioning' | 'failed'

/** Drives the App's setting-up screen. */
export const tenantProvisioning = ref<ProvisioningState>('idle')

/** The status endpoint's path, as the server serves it. */
export const TENANT_STATUS_PATH = '/api/v1/_tenant'

const POLL_MS = 2000
const FAILED_POLL_MS = 30000

const UNAVAILABLE_PROBLEMS = [
  'tenant_provisioning',
  'provisioning_busy',
  'provisioning_failed',
  'provisioning_needs_operator',
]

/** Whether a response is the server's "your organization is being set up". */
export function isProvisioningResponse(status: number, data: unknown): boolean {
  if (typeof data !== 'object' || data === null) return false
  if (status === 202) return (data as { status?: unknown }).status === 'provisioning'
  if (status !== 503) return false
  const code = problemCode((data as { type?: string }).type)
  return code !== undefined && UNAVAILABLE_PROBLEMS.includes(code)
}

let polling = false

/**
 * Starts polling the status endpoint once, however many requests saw the
 * `202`. Reloads the page when the tenant is active. Exported hooks make it
 * testable: `fetchFn`, `wait` and `reload` default to the browser's.
 */
export function startProvisioningPoll(
  deps: {
    fetchFn?: typeof fetch
    wait?: (ms: number) => Promise<void>
    reload?: () => void
  } = {}
): Promise<void> | undefined {
  if (polling) return undefined
  polling = true
  tenantProvisioning.value = 'provisioning'
  const fetchFn = deps.fetchFn ?? ((input, init) => fetch(input, init))
  const wait = deps.wait ?? ((ms: number) => new Promise<void>((r) => setTimeout(r, ms)))
  const reload = deps.reload ?? (() => window.location.reload())
  return (async () => {
    for (;;) {
      let delay = POLL_MS
      try {
        const res = await fetchFn(apiUrl(TENANT_STATUS_PATH), {
          credentials: 'same-origin',
          cache: 'no-store',
        })
        const body = (await res.json().catch(() => ({}))) as { status?: string; type?: string }
        // Active, or a state polling cannot fix (a suspended org, a session
        // that ended, an org the server does not serve): reload, so the app
        // handles it.
        const settled = body.status === 'active' || body.status === 'suspended'
        if ((res.ok && settled) || res.status === 401 || res.status === 403) {
          polling = false
          reload()
          return
        }
        if (body.status === 'provisioning') {
          tenantProvisioning.value = 'provisioning'
        } else {
          tenantProvisioning.value = 'failed'
          delay = FAILED_POLL_MS
        }
      } catch {
        // A network blip: keep polling.
      }
      await wait(delay)
    }
  })()
}

/** Resets module state between tests. */
export function resetProvisioningForTest(): void {
  polling = false
  tenantProvisioning.value = 'idle'
}
