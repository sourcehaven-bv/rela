import { computed, ref, type ComputedRef, type Ref } from 'vue'
import { listFlows, type Flow } from '@/api/flows'

/**
 * The flows of flows.yaml, read once per page load and shared by the
 * sidebar, the entity page and the flow views (TKT-ZAXRXND).
 *
 * flows.yaml is configuration, not a secret, so the list is the same for
 * every principal and one copy serves the whole app. Any failure reads as
 * "no flows": a project without flows answers 404, and an entity page must
 * not break on a flow route that is down. A later call after a failure
 * reads again.
 */

const flows = ref<Flow[]>([])
const loaded = ref(false)
let inflight: Promise<Flow[]> | null = null

async function read(): Promise<Flow[]> {
  try {
    flows.value = await listFlows()
    loaded.value = true
  } catch {
    flows.value = []
    inflight = null
  }
  return flows.value
}

/** Loads the flow list, once. Resolves to the flows, `[]` when there are none. */
export function loadFlowList(): Promise<Flow[]> {
  inflight ??= read()
  return inflight
}

/** Forgets the list, for tests and a config reload. */
export function resetFlowList(): void {
  flows.value = []
  loaded.value = false
  inflight = null
}

export interface FlowList {
  flows: Ref<Flow[]>
  /** The list has loaded; false while it loads and after a failure. */
  loaded: Ref<boolean>
  /** The flow called `name`. */
  flowNamed: (name: string) => Flow | undefined
  hasFlows: ComputedRef<boolean>
}

/** The shared flow list; starts the load on first use. */
export function useFlowList(): FlowList {
  void loadFlowList()
  return {
    flows,
    loaded,
    flowNamed: (name) => flows.value.find((f) => f.name === name),
    hasFlows: computed(() => flows.value.length > 0),
  }
}
