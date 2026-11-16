/**
 * Tenant provisioning (TKT-TNPRV8).
 *
 * In a multi-tenant rela-server, the first request of an organization that
 * has no tenant yet may start creating one. The server does not hold that
 * request open: it answers `202` with a `tenant_provisioning` problem and a
 * Location naming the status endpoint. A create that is throttled or failed
 * is a `503` with `tenant_provisioning_busy` or `tenant_provisioning_failed`.
 * On any of these the SPA shows a "setting up" screen, polls the endpoint,
 * and reloads once it reports `ready`.
 */
import { ref } from 'vue'
import { apiUrl } from './base'
import { problemCode } from '@/i18n/apiErrors'

/** Where the SPA's view of provisioning stands. */
export type ProvisioningState = 'idle' | 'provisioning' | 'failed'

/** Drives the App's setting-up screen. */
export const tenantProvisioning = ref<ProvisioningState>('idle')

/** The status endpoint's path, as the server serves it. */
export const TENANT_STATUS_PATH = '/api/v1/_tenant/status'

const POLL_MS = 2000
const FAILED_POLL_MS = 30000

const PROBLEMS: Record<number, string[]> = {
  202: ['tenant_provisioning'],
  503: ['tenant_provisioning_busy', 'tenant_provisioning_failed'],
}

/** Whether a response is the server's "your organization is being set up". */
export function isProvisioningResponse(status: number, data: unknown): boolean {
  const codes = PROBLEMS[status]
  if (!codes || typeof data !== 'object' || data === null) return false
  const code = problemCode((data as { type?: string }).type)
  return code !== undefined && codes.includes(code)
}

let polling = false

/**
 * Starts polling the status endpoint once, however many requests saw the
 * `202`. Reloads the page when the tenant is ready. Exported hooks make it
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
        // Ready, or a refusal polling cannot fix (a session that ended, an
        // org the server does not serve): reload, so the app handles it.
        if ((res.ok && body.status === 'ready') || res.status === 401 || res.status === 403) {
          polling = false
          reload()
          return
        }
        if (res.status === 202 || body.status === 'provisioning') {
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
