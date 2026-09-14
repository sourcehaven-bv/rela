import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// A board's filtering is SERVER-side (TKT-LPLZ1V, BUG-MYN56J).
//
// It used to happen here, in a switch over three operators whose default arm
// returned true — so `~`, `in`, `<`, `<=`, `>` and `>=` all passed validation
// and then did nothing, list values were flattened to "a,b" before comparing,
// and `$today` was never substituted. Those are not fixed by making the
// client-side matcher better: a second matcher IS the defect (four already
// existed, and evaluator drift is BUG-F1LTV0's documented root cause).
//
// So these tests assert the board SENDS its filters and does not re-apply
// them. Both halves matter: sending without removing the JS pass would filter
// twice, and removing without sending would show everything.

const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
}))
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/kanban/board', name: 'kanban' }),
}))
vi.mock('@/composables/useBackTarget', () => ({
  useBackTarget: () => ({ target: null }),
}))

const KANBAN_ID = 'board'
const ENTITY_TYPE = 'ticket'

describe('KanbanView server-side filtering', () => {
  function seedSchema(configOverrides: Record<string, unknown> = {}) {
    const schemaStore = useSchemaStore()
    schemaStore.kanbans.set(KANBAN_ID, {
      entity: ENTITY_TYPE,
      title: 'Board',
      column_property: 'status',
      columns: [{ value: 'todo', label: 'Todo' }],
      card: { title: 'title', fields: [] },
      ...configOverrides,
    } as never)
    schemaStore.entityTypes.set(ENTITY_TYPE, {
      name: ENTITY_TYPE,
      label: 'Ticket',
      properties: {
        title: { type: 'string', values: null },
        status: { type: 'enum', values: ['todo'] },
        due: { type: 'date', values: null },
      },
    } as never)
  }

  function seedBoard(entities: Entity[]) {
    const response: ListResponse<Entity> = {
      data: entities,
      meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
      included: {},
    }
    listAllEntitiesMock.mockResolvedValue(response)
  }

  const ticket = (id: string): Entity => ({
    id,
    type: ENTITY_TYPE,
    properties: { title: `Ticket ${id}`, status: 'todo' },
    relations: {},
  })

  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(ENTITY_TYPE, 'tickets')
    listAllEntitiesMock.mockReset()
  })

  const mounted: VueWrapper[] = []
  afterEach(() => {
    for (const w of mounted.splice(0)) w.unmount()
    document.body.innerHTML = ''
  })

  async function mountBoard(configOverrides: Record<string, unknown> = {}, entities = [ticket('T-1')]) {
    seedSchema(configOverrides)
    seedBoard(entities)
    const wrapper = mount(KanbanView, {
      props: { id: KANBAN_ID },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    mounted.push(wrapper)
    await flushPromises()
    return wrapper
  }

  function lastParams(): Record<string, unknown> {
    expect(listAllEntitiesMock).toHaveBeenCalled()
    return (listAllEntitiesMock.mock.calls.at(-1)?.[1] ?? {}) as Record<string, unknown>
  }

  it('names itself so the server applies the board’s condition', async () => {
    const wrapper = await mountBoard()
    expect(wrapper.text()).toContain('Ticket T-1') // the board really rendered
    expect(lastParams().kanban_id).toBe(KANBAN_ID)
  })

  it('sends static filters as params instead of applying them here', async () => {
    const wrapper = await mountBoard({
      filters: [{ property: 'status', operator: '!=', value: 'closed' }],
    })
    expect(wrapper.text()).toContain('Ticket T-1')
    expect(lastParams()['filter[status][ne]']).toBe('closed')
  })

  // The operators the old switch silently ignored. Each must reach the server,
  // which is the whole of BUG-MYN56J's fix: the board no longer has an opinion
  // about which operators it can evaluate.
  it.each([
    ['<=', 'lte'],
    ['>=', 'gte'],
    ['<', 'lt'],
    ['>', 'gt'],
    ['~', 'contains'],
    ['in', 'in'],
  ])('sends the %s operator (previously a silent no-op) as [%s]', async (uiOp, wireOp) => {
    await mountBoard({ filters: [{ property: 'due', operator: uiOp, value: '2026-01-01' }] })
    expect(lastParams()[`filter[due][${wireOp}]`]).toBe('2026-01-01')
  })

  // A `$today` value must travel verbatim: substitution is the server's job
  // (resolveFilterVariable). The old client-side pass compared against the
  // literal string "$today" and matched nothing.
  it('passes a $today filter value through untouched', async () => {
    await mountBoard({ filters: [{ property: 'due', operator: '<=', value: '$today' }] })
    expect(lastParams()['filter[due][lte]']).toBe('$today')
  })

  // The other half: rows arrive filtered, so the board must NOT filter again.
  // A server that returns a row the old JS pass would have dropped proves the
  // client-side matcher is gone rather than merely bypassed.
  it('renders what the server returns, without re-filtering', async () => {
    const wrapper = await mountBoard(
      { filters: [{ property: 'status', operator: '=', value: 'todo' }] },
      [ticket('T-1'), { ...ticket('T-2'), properties: { title: 'Ticket T-2', status: 'todo' } }]
    )
    expect(wrapper.text()).toContain('Ticket T-1')
    expect(wrapper.text()).toContain('Ticket T-2')
  })
})
