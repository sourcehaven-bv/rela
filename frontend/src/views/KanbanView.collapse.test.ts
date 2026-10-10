import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import { kanbanCollapseStorageKey } from '@/composables/useKanbanCollapse'
import type { Entity } from '@/types'

const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
}))

vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/kanban/board' }),
}))

vi.mock('@/composables/useBackTarget', () => ({ useBackTarget: () => null }))

function ticket(id: string, status: string): Entity {
  return { id, type: 'ticket', properties: { title: id, status }, relations: {} }
}

let pinia: ReturnType<typeof createPinia>

beforeEach(() => {
  localStorage.clear()
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest('ticket', 'tickets')
  listAllEntitiesMock.mockReset().mockResolvedValue({
    data: [ticket('T-1', 'todo'), ticket('T-2', 'later'), ticket('T-3', 'later')],
    meta: { total: 3, page: 1, per_page: 25, has_more: false },
  })
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountBoard() {
  const schema = useSchemaStore()
  schema.kanbans.set('board', {
    entity: 'ticket',
    title: 'Board',
    column_property: 'status',
    columns: [
      { value: 'todo', label: 'Todo' },
      { value: 'later', label: 'Later', collapsed: true },
    ],
    card: { title: 'title', fields: [] },
  } as never)
  schema.entityTypes.set('ticket', {
    name: 'ticket',
    properties: {
      title: { type: 'string' },
      status: { type: 'enum', values: ['todo', 'later'] },
    },
  } as never)
  const wrapper = mount(KanbanView, {
    props: { id: 'board' },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

describe('KanbanView column collapse', () => {
  it('starts a config-collapsed column as a rail with its count', async () => {
    const wrapper = await mountBoard()

    const rail = wrapper.get('button[aria-label="Expand Later"]')
    expect(rail.text()).toContain('Later')
    expect(rail.text()).toContain('2')
    expect(wrapper.text()).not.toContain('T-2')
  })

  it('collapses a column from its heading and remembers it', async () => {
    const wrapper = await mountBoard()

    await wrapper.get('button[aria-label="Collapse Todo"]').trigger('click')

    expect(wrapper.find('button[aria-label="Expand Todo"]').exists()).toBe(true)
    expect(JSON.parse(localStorage.getItem(kanbanCollapseStorageKey('board'))!)).toEqual({
      todo: true,
    })
  })

  it('expands a config-collapsed column and keeps it open', async () => {
    const wrapper = await mountBoard()

    await wrapper.get('button[aria-label="Expand Later"]').trigger('click')

    expect(wrapper.text()).toContain('T-2')
    expect(JSON.parse(localStorage.getItem(kanbanCollapseStorageKey('board'))!)).toEqual({
      later: false,
    })
  })
})
