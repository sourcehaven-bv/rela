import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import { resetPilesState } from '@/composables/usePiles'
import type { Entity, ListResponse } from '@/types'

// "Add to pile" in the list's bulk bar (TKT-K3RJLH): offered only when the
// server says piles are available, and it adds the selected rows' ADDRESSES.

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

const piles = vi.hoisted(() => ({ listPiles: vi.fn(), addPileItems: vi.fn() }))
vi.mock('@/api/piles', async (orig) => ({
  ...(await orig<typeof import('@/api/piles')>()),
  ...piles,
}))

const mockRoute = { query: {} as Record<string, string>, path: '/list/policies', name: 'list' }
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => mockRoute,
}))

const listId = 'policies'
const friday = {
  id: 'PIL-AAAA1111',
  name: 'Friday review',
  icon: 'layers',
  count: 0,
  created: '',
  updated: '',
}

// Read-only rows: nothing else makes the list selectable, so a checkbox
// appears only because of piles.
function row(id: string, face?: string): Entity {
  return {
    id,
    type: 'policy',
    properties: { title: `Title ${id}` },
    _actions: { update: false, delete: false },
    ...(face ? { _self: `/api/v1/policies/${id}@${face}` } : {}),
  } as Entity
}

async function mountList(available: boolean, rows: Entity[]) {
  const schema = useSchemaStore()
  schema.setPiles({ piles_available: available, piles: null })
  schema.lists.set(listId, {
    id: listId,
    title: 'Policies',
    entity: 'policy',
    columns: [{ property: 'title', label: 'Title' }],
  } as never)
  schema.entityTypes.set('policy', {
    name: 'policy',
    label: 'policy',
    properties: { title: { type: 'string', values: null } },
  } as never)
  const response: ListResponse<Entity> = {
    data: rows,
    meta: { total: rows.length, page: 1, per_page: 25, has_more: false },
    included: {},
  }
  listEntitiesMock.mockResolvedValue(response)
  const wrapper = mount(EntityList, {
    props: { listId },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

let pinia: ReturnType<typeof createPinia>

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest('policy', 'policies')
  listEntitiesMock.mockReset()
  piles.listPiles.mockReset().mockResolvedValue({ piles: [friday], icons: ['layers'] })
  piles.addPileItems.mockReset().mockResolvedValue({ added: 2 })
  resetPilesState()
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('EntityList: Add to pile', () => {
  it('offers no selection for read-only rows when piles are not available', async () => {
    const wrapper = await mountList(false, [row('POL-1'), row('POL-2')])
    expect(wrapper.find('.rl-table-row input[type="checkbox"]').exists()).toBe(false)
    expect(piles.listPiles).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('adds the selected rows by address to the chosen pile', async () => {
    const wrapper = await mountList(true, [row('POL-1', 'draft'), row('POL-2'), row('POL-3')])

    const box = (id: string) =>
      wrapper.find(`.rl-table-row[data-entity-id="${id}"] input[type="checkbox"]`)
    await box('POL-1').setValue(true)
    await box('POL-3').setValue(true)
    await flushPromises()

    const menu = wrapper.find('[data-testid="add-to-pile"]')
    expect(menu.exists()).toBe(true)
    await menu.find('button').trigger('click')
    await flushPromises()

    const item = document.body.querySelector('[data-testid="add-to-pile-item"]') as HTMLElement
    expect(item.textContent).toContain('Friday review')
    item.click()
    await flushPromises()

    expect(piles.addPileItems).toHaveBeenCalledWith(friday.id, ['POL-1@draft', 'POL-3'], undefined)
    // The selection is spent.
    expect((box('POL-1').element as HTMLInputElement).checked).toBe(false)
    expect((box('POL-3').element as HTMLInputElement).checked).toBe(false)
    wrapper.unmount()
  })
})
