import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
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
// The tab's link to the page entity has its own tests; here only the call.
const linkCreatedMock = vi.fn()
vi.mock('@/composables/usePageTabScope', async (orig) => ({
  ...(await orig<typeof import('@/composables/usePageTabScope')>()),
  usePageTabScope: () => ({
    params: { value: {} },
    linkCreated: (...args: unknown[]) => linkCreatedMock(...args),
    relationOrdered: { value: false },
  }),
}))

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

// The board's rows. A created card joins them, so the refetch after a
// quick-add returns it, as the server would.
let rows: Entity[] = []

function seedBoard(actions?: Record<string, boolean>) {
  rows = []
  listAllEntitiesMock.mockImplementation(async (type: string): Promise<ListResponse<Entity>> =>
    type === 'topic'
      ? {
          data: [{ id: 'TOP-1', type: 'topic', properties: { name: 'Privacy' }, relations: {} }],
          meta: { total: 1, page: 1, per_page: 25, has_more: false },
        }
      : {
          data: [...rows],
          meta: { total: rows.length, page: 1, per_page: 25, has_more: false },
          ...(actions ? { _actions: actions } : {}),
        }
  )
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest(TYPE, 'risks')
  _setEntityPluralForTest('topic', 'topics')
  listAllEntitiesMock.mockReset()
  linkCreatedMock.mockReset().mockResolvedValue(undefined)
  createEntityMock
    .mockReset()
    .mockImplementation(async (_type: string, body: { properties: Record<string, unknown> }) => {
      const created: Entity = {
        id: `RSK-${rows.length + 1}`,
        type: TYPE,
        properties: body.properties,
        relations: {},
      } as Entity
      rows.push(created)
      return created
    })
})

afterEach(() => {
  document.body.replaceChildren()
})

async function mountBoard(config: Record<string, unknown> = {}, actions?: Record<string, boolean>) {
  seedSchema(config)
  seedBoard(actions)
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

  it('leaves the property unset in a column with an empty value', async () => {
    const wrapper = await mountBoard({
      columns: [
        { value: '', label: 'Unsorted' },
        { value: 'open', label: 'Open' },
      ],
    })
    await type(wrapper, 0, 'Loose')
    expect(createEntityMock).toHaveBeenCalledWith(TYPE, { properties: { title: 'Loose' } })
  })

  it('sends the world of a world-bound board', async () => {
    const wrapper = await mountBoard()
    useSchemaStore().defaultWorld = 'draft'
    await flushPromises()
    await type(wrapper, 0, 'Draft risk')
    expect(createEntityMock).toHaveBeenCalledWith(TYPE, {
      properties: { title: 'Draft risk', stage: 'open' },
      world: 'draft',
    })
  })

  it('links the card to the page and shows the card on the board', async () => {
    const wrapper = await mountBoard()
    const info = vi.spyOn(useUIStore(), 'info')
    await type(wrapper, 0, 'Data leak')
    expect(linkCreatedMock).toHaveBeenCalledWith(expect.objectContaining({ id: rows[0].id }))
    expect(wrapper.findAll('.kanban-card').map((c) => c.text())).toEqual([
      expect.stringContaining('Data leak'),
    ])
    expect(info).not.toHaveBeenCalled()
  })

  it('clears the text when the card was made but not linked', async () => {
    linkCreatedMock.mockRejectedValue(new Error('link refused'))
    const wrapper = await mountBoard()
    const error = vi.spyOn(useUIStore(), 'error')
    const input = await type(wrapper, 0, 'Data leak')
    expect(error).toHaveBeenCalledWith('link refused')
    expect((input.element as HTMLInputElement).value).toBe('')
  })

  it('shows the warnings of an accepted create', async () => {
    createEntityMock.mockResolvedValue({
      id: 'RSK-1',
      type: TYPE,
      properties: { title: 'Data leak' },
      relations: {},
      warnings: [{ code: 'required_property_unset', detail: 'owner is unset' }],
    })
    const wrapper = await mountBoard()
    const warning = vi.spyOn(useUIStore(), 'warning')
    await type(wrapper, 0, 'Data leak')
    expect(warning).toHaveBeenCalledWith('Card added with warnings: owner is unset')
  })

  it('says so when the board filters hide the new card', async () => {
    const wrapper = await mountBoard({
      filters: [{ property: 'level', operator: '==', value: 'high' }],
    })
    const info = vi.spyOn(useUIStore(), 'info')
    await type(wrapper, 0, 'Data leak')
    expect(info).toHaveBeenCalledWith('Added "Data leak". This board\'s filters hide it.')
  })

  it('sends one create for a second Enter while the first is out', async () => {
    let finish: (e: Entity) => void = () => {}
    createEntityMock.mockImplementation(() => new Promise<Entity>((resolve) => (finish = resolve)))
    const wrapper = await mountBoard()
    const input = inputs(wrapper)[0]
    await input.setValue('Data leak')
    await input.trigger('keydown', { key: 'Enter' })
    await input.trigger('keydown', { key: 'Enter' })
    finish({ id: 'RSK-1', type: TYPE, properties: { title: 'Data leak' }, relations: {} } as Entity)
    await flushPromises()
    expect(createEntityMock).toHaveBeenCalledTimes(1)
  })
})
