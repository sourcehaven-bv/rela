import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// Count card fields (TKT-WA25G2): `display: count` on a relation and
// `comments: true` render as an icon and a number, not as titles.

const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/kanban/board' }),
}))

vi.mock('@/composables/useBackTarget', () => ({
  useBackTarget: () => null,
}))

const KANBAN_ID = 'board'
const ENTITY_TYPE = 'ticket'

describe('KanbanView count card fields', () => {
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

  async function mountBoard(fields: Array<Record<string, unknown>>, entity: Partial<Entity>) {
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
      properties: { title: { type: 'string' }, status: { type: 'enum', values: ['todo'] } },
    } as never)
    const response: ListResponse<Entity> = {
      data: [
        {
          id: 'T-1',
          type: ENTITY_TYPE,
          properties: { title: 'Ticket T-1', status: 'todo' },
          relations: {},
          ...entity,
        },
      ],
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

  function requestParams(): Record<string, unknown> {
    return (listAllEntitiesMock.mock.calls[0][1] ?? {}) as Record<string, unknown>
  }

  it('counts the related ids instead of listing their titles', async () => {
    const wrapper = await mountBoard(
      [{ relation: 'subtask-of', direction: 'incoming', display: 'count', label: 'subtasks' }],
      { relations: { 'subtask-of_inverse': ['T-2', 'T-3'] } }
    )
    const counts = wrapper.find('.card-counts')
    expect(counts.text()).toContain('2')
    expect(counts.text()).toContain('subtasks')
    expect(wrapper.find('.kanban-card').text()).not.toContain('T-2')
    // A count needs no titles, so the board does not ask for includes.
    expect(requestParams().include).toBeUndefined()
  })

  it('shows the served comment count and asks for it', async () => {
    const wrapper = await mountBoard([{ comments: true }], { _comment_count: 3 })
    expect(wrapper.find('.card-counts').text()).toContain('3')
    expect(wrapper.find('.card-counts').text()).toContain('comments')
    expect(requestParams().comment_counts).toBe(true)
  })

  it('shows the label beside the number unless show_label is false', async () => {
    const shown = await mountBoard([{ comments: true }], { _comment_count: 3 })
    expect(shown.find('.card-counts .rl-meta-item__label').text()).toBe('3 comments')

    const hidden = await mountBoard([{ comments: true, show_label: false }], { _comment_count: 3 })
    expect(hidden.find('.card-counts .rl-meta-item__label').text()).toBe('3')
    // Still spoken to a screen reader, and on hover.
    expect(hidden.find('.card-counts .rl-visually-hidden').text()).toContain('comments')
    expect(hidden.find('.card-counts .rl-meta-item').attributes('title')).toBe('comments')
  })

  it('counts each related id once', async () => {
    const wrapper = await mountBoard(
      [{ relation: 'subtask-of', display: 'count', show_label: false }],
      {
        relations: { 'subtask-of': ['T-2', 'T-2'] },
      }
    )
    expect(wrapper.find('.card-counts .rl-meta-item__label').text()).toBe('1')
  })

  it('does not ask for comment counts when no card shows them', async () => {
    await mountBoard([{ relation: 'subtask-of', display: 'count' }], {})
    expect(requestParams().comment_counts).toBeUndefined()
  })

  it.each([
    ['zero comments', [{ comments: true }], { _comment_count: 0 }],
    ['a comment count the server did not serve', [{ comments: true }], {}],
    ['no related ids', [{ relation: 'subtask-of', display: 'count' }], { relations: {} }],
  ])('renders nothing for %s', async (_name, fields, entity) => {
    const wrapper = await mountBoard(fields, entity as Partial<Entity>)
    expect(wrapper.find('.card-counts').exists()).toBe(false)
  })
})
