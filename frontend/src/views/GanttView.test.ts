import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { nextTick, ref } from 'vue'
import GanttView from './GanttView.vue'
import { useSchemaStore } from '@/stores/schema'
import type { GanttNode, GanttResponse } from '@/api/gantts'

const getGanttMock = vi.fn()
const getEntityMock = vi.fn()
const updateEntityMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  getGantt: (...args: unknown[]) => getGanttMock(...args),
  getEntity: (...args: unknown[]) => getEntityMock(...args),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
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
  return mount(GanttView, {
    props: { id: 'plan' },
    global: { plugins: [getActivePinia()!, PiniaColada] },
  })
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

  it('drops a fetch the user navigated away from before it landed', async () => {
    getGanttMock.mockResolvedValue(forest(true))
    const w = mountGantt()
    await flushPromises()

    // Drilling into B on truncated data fetches B's subtree; the user goes
    // back to "All work" before it answers. The forest already loaded answers
    // that without a fetch, so the late subtree must not replace it.
    let answer!: (r: GanttResponse) => void
    getGanttMock.mockReturnValue(new Promise<GanttResponse>((r) => (answer = r)))
    routeQuery.value = { path: 'B' }
    await flushPromises()
    routeQuery.value = {}
    await flushPromises()
    answer({ roots: [node('B', [node('C')])], truncated: false })
    await flushPromises()

    expect(w.find('.row[data-node-id="A"]').exists()).toBe(true)
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

  // An entity page's timeline tab pins the chart to its anchor (`scope: root`).
  // The URL's drill path then continues below the anchor, never above it.
  it('pins a root: fetches its subtree and keeps it out of ?path=', async () => {
    getGanttMock.mockResolvedValue({ roots: [node('B', [node('C', [node('D')])])], truncated: false })
    const w = mount(GanttView, { props: { id: 'plan', root: 'B' } })
    await flushPromises()
    expect(getGanttMock).toHaveBeenLastCalledWith('plan', 'B')
    expect(w.find('.crumb').text()).not.toBe('All work')

    await w.find('[data-node-id="C"] .bar').trigger('click')
    expect(routeQuery.value.path).toBe('C')
    w.unmount()
  })

  it('clicking the drilled root opens the entity instead of growing the path', async () => {
    getGanttMock.mockResolvedValue(forest(false))
    const w = mountGantt()
    await flushPromises()

    routeQuery.value = { path: 'B' }
    await flushPromises()
    routerPush.mockClear()

    // B is now the top row; it cannot drill into itself — its click opens
    // the entity page, and the drill path must not grow.
    await w.find('[data-node-id="B"] .bar').trigger('click')
    expect(routerPush).toHaveBeenCalledWith('/entity/project/B')
    expect(routeQuery.value.path).toBe('B')
    w.unmount()
  })

  it('tree name opens the entity; the bar drills', async () => {
    getGanttMock.mockResolvedValue(forest(false))
    const w = mountGantt()
    await flushPromises()
    routerPush.mockClear()

    // Name in the tree column → entity page, drill path untouched.
    await w.find('[data-node-id="A"] .tname').trigger('click')
    expect(routerPush).toHaveBeenCalledWith('/entity/project/A')
    expect(routeQuery.value.path).toBeUndefined()

    // Bar in the chart → drill.
    routerPush.mockClear()
    await w.find('[data-node-id="A"] .bar').trigger('click')
    await flushPromises()
    expect(routeQuery.value.path).toBe('A')
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

describe('GanttView containment-loop marker', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const store = useSchemaStore()
    store.gantts.set('plan', {
      title: 'Plan',
      hierarchy: ['contains'],
      multi_parent: 'first',
      on_cycle: 'mark',
      default_depth: 2,
      max_depth: 10,
      max_nodes: 2000,
      sources: { project: { start: 'planned_start', end: 'planned_end' } },
    })
    routeQuery.value = {}
    getGanttMock.mockReset()
    routerPush.mockClear()
  })

  it('flags only the node the loop closes onto, and renders the rest', async () => {
    const looped = node('B', [node('C')])
    looped.in_cycle = true
    getGanttMock.mockResolvedValue({ roots: [node('A', [looped])] })

    const w = mountGantt()
    await flushPromises()

    const flags = w.findAll('.cycle-flag')
    expect(flags).toHaveLength(1)
    expect(flags[0].attributes('aria-label')).toContain('containment loop')
    expect(flags[0].attributes('aria-label')).toContain('Node B')
    // The loop does not cost the view its other rows — the whole point of
    // mark over error. (Node C sits past default_depth, so it is collapsed
    // rather than absent; A and B are what this asserts.)
    expect(w.text()).toContain('Node A')
    expect(w.text()).toContain('Node B')
    w.unmount()
  })

  it('renders no marker when nothing is flagged', async () => {
    getGanttMock.mockResolvedValue(forest())
    const w = mountGantt()
    await flushPromises()

    expect(w.findAll('.cycle-flag')).toHaveLength(0)
    w.unmount()
  })
})

describe('GanttView scroll navigation', () => {
  // jsdom keeps scrollLeft at 0 and has no layout; a backing field lets the
  // test read what the view asked for.
  const scrolls = new WeakMap<Element, number>()
  const original = Object.getOwnPropertyDescriptor(Element.prototype, 'scrollLeft')
  const originalMatchMedia = window.matchMedia
  const originalClientWidth = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'clientWidth')
  // 880px chart: 280px tree column + 600px of visible timeline.
  const CHART_W = 880
  const VISIBLE_W = 600
  const day = (s: string) => Math.floor(Date.parse(s + 'T00:00:00Z') / 86_400_000)
  // forest() spans 2026-01-01..2026-06-01; forestSpan pads it by 6 days.
  const axisStart = day('2025-12-26')

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 2, 1, 12)) // local 2026-03-01
    Object.defineProperty(Element.prototype, 'scrollLeft', {
      configurable: true,
      get() {
        return scrolls.get(this) ?? 0
      },
      set(v: number) {
        scrolls.set(this, v)
      },
    })
    Object.defineProperty(HTMLElement.prototype, 'clientWidth', {
      configurable: true,
      get() {
        return (this as HTMLElement).classList.contains('chart') ? CHART_W : 0
      },
    })
    // Reduced motion: the buttons then set scrollLeft directly.
    window.matchMedia = ((q: string) => ({
      matches: true,
      media: q,
      addEventListener: () => {},
      removeEventListener: () => {},
    })) as unknown as typeof window.matchMedia
    setActivePinia(createPinia())
    useSchemaStore().gantts.set('plan', {
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
    getGanttMock.mockResolvedValue(forest())
  })

  afterEach(() => {
    vi.useRealTimers()
    if (original) Object.defineProperty(Element.prototype, 'scrollLeft', original)
    if (originalClientWidth) Object.defineProperty(HTMLElement.prototype, 'clientWidth', originalClientWidth)
    else delete (HTMLElement.prototype as { clientWidth?: number }).clientWidth
    window.matchMedia = originalMatchMedia
  })

  async function mounted() {
    const w = mountGantt()
    await flushPromises()
    await nextTick()
    return w
  }

  const chartScroll = (w: ReturnType<typeof mountGantt>) => w.get('.chart').element.scrollLeft

  it('gives every day the same width and labels every month', async () => {
    const w = await mounted()
    const days = day('2026-06-07') - axisStart
    expect(w.get('.chart').attributes('style')).toContain(`--timeline-w: ${days * 12}px`)
    expect(w.findAll('.tick').map((t) => t.text())).toEqual(["Jan '26", 'Feb', 'Mar', 'Apr', 'May', 'Jun'])
    w.unmount()
  })

  it('opens at today and steps one unit with the arrows', async () => {
    const w = await mounted()
    // Today sits in the middle of the visible timeline.
    const today = (day('2026-03-01') - axisStart) * 12 - VISIBLE_W / 2
    expect(chartScroll(w)).toBe(today)

    await w.get('[aria-label="Later"]').trigger('click')
    expect(chartScroll(w)).toBe(today + 30 * 12)
    await w.get('[aria-label="Earlier"]').trigger('click')
    await w.get('[aria-label="Earlier"]').trigger('click')
    expect(chartScroll(w)).toBe(today - 30 * 12)

    const now = w.findAll('button').find((b) => b.text() === 'Now')!
    await now.trigger('click')
    expect(chartScroll(w)).toBe(today)
    w.unmount()
  })

  it('keeps the day at the left edge when the zoom changes', async () => {
    const w = await mounted()
    w.get('.chart').element.scrollLeft = (day('2026-04-10') - axisStart) * 12
    const week = w.findAll('.zoom-seg button').find((b) => b.text() === 'week')!
    await week.trigger('click')
    await nextTick()
    await nextTick()
    expect(chartScroll(w)).toBe((day('2026-04-10') - axisStart) * 40)
    w.unmount()
  })

  it('draws a bar to the end of its end day', async () => {
    getGanttMock.mockResolvedValue({
      // B widens the axis past the screen, so days keep their 40px.
      roots: [
        { ...node('A'), planned: { start: '2026-03-02', end: '2026-03-06' } },
        { ...node('B'), planned: { start: '2026-01-01', end: '2026-06-01' } },
      ],
      truncated: false,
    })
    const w = await mounted()
    await w.findAll('.zoom-seg button').find((b) => b.text() === 'week')!.trigger('click')
    const timeline = parseFloat(w.get('.chart').attributes('style')!.match(/--timeline-w: ([\d.]+)px/)![1])
    const bar = w.get('.row[data-node-id="A"] .bar')
    const width = parseFloat(bar.attributes('style')!.match(/width: ([\d.]+)%/)![1])
    // Monday to Friday is five days of 40px, not four.
    expect((width / 100) * timeline).toBeCloseTo(5 * 40, 3)
    w.unmount()
  })

  it('compresses a span too long to draw and says so', async () => {
    // A typo: 2062 for 2026. At 40px a day this would be ~525,000px wide.
    getGanttMock.mockResolvedValue({
      roots: [{ ...node('A'), planned: { start: '2026-01-01', end: '2062-01-01' } }],
      truncated: false,
    })
    const w = await mounted()
    await w.findAll('.zoom-seg button').find((b) => b.text() === 'week')!.trigger('click')
    const timeline = parseFloat(w.get('.chart').attributes('style')!.match(/--timeline-w: ([\d.]+)px/)![1])
    expect(timeline).toBeLessThanOrEqual(250_000)
    expect(w.text()).toContain('compressed')
    // Labels are thinned to stay readable: at least 56px apart.
    const ticks = w.findAll('.tick')
    expect(ticks.length).toBeLessThanOrEqual(250_000 / 56)
    w.unmount()
  })

  it('disables Now when today is far outside the plan', async () => {
    vi.setSystemTime(new Date(2027, 5, 1, 12))
    const w = await mounted()
    const now = w.findAll('button').find((b) => b.text() === 'Now')!
    expect(now.attributes('disabled')).toBeDefined()
    expect(w.text()).toContain('Today is outside this plan')
    expect(chartScroll(w)).toBe(0)
    w.unmount()
  })
})

describe('GanttView drag', () => {
  const stored = {
    id: 'A',
    type: 'project',
    properties: { planned_start: '2026-01-01', planned_end: '2026-06-01' },
    _versions: { properties: { planned_start: 's1', planned_end: 'e1' }, content: 'c' },
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    useSchemaStore().gantts.set('plan', {
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
    getGanttMock.mockReset().mockResolvedValue({ roots: [node('A')] })
    getEntityMock.mockReset()
    updateEntityMock.mockReset().mockResolvedValue({})
    routerPush.mockClear()
  })

  async function focusLabel(w: ReturnType<typeof mountGantt>) {
    await w.find('.row[data-node-id="A"] .bar-name').trigger('focus')
    await flushPromises()
  }

  it('shows no handles until the entity allows the write', async () => {
    getEntityMock.mockResolvedValue({ ...stored, _actions: { update: false } })
    const w = mountGantt()
    await flushPromises()
    expect(w.find('[data-testid="gantt-drag-window"]').exists()).toBe(false)
    await focusLabel(w)
    expect(getEntityMock).toHaveBeenCalledWith('project', 'A')
    expect(w.find('[data-testid="gantt-drag-window"]').exists()).toBe(false)
    w.unmount()
  })

  it('offers start, move and end sliders and writes from the keyboard', async () => {
    getEntityMock.mockResolvedValue(stored)
    const w = mountGantt()
    await flushPromises()
    await focusLabel(w)
    const handles = w.findAll('[role="slider"]')
    expect(handles.map((h) => h.attributes('data-testid'))).toEqual([
      'gantt-drag-start',
      'gantt-drag-move',
      'gantt-drag-end',
    ])
    expect(handles[0].attributes('aria-valuetext')).toBe('2026-01-01')

    const move = w.find('[data-testid="gantt-drag-move"]')
    await move.trigger('keydown', { key: 'ArrowRight' })
    expect(w.find('[data-testid="gantt-drag-start"]').attributes('aria-valuetext')).toBe(
      '2026-01-02'
    )
    expect(updateEntityMock).not.toHaveBeenCalled()

    getGanttMock.mockResolvedValue({ roots: [node('A')] })
    await move.trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(updateEntityMock).toHaveBeenCalledWith('project', 'A', {
      properties: { planned_start: '2026-01-02', planned_end: '2026-06-02' },
      preconditions: { properties: { planned_start: 's1', planned_end: 'e1' } },
    })
    expect(getGanttMock).toHaveBeenCalledTimes(2)
    w.unmount()
  })

  it('keeps focus on the handle through the write and reload', async () => {
    getEntityMock.mockResolvedValue(stored)
    const w = mount(GanttView, {
      props: { id: 'plan' },
      global: { plugins: [getActivePinia()!, PiniaColada] },
      attachTo: document.body,
    })
    await flushPromises()
    await focusLabel(w)
    const move = w.find('[data-testid="gantt-drag-move"]')
    ;(move.element as HTMLElement).focus()
    await move.trigger('keydown', { key: 'ArrowRight' })
    await move.trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(updateEntityMock).toHaveBeenCalledTimes(1)
    expect(document.activeElement?.getAttribute('data-testid')).toBe('gantt-drag-move')
    w.unmount()
  })

  it('reloads the scope navigated to, not the one the write began in', async () => {
    getEntityMock.mockResolvedValue(stored)
    getGanttMock.mockResolvedValue({ roots: [node('A', [node('B')])], truncated: true })
    const w = mountGantt()
    await flushPromises()
    await focusLabel(w)
    let written!: (v: unknown) => void
    updateEntityMock.mockReturnValueOnce(new Promise((r) => (written = r)))
    const move = w.find('[data-testid="gantt-drag-move"]')
    await move.trigger('keydown', { key: 'ArrowRight' })
    await move.trigger('keydown', { key: 'Enter' })
    await flushPromises()

    // Drill while the write is in flight; the truncated tree makes it fetch,
    // and that fetch is still out when the write lands.
    let drilled!: (r: GanttResponse) => void
    getGanttMock.mockReturnValueOnce(new Promise<GanttResponse>((r) => (drilled = r)))
    getGanttMock.mockResolvedValue({ roots: [node('B')] })
    routeQuery.value = { path: 'B' }
    await flushPromises()
    written({})
    await flushPromises()
    drilled({ roots: [node('B')] })
    await flushPromises()

    expect(getGanttMock).toHaveBeenLastCalledWith('plan', 'B')
    expect(w.find('.row[data-node-id="A"]').exists()).toBe(false)
    w.unmount()
  })

  it('does not drill when a drag ends on the window', async () => {
    getEntityMock.mockResolvedValue(stored)
    getGanttMock.mockResolvedValue({ roots: [node('A', [node('B')])] })
    const w = mountGantt()
    await flushPromises()
    await focusLabel(w)
    const move = w.find('[data-testid="gantt-drag-move"]')
    const ev = (type: string, clientX: number) => {
      const e = new MouseEvent(type, { clientX, button: 0, bubbles: true })
      Object.defineProperty(e, 'pointerId', { value: 1 })
      return e
    }
    move.element.dispatchEvent(ev('pointerdown', 100))
    move.element.dispatchEvent(ev('pointermove', 400))
    move.element.dispatchEvent(ev('pointerup', 400))
    move.element.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()
    expect(routerPush).not.toHaveBeenCalled()
    expect(updateEntityMock).toHaveBeenCalledTimes(1)
    w.unmount()
  })

  it('drills on a click on the draggable window', async () => {
    getEntityMock.mockResolvedValue(stored)
    getGanttMock.mockResolvedValue({ roots: [node('A', [node('B')])] })
    const w = mountGantt()
    await flushPromises()
    await focusLabel(w)
    await w.find('[data-testid="gantt-drag-window"]').trigger('click')
    expect(routerPush).toHaveBeenCalled()
    w.unmount()
  })
})
