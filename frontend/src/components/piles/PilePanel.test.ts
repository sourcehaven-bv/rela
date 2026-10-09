import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import { useSchemaStore, useUIStore } from '@/stores'
import { resetPilesState } from '@/composables/usePiles'
import { resetFlyout } from '@/composables/useFlyout'
import type { Pile } from '@/api/piles'
import PilePanel from './PilePanel.vue'

const api = vi.hoisted(() => ({
  listPiles: vi.fn(),
  getPile: vi.fn(),
  addPileItems: vi.fn(),
  removePileItems: vi.fn(),
}))
vi.mock('@/api/piles', async (orig) => ({
  ...(await orig<typeof import('@/api/piles')>()),
  ...api,
}))
vi.mock('@/api/transforms', () => ({ getTransforms: vi.fn().mockResolvedValue([]) }))
vi.mock('@/composables/useEvents', () => ({ useEvents: () => ({ on: () => {}, off: () => {} }) }))

const pileId = 'PIL-AAAA1111'

function pileWith(items: Pile['items']): Pile {
  return {
    id: pileId,
    name: 'Friday review',
    icon: 'layers',
    count: items.length,
    created: '',
    updated: '',
    items,
  }
}

const mixed = pileWith([
  { id: 'TKT-1', face: '', address: 'TKT-1', type: 'ticket', title: 'Quick entry' },
  { id: 'RISK-1', face: '', address: 'RISK-1', type: 'risk', title: 'Supplier lock-in' },
  { id: 'TKT-2', face: '', address: 'TKT-2', type: 'ticket', title: 'Collapsible column' },
  { id: 'POL-1', face: 'draft', address: 'POL-1@draft', type: 'policy', title: 'Access policy' },
])

let pinia: Pinia
let router: Router
const mounted: Array<{ unmount: () => void }> = []

async function mountPanel() {
  const schema = useSchemaStore()
  schema.setPiles({ piles_available: true, piles: null })
  schema.entityTypes.set('ticket', {
    label: 'Ticket',
    label_plural: 'Tickets',
    properties: {},
  } as never)
  schema.entityTypes.set('risk', { label: 'Risk', label_plural: 'Risks', properties: {} } as never)
  const wrapper = mount(PilePanel, {
    props: { pileId },
    global: { plugins: [pinia, PiniaColada, router] },
    attachTo: document.body,
  })
  mounted.push(wrapper)
  await flushPromises()
  return wrapper
}

beforeEach(async () => {
  pinia = createPinia()
  setActivePinia(pinia)
  router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:p(.*)*', component: { template: '<div/>' } }],
  })
  await router.push('/')
  Object.values(api).forEach((fn) => fn.mockReset())
  api.listPiles.mockResolvedValue({ piles: [mixed], icons: ['layers'] })
  api.getPile.mockResolvedValue(mixed)
  resetPilesState()
  resetFlyout()
})

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount())
})

describe('PilePanel', () => {
  it('groups items by type under the schema’s plural labels, in pile order', async () => {
    const wrapper = await mountPanel()
    const groups = wrapper.findAll('[data-testid="pile-group"]')
    expect(groups.map((g) => g.find('h3').text())).toEqual(['Tickets', 'Risks', 'policy'])
    expect(groups.map((g) => g.findAll('input[type="checkbox"]').length)).toEqual([2, 1, 1])
    expect(groups[0].text()).toContain('Quick entry')
    expect(groups[0].text()).toContain('Collapsible column')
    expect(wrapper.text()).toContain('4 items')
  })

  it('links Step through and each row to the item’s address in the pile scope', async () => {
    const wrapper = await mountPanel()
    const step = wrapper.find('[data-testid="pile-step-through"]')
    expect(step.attributes('href')).toBe(`/entity/ticket/TKT-1?from=pile&pile=${pileId}`)
    const policy = wrapper.findAll('a').find((a) => a.text() === 'Access policy')
    expect(policy?.attributes('href')).toBe(`/entity/policy/POL-1@draft?from=pile&pile=${pileId}`)
  })

  it('removes the ticked rows, then puts them back on Undo', async () => {
    api.removePileItems.mockResolvedValue(undefined)
    api.addPileItems.mockResolvedValue({ added: 2 })
    const toast = vi.spyOn(useUIStore(), 'showToast')
    const wrapper = await mountPanel()

    const boxes = wrapper.findAll('input[type="checkbox"]')
    await boxes[0].setValue(true)
    await boxes[1].setValue(true)

    expect(wrapper.find('[data-testid="pile-ticked"]').text()).toBe('2 selected')
    expect(wrapper.find('[data-testid="pile-step-through"]').exists()).toBe(false)
    const remove = wrapper.find('[data-testid="pile-remove"]')
    expect(remove.text()).toContain('Remove 2')

    api.getPile.mockResolvedValue(pileWith([mixed.items[1], mixed.items[3]]))
    await remove.trigger('click')
    await flushPromises()

    expect(api.removePileItems).toHaveBeenCalledWith(pileId, ['TKT-1', 'TKT-2'], undefined)
    expect(wrapper.find('[data-testid="pile-ticked"]').exists()).toBe(false)

    const [, message, , action] = toast.mock.calls[0]
    expect(message).toBe('2 items removed from Friday review')
    action!.onAction()
    await flushPromises()
    expect(api.addPileItems).toHaveBeenCalledWith(pileId, ['TKT-1', 'TKT-2'], undefined)
  })

  it('explains how to fill an empty pile', async () => {
    api.getPile.mockResolvedValue(pileWith([]))
    const wrapper = await mountPanel()
    expect(wrapper.text()).toContain('This pile is empty')
    expect(wrapper.text()).toContain('Add to pile')
    expect(wrapper.find('[data-testid="pile-step-through"]').exists()).toBe(false)
  })

  it('drops the pile it held when a refetch answers 404', async () => {
    const { ApiError } = await import('@/api/errors')
    const wrapper = await mountPanel()
    expect(wrapper.text()).toContain('Quick entry')

    api.getPile.mockRejectedValue(
      new ApiError('gone', { kind: 'http', status: 404, original: null })
    )
    const { useQueryCache } = await import('@pinia/colada')
    await useQueryCache()
      .invalidateQueries({ key: ['piles'] })
      .catch(() => {})
    await flushPromises()

    expect(wrapper.text()).not.toContain('Quick entry')
  })
})
