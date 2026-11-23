import { describe, it, expect, vi, beforeEach } from 'vitest'
import { reactive } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import SankeyView from './SankeyView.vue'
import { useSchemaStore } from '@/stores/schema'
import { resetDetailPanel, useDetailPanelOutlet } from '@/composables/useDetailPanel'
import type { SankeyResponse } from '@/api/sankeys'

const getSankeyMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  getSankey: (...args: unknown[]) => getSankeyMock(...args),
}))

const routerPush = vi.fn()
const routeState = reactive<{ query: Record<string, string> }>({ query: {} })
const routerReplace = vi.fn(async (to: { query: Record<string, string> }) => {
  routeState.query = to.query
})
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush, replace: routerReplace }),
  useRoute: () => routeState,
}))

function response(truncated = false): SankeyResponse {
  return {
    nodes: [
      { id: 'R-1', type: 'risk', title: 'Phishing' },
      { id: 'R-2', type: 'risk', title: 'Loss' },
      { id: 'C-1', type: 'control', title: 'MFA', tone: 'done', sort_key: '1' },
      { id: 'C-2', type: 'control', title: 'Backups', tone: 'planned', sort_key: '2' },
    ],
    links: [
      { source: 'R-1', target: 'C-1', relation: 'mitigated-by' },
      { source: 'R-2', target: 'C-2', relation: 'mitigated-by' },
    ],
    truncated,
  }
}

function mountView() {
  return mount(SankeyView, { props: { id: 'flow' }, global: { plugins: [getActivePinia()!] } })
}

describe('SankeyView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useSchemaStore().sankeys.set('flow', {
      title: 'Flow',
      columns: [['risk'], ['control']],
      edges: ['mitigated-by'],
      types: { control: { order: 'num', color_by: 'status' } },
      max_nodes: 2000,
    })
    getSankeyMock.mockReset()
    routerPush.mockClear()
    routerReplace.mockClear()
    routeState.query = {}
    resetDetailPanel()
  })

  it('fetches once and renders a node and a ribbon per entity and link', async () => {
    getSankeyMock.mockResolvedValue(response())
    const w = mountView()
    await flushPromises()

    expect(getSankeyMock).toHaveBeenCalledWith('flow')
    expect(w.findAll('rect.node')).toHaveLength(4)
    expect(w.findAll('path.link')).toHaveLength(2)
    expect(w.text()).toContain('Phishing')
    expect(w.text()).toContain('Backups')
    w.unmount()
  })

  it('gives equal tones equal colours and different tones different ones', async () => {
    getSankeyMock.mockResolvedValue(response())
    const w = mountView()
    await flushPromises()

    const fill = (label: string) =>
      w
        .findAll('rect.node')
        .find((r) => r.attributes('aria-label')?.includes(label))!
        .attributes('fill')
    expect(fill('MFA')).not.toBe(fill('Backups'))
    w.unmount()
  })

  it('highlights the path of the hovered node and dims the rest', async () => {
    getSankeyMock.mockResolvedValue(response())
    const w = mountView()
    await flushPromises()

    const mfa = w.findAll('rect.node').find((r) => r.attributes('aria-label')?.includes('MFA'))!
    await mfa.trigger('mouseenter')

    expect(w.find('svg').classes()).toContain('dim')
    expect(w.findAll('g.node-group.on')).toHaveLength(2) // MFA and Phishing
    expect(w.findAll('path.link.on')).toHaveLength(1)

    await mfa.trigger('mouseleave')
    expect(w.find('svg').classes()).not.toContain('dim')
    w.unmount()
  })

  it('selects a node on click and on Enter, and opens it in the detail panel', async () => {
    getSankeyMock.mockResolvedValue(response())
    const w = mountView()
    await flushPromises()
    const outlet = useDetailPanelOutlet()
    expect(outlet.open.value).toBe(false)

    const node = w.findAll('rect.node').find((r) => r.attributes('aria-label')?.includes('MFA'))!
    await node.trigger('click')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({ query: { selected: 'C-1' } })
    expect(outlet.open.value).toBe(true)
    expect(outlet.content.value?.mode).toBe('overlay')
    expect(outlet.content.value?.props).toMatchObject({ entityType: 'control', entityId: 'C-1' })
    expect(w.findAll('g.node-group.selected')).toHaveLength(1)

    const other = w.findAll('rect.node').find((r) => r.attributes('aria-label')?.includes('Loss'))!
    await other.trigger('keydown.enter')
    expect(routerReplace).toHaveBeenLastCalledWith({ query: { selected: 'R-2' } })
    w.unmount()
  })

  it('closes the panel by dropping the selection, and expands to the entity page', async () => {
    getSankeyMock.mockResolvedValue(response())
    routeState.query = { selected: 'C-1', other: 'x' }
    const w = mountView()
    await flushPromises()
    const props = useDetailPanelOutlet().content.value!.props as {
      onClose: () => void
      onExpand: () => void
    }

    props.onExpand()
    expect(routerPush).toHaveBeenLastCalledWith('/entity/control/C-1')

    props.onClose()
    await flushPromises()
    expect(routerReplace).toHaveBeenLastCalledWith({ query: { other: 'x' } })
    expect(useDetailPanelOutlet().open.value).toBe(false)
    w.unmount()
  })

  it('addresses a faced node by its face', async () => {
    getSankeyMock.mockResolvedValue({
      nodes: [
        { id: 'P-1', type: 'policy', title: 'Policy', face: 'published' },
        { id: 'C-1', type: 'control', title: 'MFA' },
      ],
      links: [{ source: 'P-1', target: 'C-1', relation: 'x' }],
    })
    routeState.query = { selected: 'P-1' }
    const w = mountView()
    await flushPromises()
    expect(useDetailPanelOutlet().content.value?.props.entityId).toBe('P-1@published')
    w.unmount()
  })

  describe('pips', () => {
    function withPips(): SankeyResponse {
      const r = response()
      r.nodes[2].pips = [
        { id: 'EV-1', type: 'evidence', title: 'Audit report', tone: 'ok' },
        { id: 'EV-2', type: 'evidence', title: 'Screenshot', tone: 'stale' },
      ]
      r.nodes[2].pips_more = 3
      return r
    }

    it('draws a dot per pip beside its node, not a node, with the extra count', async () => {
      getSankeyMock.mockResolvedValue(withPips())
      const w = mountView()
      await flushPromises()

      expect(w.findAll('rect.node')).toHaveLength(4)
      expect(w.findAll('circle.pip')).toHaveLength(2)
      expect(w.find('.pips-more').text()).toBe('+3')
      const fills = w.findAll('circle.pip').map((c) => c.attributes('fill'))
      expect(fills[0]).not.toBe(fills[1])
      w.unmount()
    })

    it('opens the pip entity in the panel when its dot is clicked', async () => {
      getSankeyMock.mockResolvedValue(withPips())
      const w = mountView()
      await flushPromises()

      await w.findAll('circle.pip')[1].trigger('click')
      await flushPromises()
      expect(routerReplace).toHaveBeenLastCalledWith({ query: { selected: 'EV-2' } })
      expect(useDetailPanelOutlet().content.value?.props).toMatchObject({
        entityType: 'evidence',
        entityId: 'EV-2',
      })
      expect(w.findAll('circle.pip.selected')).toHaveLength(1)
      w.unmount()
    })

    it('names each dot and makes it keyboard focusable', async () => {
      getSankeyMock.mockResolvedValue(withPips())
      const w = mountView()
      await flushPromises()
      for (const c of w.findAll('circle.pip')) {
        expect(c.attributes('tabindex')).toBe('0')
        expect(c.attributes('aria-label')).toContain('evidence')
      }
      w.unmount()
    })

    it('draws nothing when no node has pips', async () => {
      getSankeyMock.mockResolvedValue(response())
      const w = mountView()
      await flushPromises()
      expect(w.find('circle.pip').exists()).toBe(false)
      expect(w.find('.pips-more').exists()).toBe(false)
      w.unmount()
    })
  })

  it('makes every node keyboard focusable and named', async () => {
    getSankeyMock.mockResolvedValue(response())
    const w = mountView()
    await flushPromises()

    for (const r of w.findAll('rect.node')) {
      expect(r.attributes('tabindex')).toBe('0')
      expect(r.attributes('aria-label')).toBeTruthy()
    }
    w.unmount()
  })

  it('flags a truncated response', async () => {
    getSankeyMock.mockResolvedValue(response(true))
    const w = mountView()
    await flushPromises()
    expect(w.find('.truncated-flag').exists()).toBe(true)
    w.unmount()
  })

  it('shows an empty state when nothing is visible', async () => {
    getSankeyMock.mockResolvedValue({ nodes: [], links: [] })
    const w = mountView()
    await flushPromises()
    expect(w.find('.sankey-empty').exists()).toBe(true)
    expect(w.find('svg').exists()).toBe(false)
    w.unmount()
  })

  it('shows the error when the fetch fails', async () => {
    getSankeyMock.mockRejectedValue(new Error('boom'))
    const w = mountView()
    await flushPromises()
    expect(w.find('.sankey-error').exists()).toBe(true)
    w.unmount()
  })
})
