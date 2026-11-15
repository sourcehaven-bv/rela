import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import { ApiError } from '@/api/errors'
import type { Entity, ListResponse } from '@/types'

// Quick-add (TKT-IP4273S): a title input at the foot of every column and
// swimlane cell. The create goes through the entities store to createEntity,
// mocked here; the server's side of the contract is the ordinary create API.
const listAllEntitiesMock = vi.fn()
const createEntityMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
}))
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  createEntity: (...args: unknown[]) => createEntityMock(...args),
}))
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/kanban/board' }),
}))
vi.mock('@/composables/useBackTarget', () => ({ useBackTarget: () => null }))

const TYPE = 'risk'

function seedSchema(config: Record<string, unknown>) {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set('board', {
    entity: TYPE,
    title: 'Board',
    column_property: 'stage',
    columns: [
      { value: 'open', label: 'Open' },
      { value: 'decided', label: 'Decided' },
    ],
    card: { title: 'title' },
    quick_add: true,
    ...config,
  } as never)
  schemaStore.entityTypes.set(TYPE, {
    name: TYPE,
    label: 'Risk',
    primary: 'title',
    properties: {
      title: { type: 'string' },
      stage: { type: 'enum', values: ['open', 'decided'] },
      level: { type: 'enum', values: ['low', 'high'] },
    },
  } as never)
  schemaStore.entityTypes.set('topic', {
    name: 'topic',
    label: 'Topic',
    primary: 'name',
    properties: {},
  } as never)
  schemaStore.relationTypes.set('in-topic', {
    id: 'in-topic',
    from: [TYPE],
    to: ['topic'],
  } as never)
}

function seedBoard(entities: Entity[], actions?: Record<string, boolean>) {
  const response: ListResponse<Entity> = {
    data: entities,
    meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
    ...(actions ? { _actions: actions } : {}),
  }
  listAllEntitiesMock.mockImplementation(async (type: string) =>
    type === 'topic'
      ? {
          data: [{ id: 'TOP-1', type: 'topic', properties: { name: 'Privacy' }, relations: {} }],
          meta: { total: 1, page: 1, per_page: 25, has_more: false },
        }
      : response
  )
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest(TYPE, 'risks')
  _setEntityPluralForTest('topic', 'topics')
  listAllEntitiesMock.mockReset()
  createEntityMock
    .mockReset()
    .mockImplementation(async (_type: string, body: { properties: Record<string, unknown> }) => ({
      id: 'RSK-9',
      type: TYPE,
      properties: body.properties,
      relations: {},
    }))
})

afterEach(() => {
  document.body.replaceChildren()
})

async function mountBoard(config: Record<string, unknown> = {}, actions?: Record<string, boolean>) {
  seedSchema(config)
  seedBoard([], actions)
  const wrapper = mount(KanbanView, {
    props: { id: 'board' },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

function inputs(wrapper: VueWrapper) {
  return wrapper.findAll('[data-testid="kanban-quick-add"] input')
}

async function type(wrapper: VueWrapper, index: number, text: string) {
  const input = inputs(wrapper)[index]
  await input.setValue(text)
  await input.trigger('keydown', { key: 'Enter' })
  await flushPromises()
  return input
}

describe('KanbanView quick-add', () => {
  it('renders one input per column, named after it', async () => {
    const wrapper = await mountBoard()
    const found = inputs(wrapper)
    expect(found).toHaveLength(2)
    const labels = found.map((i) =>
      document.querySelector(`label[for="${i.attributes('id')}"]`)?.textContent?.trim()
    )
    expect(labels).toEqual(['Add a card to Open', 'Add a card to Decided'])
  })

  it('creates a card with the title and the column value, then clears the input', async () => {
    const wrapper = await mountBoard()
    const input = await type(wrapper, 1, '  Data leak  ')
    expect(createEntityMock).toHaveBeenCalledWith(TYPE, {
      properties: { title: 'Data leak', stage: 'decided' },
    })
    expect((input.element as HTMLInputElement).value).toBe('')
  })

  it('sends nothing for a blank title', async () => {
    const wrapper = await mountBoard()
    await type(wrapper, 0, '   ')
    expect(createEntityMock).not.toHaveBeenCalled()
  })

  it('ignores the Enter that commits an IME composition', async () => {
    const wrapper = await mountBoard()
    const input = inputs(wrapper)[0]
    await input.setValue('漢字')
    await input.trigger('keydown', { key: 'Enter', isComposing: true })
    await flushPromises()
    expect(createEntityMock).not.toHaveBeenCalled()
  })

  it('sets the swimlane value from the cell', async () => {
    const wrapper = await mountBoard({
      swimlane_property: 'level',
      swimlanes: [
        { value: 'low', label: 'Low' },
        { value: 'high', label: 'High' },
      ],
    })
    // Lane by lane: Low (Open, Decided), then High (Open, Decided).
    expect(inputs(wrapper)).toHaveLength(4)
    await type(wrapper, 2, 'Fines')
    expect(createEntityMock).toHaveBeenCalledWith(TYPE, {
      properties: { title: 'Fines', stage: 'open', level: 'high' },
    })
  })

  it('links a relation-backed card to its column target, and an Other card to none', async () => {
    const wrapper = await mountBoard({
      column_property: '',
      columns: undefined,
      columns_from: { relation: 'in-topic' },
      layout: 'wall',
    })
    // The Privacy group, then the pile.
    expect(inputs(wrapper)).toHaveLength(2)
    await type(wrapper, 0, 'Consent')
    expect(createEntityMock).toHaveBeenLastCalledWith(TYPE, {
      properties: { title: 'Consent' },
      relations: { 'in-topic': { data: [{ type: 'topic', id: 'TOP-1' }] } },
    })
    await type(wrapper, 1, 'Loose')
    expect(createEntityMock).toHaveBeenLastCalledWith(TYPE, { properties: { title: 'Loose' } })
  })

  it('shows a refused create inline and keeps the text', async () => {
    createEntityMock.mockRejectedValue(
      new ApiError('Title is too short', { kind: 'http', status: 422, original: null })
    )
    const wrapper = await mountBoard()
    const input = await type(wrapper, 0, 'X')
    expect((input.element as HTMLInputElement).value).toBe('X')
    expect(input.attributes('aria-invalid')).toBe('true')
    expect(wrapper.find('[data-testid="kanban-quick-add"]').text()).toContain('Title is too short')
  })

  it('is absent without quick_add', async () => {
    const wrapper = await mountBoard({ quick_add: undefined })
    expect(inputs(wrapper)).toHaveLength(0)
  })

  it('is absent when the reader may not create the type', async () => {
    const wrapper = await mountBoard({}, { create: false })
    expect(inputs(wrapper)).toHaveLength(0)
  })
})
