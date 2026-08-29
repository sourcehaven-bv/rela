import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'
import GanttView from './GanttView.vue'
import { useSchemaStore } from '@/stores/schema'
import type { GanttNode, GanttResponse } from '@/api/gantts'

const getGanttMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  getGantt: (...args: unknown[]) => getGanttMock(...args),
}))

const routeQuery = ref<Record<string, string>>({})
// push() must actually update the query, like a real router: the drill path
// lives in the URL, and a mock that swallowed the write would freeze the view
// on its initial scope and quietly pass every navigation test.
const routerPush = vi.fn((to: { query?: Record<string, string> } | string) => {
  if (typeof to === 'object' && to?.query) {
    routeQuery.value = Object.fromEntries(
      Object.entries(to.query).filter(([, v]) => v !== undefined),
    ) as Record<string, string>
  }
  return Promise.resolve()
})
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush }),
  useRoute: () => ({
    get query() {
      return routeQuery.value
    },
  }),
}))

function node(id: string, children: GanttNode[] = []): GanttNode {
  return {
    id,
    type: 'project',
    title: `Node ${id}`,
    planned: { start: '2026-01-01', end: '2026-06-01' },
    children,
  }
}

function forest(truncated = false): GanttResponse {
  return {
    roots: [node('A', [node('B', [node('C')])])],
    truncated,
  }
}

function mountGantt() {
  return mount(GanttView, { props: { id: 'plan' } })
}

describe('GanttView fetch policy', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const store = useSchemaStore()
    store.gantts.set('plan', {
      title: 'Plan',
      hierarchy: ['contains'],
      multi_parent: 'first',
      on_cycle: 'error',
      default_depth: 2,
      max_depth: 10,
      max_nodes: 2000,
      sources: { project: { start: 'planned_start', end: 'planned_end' } },
    })
    routeQuery.value = {}
    getGanttMock.mockReset()
    routerPush.mockClear()
  })

  it('fetches the full forest on mount and renders rows', async () => {
    getGanttMock.mockResolvedValue(forest())
    const w = mountGantt()
    await flushPromises()

    expect(getGanttMock).toHaveBeenCalledTimes(1)
    expect(getGanttMock).toHaveBeenCalledWith('plan', undefined)
    expect(w.text()).toContain('Node A')
    expect(w.text()).toContain('Node B')
    w.unmount()
  })

  it('drills client-side when the fetched tree can answer (no refetch)', async () => {
    getGanttMock.mockResolvedValue(forest(false))
    const w = mountGantt()
    await flushPromises()

    routeQuery.value = { path: 'B' }
    await flushPromises()

    // Untruncated and B is present locally — the fast path must not refetch.
    expect(getGanttMock).toHaveBeenCalledTimes(1)
    expect(w.text()).toContain('Node B')
    expect(w.text()).not.toContain('Node A ')
    w.unmount()
  })

  it('refetches with ?root= when the response was truncated', async () => {
    getGanttMock.mockResolvedValue(forest(true))
    const w = mountGantt()
    await flushPromises()

    getGanttMock.mockResolvedValue({ roots: [node('B', [node('C')])], truncated: false })
    routeQuery.value = { path: 'B' }
    await flushPromises()

    // Truncated data may be missing B's children server-side cut — the view
    // must go back to the server for the subtree ("drill in to see more").
    expect(getGanttMock).toHaveBeenCalledTimes(2)
    expect(getGanttMock).toHaveBeenLastCalledWith('plan', 'B')
    expect(w.text()).toContain('Node C')
    w.unmount()
  })

  it('refetches the full forest when drilling back above a scoped fetch', async () => {
    getGanttMock.mockResolvedValue(forest(true))
    const w = mountGantt()
    await flushPromises()

    getGanttMock.mockResolvedValue({ roots: [node('B')], truncated: false })
    routeQuery.value = { path: 'B' }
    await flushPromises()

    getGanttMock.mockResolvedValue(forest(true))
    routeQuery.value = {}
    await flushPromises()

    expect(getGanttMock).toHaveBeenCalledTimes(3)
    expect(getGanttMock).toHaveBeenLastCalledWith('plan', undefined)
    w.unmount()
  })

  it('shows the fetch error instead of an empty chart', async () => {
    getGanttMock.mockRejectedValue(new Error('boom'))
    const w = mountGantt()
    await flushPromises()

    expect(w.find('.gantt-error').exists()).toBe(true)
    w.unmount()
  })
})
