import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// Card fields moved onto the widget registry in mode:'display' (TKT-S9C14S).
//
// Before that migration getCardFieldValue was a bare `String(v || '')` that
// never called the shared cell formatter, so cards showed raw ISO datetimes
// and "true"/"false" -- and `|| ''` collapsed `false` and `0` to empty, which
// made visibleCardFields DROP the field from the card entirely. These tests
// pin the fixed behaviour; several of them fail against the old code.

const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/kanban/board' }),
}))

vi.mock('@/composables/useBackTarget', () => ({
  useBackTarget: () => null,
}))

const KANBAN_ID = 'board'
const ENTITY_TYPE = 'ticket'

describe('KanbanView card field rendering via widgets', () => {
  function seedSchema(
    properties: Record<string, unknown>,
    fields: Array<Record<string, unknown>>
  ) {
    const schemaStore = useSchemaStore()
    schemaStore.kanbans.set(KANBAN_ID, {
      entity: ENTITY_TYPE,
      title: 'Board',
      column_property: 'status',
      columns: [{ value: 'todo', label: 'Todo' }],
      card: { title: 'title', fields },
    } as never)
    schemaStore.entityTypes.set(ENTITY_TYPE, {
      name: ENTITY_TYPE,
      label: 'Ticket',
      properties: {
        title: { type: 'string' },
        status: { type: 'enum', values: ['todo'] },
        ...properties,
      },
    } as never)
  }

  async function mountBoard(
    properties: Record<string, unknown>,
    fields: Array<Record<string, unknown>>,
    entityProps: Record<string, unknown>
  ) {
    seedSchema(properties, fields)
    const entity: Entity = {
      id: 'T-1',
      type: ENTITY_TYPE,
      properties: { title: 'Ticket T-1', status: 'todo', ...entityProps },
      relations: {},
    }
    const response: ListResponse<Entity> = {
      data: [entity],
      meta: { total: 1, page: 1, per_page: 25, has_more: false },
      included: {},
    }
    listAllEntitiesMock.mockResolvedValue(response)
    const wrapper = mount(KanbanView, {
      props: { id: KANBAN_ID },
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
    _setEntityPluralForTest(ENTITY_TYPE, 'tickets')
    listAllEntitiesMock.mockReset()
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  // --- The bug fix: false / 0 are set values, not absent ones ---

  it('renders a false boolean as No instead of dropping the field', async () => {
    const wrapper = await mountBoard(
      { done: { type: 'boolean' } },
      [{ property: 'done', label: 'Done' }],
      { done: false }
    )
    const card = wrapper.find('.kanban-card')
    expect(card.text()).toContain('Done')
    expect(card.text()).toContain('No')
  })

  it('renders a zero integer instead of dropping the field', async () => {
    const wrapper = await mountBoard(
      { count: { type: 'integer' } },
      [{ property: 'count', label: 'Count' }],
      { count: 0 }
    )
    expect(wrapper.find('.kanban-card').text()).toContain('Count')
    expect(wrapper.find('.kanban-card').text()).toMatch(/\b0\b/)
  })

  it('renders a true boolean as Yes, not "true"', async () => {
    const wrapper = await mountBoard(
      { done: { type: 'boolean' } },
      [{ property: 'done', label: 'Done' }],
      { done: true }
    )
    const text = wrapper.find('.kanban-card').text()
    expect(text).toContain('Yes')
    expect(text).not.toContain('true')
  })

  // --- The bug fix: values are formatted, not stringified raw ---

  it('renders a datetime formatted rather than as a raw ISO string', async () => {
    const wrapper = await mountBoard(
      { due: { type: 'datetime' } },
      [{ property: 'due', label: 'Due' }],
      { due: '2026-03-04T10:30:00Z' }
    )
    const text = wrapper.find('.kanban-card').text()
    expect(text).not.toContain('2026-03-04T10:30:00Z')
    expect(text).toMatch(/2026/)
  })

  it('renders a date formatted rather than as the raw stored string', async () => {
    const wrapper = await mountBoard(
      { due: { type: 'date' } },
      [{ property: 'due', label: 'Due' }],
      { due: '2026-03-04' }
    )
    expect(wrapper.find('.kanban-card').text()).toMatch(/2026/)
  })

  // --- Preserved behaviour ---

  it('still drops a genuinely unset field from the card', async () => {
    const wrapper = await mountBoard(
      { effort: { type: 'enum', values: ['s', 'm'] } },
      [{ property: 'effort', label: 'Effort' }],
      {}
    )
    expect(wrapper.find('.kanban-card').text()).not.toContain('Effort')
  })

  it('drops a field whose value is an empty string', async () => {
    const wrapper = await mountBoard(
      { note: { type: 'string' } },
      [{ property: 'note', label: 'Note' }],
      { note: '' }
    )
    expect(wrapper.find('.kanban-card').text()).not.toContain('Note')
  })

  it('renders a scalar enum as a Badge', async () => {
    const wrapper = await mountBoard(
      { effort: { type: 'enum', values: ['s', 'm'] } },
      [{ property: 'effort', label: 'Effort' }],
      { effort: 'm' }
    )
    expect(wrapper.find('.kanban-card .badge').exists()).toBe(true)
  })

  it('renders a list-valued enum as one Badge per value, not one comma-joined Badge', async () => {
    // Pre-migration this rendered a single Badge whose value was "a, b".
    const wrapper = await mountBoard(
      { tags: { type: 'enum', values: ['a', 'b'], list: true } },
      [{ property: 'tags', label: 'Tags' }],
      { tags: ['a', 'b'] }
    )
    expect(wrapper.findAll('.kanban-card .badge').length).toBe(2)
  })
})
