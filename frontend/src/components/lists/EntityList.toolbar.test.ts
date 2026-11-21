import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import FilterBar from './FilterBar.vue'
import SortMenu from './SortMenu.vue'
import { withPageHeader } from '@/composables/pageHeaderTestHost'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity } from '@/types'

// The list's toolbar: a search box, a Filters drop-down for `filter_controls`,
// and a Sort menu. The inline filter row is gone, so the controls are reached
// through the header, and a sort picked there is the `sort` param the server
// orders the rows by.

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

const mockRoute = { query: {} as Record<string, unknown>, path: '/list/taken', name: 'list' }
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => mockRoute,
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

const listId = 'taken'
const entityType = 'taak'

function seedSchema(opts: { filterControls?: boolean; unsortable?: boolean } = {}) {
  const schemaStore = useSchemaStore()
  schemaStore.lists.set(listId, {
    id: listId,
    title: 'Taken',
    entity: entityType,
    columns: [
      { property: 'title', label: 'Titel' },
      { property: 'status', label: 'Status' },
      { property: 'notes', label: 'Notes', sortable: false },
    ],
    ...(opts.filterControls ? { filter_controls: [{ property: 'status', label: 'Status' }] } : {}),
  } as never)
  schemaStore.entityTypes.set(entityType, {
    name: entityType,
    label: 'Taak',
    properties: {
      title: { type: 'string', values: null },
      status: { type: 'enum', values: ['todo', 'done'] },
      notes: { type: 'string', values: null },
    },
  } as never)
}

const taak: Entity = { id: 'TAAK-1', type: entityType, properties: { title: 'Open werk', status: 'todo' } }

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest(entityType, 'taken')
  mockRoute.query = {}
  listEntitiesMock.mockReset().mockResolvedValue({
    data: [taak],
    meta: { total: 1, page: 1, per_page: 25, has_more: false },
    included: {},
  })
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountList() {
  const wrapper = mount(withPageHeader(EntityList), {
    props: { listId },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

function lastParams() {
  const calls = listEntitiesMock.mock.calls
  return calls[calls.length - 1][1] as Record<string, unknown>
}

function menuItem(label: string): HTMLElement {
  const item = Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find((el) =>
    el.textContent?.includes(label),
  )
  if (!item) throw new Error(`no menu item "${label}"`)
  return item
}

describe('EntityList toolbar', () => {
  it('offers the sortable columns, and not an unsortable one', async () => {
    seedSchema()
    const wrapper = await mountList()

    const options = wrapper.findComponent(SortMenu).props('options') as { property: string }[]
    expect(options.map((o) => o.property)).toEqual(['title', 'status'])
    wrapper.unmount()
  })

  it('sorts by the chosen property, then flips the direction', async () => {
    seedSchema()
    const wrapper = await mountList()
    expect(lastParams().sort).toBeUndefined()

    await wrapper.find('[data-testid="sort-menu-button"]').trigger('click')
    await flushPromises()
    menuItem('Status').click()
    await flushPromises()
    expect(lastParams().sort).toBe('status')

    await wrapper.find('[data-testid="sort-menu-button"]').trigger('click')
    await flushPromises()
    menuItem('Descending').click()
    await flushPromises()
    expect(lastParams().sort).toBe('-status')

    await wrapper.find('[data-testid="sort-menu-button"]').trigger('click')
    await flushPromises()
    menuItem('Default order').click()
    await flushPromises()
    // The unsorted read is already cached, so the proof is the menu's state.
    expect(wrapper.findComponent(SortMenu).props('modelValue')).toEqual([])
    wrapper.unmount()
  })

  it('shows the filter controls behind a Filters button, not inline', async () => {
    seedSchema({ filterControls: true })
    const wrapper = await mountList()

    expect(wrapper.findComponent(FilterBar).exists()).toBe(true)
    expect(wrapper.find('[data-testid="filter-menu-button"]').text()).toContain('Filters')
    // Closed: the controls are not in the document.
    expect(document.querySelector('[data-testid="filter-panel"]')).toBeNull()
    wrapper.unmount()
  })

  it('has no Filters button when the list declares no filter controls', async () => {
    seedSchema()
    const wrapper = await mountList()

    expect(wrapper.find('[data-testid="filter-menu-button"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
