import { computed, effectScope, ref, watch, type ComputedRef, type Ref } from 'vue'
import { listFlows, type Flow } from '@/api/flows'
import { useSchemaStore } from '@/stores/schema'

/**
 * The flows of flows.yaml, read once per page load and shared by the
 * sidebar, the entity page and the flow views (TKT-ZAXRXND).
 *
 * flows.yaml is configuration, not a secret, so the list is the same for
 * every principal and one copy serves the whole app. A project without
 * flows says so in `capabilities.flows` of `/_schema`, and then the list is
 * not asked for: the route would answer 404 on every page. Any failure reads
 * as "no flows", so an entity page does not break on a flow route that is
 * down. A later call after a failure reads again.
 */

const flows = ref<Flow[]>([])
const loaded = ref(false)
let inflight: Promise<Flow[]> | null = null

type SchemaStore = ReturnType<typeof useSchemaStore>

/**
 * Resolves once the schema has loaded. It does not start the load: App.vue
 * does, as for `capabilities.git`. The watch lives in a detached scope, so
 * it outlives the component that asked first.
 */
function schemaLoaded(schema: SchemaStore): Promise<void> {
  if (schema.loaded) return Promise.resolve()
  return new Promise((resolve) => {
    const scope = effectScope(true)
    scope.run(() =>
      watch(
        () => schema.loaded,
        (done) => {
          if (!done) return
          scope.stop()
          resolve()
        }
      )
    )
  })
}

async function read(): Promise<Flow[]> {
  const schema = useSchemaStore()
  try {
    await schemaLoaded(schema)
    flows.value = schema.flowsEnabled ? await listFlows() : []
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
