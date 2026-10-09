import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { defineComponent, reactive } from 'vue'
import { ApiError } from '@/api/errors'
import { useSchemaStore, useUIStore } from '@/stores'
import { resetFlyout, useFlyout } from './useFlyout'
import { PILES_EVENT_DEBOUNCE_MS, pileErrorMessage, resetPilesState, usePiles } from './usePiles'
import { knownPileName } from './pileNames'
import { PILE_REQUEST_MAX_ITEMS } from '@/api/piles'

const api = vi.hoisted(() => ({
  listPiles: vi.fn(),
  createPile: vi.fn(),
  updatePile: vi.fn(),
  deletePile: vi.fn(),
  addPileItems: vi.fn(),
  removePileItems: vi.fn(),
  getPile: vi.fn(),
}))
vi.mock('@/api/piles', async (orig) => ({
  ...(await orig<typeof import('@/api/piles')>()),
  ...api,
}))

const route = reactive<{ path: string; query: Record<string, string> }>({ path: '/', query: {} })
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ push: vi.fn() }) }))

const handlers: Array<() => void> = []
vi.mock('@/composables/useEvents', () => ({
  useEvents: () => ({
    on: (type: string, handler: () => void) => {
      if (type === 'entity:changed') handlers.push(handler)
    },
    off: () => {},
  }),
}))

function problem(status: number, code: string, title = 'Refused'): ApiError {
  return new ApiError(title, {
    kind: 'http',
    status,
    problem: { type: `https://rela.dev/errors/${code}`, title, status },
    original: null,
  })
}

const friday = {
  id: 'PIL-AAAA1111',
  name: 'Friday review',
  icon: 'layers',
  count: 2,
  created: '',
  updated: '',
}

let pinia: Pinia
const mounted: Array<{ unmount: () => void }> = []

function mountPiles(available = true) {
  useSchemaStore().setPiles({ piles_available: available, piles: null })
  let piles!: ReturnType<typeof usePiles>
  const Host = defineComponent({
    setup() {
      piles = usePiles()
      return () => null
    },
  })
  const wrapper = mount(Host, { global: { plugins: [pinia, PiniaColada] } })
  mounted.push(wrapper)
  return () => piles
}

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  Object.values(api).forEach((fn) => fn.mockReset())
  api.listPiles.mockResolvedValue({ piles: [friday], icons: ['layers', 'star'] })
  handlers.length = 0
  route.path = '/'
  route.query = {}
  resetPilesState()
  resetFlyout()
})

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount())
  vi.useRealTimers()
})

describe('usePiles', () => {
  it('fetches nothing when piles are not available', async () => {
    const piles = mountPiles(false)
    await flushPromises()
    expect(api.listPiles).not.toHaveBeenCalled()
    expect(piles().piles.value).toEqual([])
  })

  it('lists the piles in the page world and remembers their names', async () => {
    route.query = { world: 'published' }
    useSchemaStore().worlds.set('published', { readable: true } as never)
    const piles = mountPiles()
    await flushPromises()
    expect(api.listPiles).toHaveBeenCalledWith('published', expect.anything())
    expect(piles().piles.value).toEqual([friday])
    expect(piles().icons.value).toEqual(['layers', 'star'])
    expect(knownPileName(friday.id)).toBe('Friday review')
  })

  it('does not refetch on navigation', async () => {
    mountPiles()
    await flushPromises()
    const before = api.listPiles.mock.calls.length
    route.path = '/list/tickets'
    await flushPromises()
    expect(api.listPiles.mock.calls.length).toBe(before)
  })

  it('refetches once after a burst of entity changes settles', async () => {
    vi.useFakeTimers()
    mountPiles()
    mountPiles()
    await flushPromises()
    const before = api.listPiles.mock.calls.length

    handlers.forEach((h) => h())
    handlers.forEach((h) => h())
    await vi.advanceTimersByTimeAsync(PILES_EVENT_DEBOUNCE_MS - 1)
    expect(api.listPiles.mock.calls.length).toBe(before)

    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()
    expect(api.listPiles.mock.calls.length).toBe(before + 1)
  })

  it('creates a pile with items and refetches', async () => {
    api.createPile.mockResolvedValue({
      ...friday,
      id: 'PIL-NEW00001',
      name: 'New',
      count: 2,
      items: [],
    })
    const success = vi.spyOn(useUIStore(), 'showToast')
    const piles = mountPiles()
    await flushPromises()
    const before = api.listPiles.mock.calls.length

    const pile = await piles().create({
      name: 'New',
      icon: 'star',
      items: ['TKT-1', 'POL-1@draft'],
    })
    await flushPromises()

    expect(pile?.id).toBe('PIL-NEW00001')
    expect(api.createPile).toHaveBeenCalledWith(
      { name: 'New', icon: 'star', items: ['TKT-1', 'POL-1@draft'] },
      undefined
    )
    expect(success).toHaveBeenCalledWith(
      'success',
      'Pile New created with 2 items',
      5000,
      expect.any(Object)
    )
    expect(api.listPiles.mock.calls.length).toBe(before + 1)
  })

  it('says when a selection was cut to the request limit', async () => {
    const selected = Array.from({ length: PILE_REQUEST_MAX_ITEMS + 12 }, (_, i) => `TKT-${i + 1}`)
    api.addPileItems.mockResolvedValueOnce({ added: PILE_REQUEST_MAX_ITEMS })
    api.createPile.mockResolvedValueOnce({ ...friday, id: 'PIL-NEW00002', name: 'Big', count: 500, items: [] })
    const toast = vi.spyOn(useUIStore(), 'showToast')
    const piles = mountPiles()

    await piles().addItems(friday, selected)
    expect(api.addPileItems.mock.calls[0][1]).toHaveLength(PILE_REQUEST_MAX_ITEMS)
    expect(toast).toHaveBeenCalledWith(
      'success',
      '500 added to Friday review. Only the first 500 of 512 selected items were sent.',
      5000,
      expect.any(Object)
    )

    await piles().create({ name: 'Big', items: selected })
    expect(api.createPile.mock.calls[0][0].items).toHaveLength(PILE_REQUEST_MAX_ITEMS)
    expect(toast).toHaveBeenCalledWith(
      'success',
      'Pile Big created with 500 items. Only the first 500 of 512 selected items were sent.',
      5000,
      expect.any(Object)
    )
  })

  it('reports a taken name and resolves to null', async () => {
    api.createPile.mockRejectedValue(problem(409, 'pile_name_taken'))
    const error = vi.spyOn(useUIStore(), 'error')
    const piles = mountPiles()

    expect(await piles().create({ name: 'Friday review' })).toBeNull()
    expect(error).toHaveBeenCalledWith('You already have a pile with that name')
  })

  it('reports how many were added, and an unreadable item without naming it', async () => {
    api.addPileItems.mockResolvedValueOnce({ added: 1 })
    const toast = vi.spyOn(useUIStore(), 'showToast')
    const error = vi.spyOn(useUIStore(), 'error')
    const piles = mountPiles()

    expect(await piles().addItems(friday, ['TKT-1', 'TKT-2'])).toBe(1)
    expect(toast).toHaveBeenCalledWith(
      'success',
      '1 added to Friday review (1 already on it)',
      5000,
      expect.any(Object)
    )

    api.addPileItems.mockRejectedValueOnce(problem(404, 'item_not_found'))
    expect(await piles().addItems(friday, ['TKT-404'])).toBeNull()
    expect(error).toHaveBeenCalledWith('Some items are not available')
  })

  it('opens the pile from the toast after an add', async () => {
    api.addPileItems.mockResolvedValue({ added: 1 })
    const toast = vi.spyOn(useUIStore(), 'showToast')
    const piles = mountPiles()

    await piles().addItems(friday, ['TKT-1'])
    toast.mock.calls[0][3]!.onAction()

    expect(useFlyout().pile.value?.pileId).toBe(friday.id)
  })

  it('removes items and re-adds exactly those on Undo', async () => {
    api.removePileItems.mockResolvedValue(undefined)
    api.addPileItems.mockResolvedValue({ added: 2 })
    const toast = vi.spyOn(useUIStore(), 'showToast')
    const piles = mountPiles()

    expect(await piles().removeItems(friday, ['TKT-1', 'POL-1@draft'])).toBe(true)
    expect(api.removePileItems).toHaveBeenCalledWith(friday.id, ['TKT-1', 'POL-1@draft'], undefined)
    const [, message, , action] = toast.mock.calls[0]
    expect(message).toBe('2 items removed from Friday review')
    expect(action?.label).toBe('Undo')

    action!.onAction()
    await flushPromises()
    expect(api.addPileItems).toHaveBeenCalledWith(friday.id, ['TKT-1', 'POL-1@draft'], undefined)
  })

  it('deletes a pile and closes it when it is open', async () => {
    api.deletePile.mockResolvedValue(undefined)
    const piles = mountPiles()
    useFlyout().openPile({ navId: `pile:${friday.id}`, title: friday.name, pileId: friday.id })

    expect(await piles().remove(friday)).toBe(true)
    expect(api.deletePile).toHaveBeenCalledWith(friday.id)
    expect(useFlyout().pile.value).toBeNull()
  })
})

describe('pileErrorMessage', () => {
  it.each([
    ['pile_limit', 409, 'You have reached the maximum number of piles. Delete one to make room.'],
    ['item_not_found', 404, 'Some items are not available'],
    ['pile_name_taken', 409, 'You already have a pile with that name'],
  ])('maps %s', (code, status, want) => {
    expect(pileErrorMessage(problem(status, code), 'fallback')).toBe(want)
  })

  it('keeps the server message for an ambiguous address, which names the faces', () => {
    const err = problem(409, 'ambiguous_address', 'POL-1 has several faces: draft, published')
    expect(pileErrorMessage(err, 'fallback')).toBe('POL-1 has several faces: draft, published')
  })
})
