import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { useSchemaStore } from '@/stores'
import { resetPilesState, usePiles } from '@/composables/usePiles'
import type { PileSummary } from '@/api/piles'
import PileToggles from './PileToggles.vue'

const api = vi.hoisted(() => ({
  listPiles: vi.fn(),
  addPileItems: vi.fn(),
  removePileItems: vi.fn(),
  markPilesOpened: vi.fn(),
}))
vi.mock('@/api/piles', async (orig) => ({
  ...(await orig<typeof import('@/api/piles')>()),
  ...api,
}))
vi.mock('@/composables/useEvents', () => ({ useEvents: () => ({ on: () => {}, off: () => {} }) }))

const declared = (over: Partial<PileSummary>): PileSummary => ({
  id: 'PIL-FAV',
  name: 'Favorites',
  icon: 'star',
  count: 0,
  created: '',
  updated: '',
  key: 'favorites',
  locked: true,
  addresses: [],
  ...over,
})

let pinia: Pinia
const mounted: Array<{ unmount: () => void }> = []

async function mountToggles(address: string, entityType = 'ticket') {
  const schema = useSchemaStore()
  schema.setPiles({
    piles_available: true,
    piles: {
      actions: [],
      export: [],
      toggles: [
        { pile: 'favorites', icon_active: 'heart' },
        { pile: 'inbox', entity_types: ['risk'] },
      ],
    },
  })
  const wrapper = mount(PileToggles, {
    props: { address, entityType },
    global: { plugins: [pinia, PiniaColada] },
  })
  mounted.push(wrapper)
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  Object.values(api).forEach((fn) => fn.mockReset())
  api.addPileItems.mockResolvedValue({ added: 1 })
  resetPilesState()
})

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount())
})

describe('PileToggles', () => {
  it('adds the entity to the pile, and shows itself pressed while it is on it', async () => {
    api.listPiles.mockResolvedValue({ piles: [declared({})], icons: [] })
    const wrapper = await mountToggles('TKT-1')
    // The inbox toggle is limited to risks, and the user has no inbox anyway.
    const buttons = wrapper.findAll('[data-testid="pile-toggle"]')
    expect(buttons).toHaveLength(1)
    expect(buttons[0].attributes('aria-pressed')).toBe('false')
    expect(buttons[0].attributes('aria-label')).toBe('Add to Favorites')

    await buttons[0].trigger('click')
    await flushPromises()
    expect(api.addPileItems).toHaveBeenCalledWith('PIL-FAV', ['TKT-1'])
  })

  it('takes the entity off the pile when it is on it', async () => {
    api.listPiles.mockResolvedValue({ piles: [declared({ addresses: ['TKT-1'] })], icons: [] })
    const wrapper = await mountToggles('TKT-1')
    const button = wrapper.get('[data-testid="pile-toggle"]')
    expect(button.attributes('aria-pressed')).toBe('true')
    await button.trigger('click')
    await flushPromises()
    expect(api.removePileItems).toHaveBeenCalledWith('PIL-FAV', ['TKT-1'])
  })

  it('reports the entity opened only when it is unread on an auto-read pile', async () => {
    const inbox = declared({
      id: 'PIL-INBOX',
      key: 'inbox',
      name: 'Notifications',
      read_state: true,
      auto_read: true,
      unread_addresses: ['RISK-1'],
    })
    api.listPiles.mockResolvedValue({ piles: [inbox], icons: [] })
    await mountToggles('TKT-1')
    expect(api.markPilesOpened).not.toHaveBeenCalled()

    await mountToggles('RISK-1', 'risk')
    expect(api.markPilesOpened).toHaveBeenCalledWith(['RISK-1'])
  })

  it('reads a notification that arrives while its entity is open', async () => {
    const inbox = declared({ id: 'PIL-INBOX', key: 'inbox', read_state: true, auto_read: true })
    api.listPiles.mockResolvedValue({ piles: [inbox], icons: [] })
    const wrapper = await mountToggles('RISK-1', 'risk')
    expect(api.markPilesOpened).not.toHaveBeenCalled()

    api.listPiles.mockResolvedValue({
      piles: [{ ...inbox, unread_addresses: ['RISK-1'] }],
      icons: [],
    })
    await wrapper.vm.$.appContext.app.runWithContext(() => usePiles().refresh())
    await flushPromises()
    expect(api.markPilesOpened).toHaveBeenCalledWith(['RISK-1'])
  })
})
