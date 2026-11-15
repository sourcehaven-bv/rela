import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import { ApiError } from '@/api/errors'
import type { ConfigSnapshot } from '@/api/configure'
import { getIn, newMap } from '@/configure/tree'
import { endConfigSave, isConfigSaveInFlight } from '@/configure/saveGate'

// "Add column" (TKT-1X5G3BP): a new enum value saved through the Configure
// API. getConfigure and saveConfigure are mocked; the server's allowlist and
// config:edit check are its own tests.
const listAllEntitiesMock = vi.fn()
const getMeMock = vi.fn()
const getConfigureMock = vi.fn()
const saveConfigureMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  getMe: () => getMeMock(),
}))
vi.mock('@/api/configure', async (orig) => ({
  ...(await orig<typeof import('@/api/configure')>()),
  getConfigure: () => getConfigureMock(),
  saveConfigure: (...args: unknown[]) => saveConfigureMock(...args),
}))
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/kanban/board' }),
}))
vi.mock('@/composables/useBackTarget', () => ({ useBackTarget: () => null }))

const TYPE = 'risk'
const stagePath = ['entities', TYPE, 'properties', 'stage']

function snapshot(): ConfigSnapshot {
  const stage = newMap({ type: 'enum', values: ['open', 'decided'] })
  return {
    version: 'v7',
    schema: newMap({ entities: newMap({ [TYPE]: newMap({ properties: newMap({ stage }) }) }) }),
    data_entry: newMap({
      kanbans: newMap({ board: newMap({ entity_type: TYPE, column_property: 'stage' }) }),
    }),
    editable: {},
  }
}

function seed(config: Record<string, unknown>) {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set('board', {
    entity: TYPE,
    title: 'Board',
    column_property: 'stage',
    card: { title: 'title' },
    add_column: true,
    ...config,
  } as never)
  schemaStore.entityTypes.set(TYPE, {
    name: TYPE,
    label: 'Risk',
    primary: 'title',
    properties: { title: { type: 'string' }, stage: { type: 'enum', values: ['open', 'decided'] } },
  } as never)
  listAllEntitiesMock.mockResolvedValue({
    data: [],
    meta: { total: 0, page: 1, per_page: 25, has_more: false },
  })
}

const reload = vi.fn()
const realLocation = window.location
let pinia: ReturnType<typeof createPinia>

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest(TYPE, 'risks')
  vi.resetAllMocks()
  getMeMock.mockResolvedValue({ user: 'alice', can_configure: true })
  getConfigureMock.mockResolvedValue(snapshot())
  saveConfigureMock.mockResolvedValue({ version: 'v8', problems: null, saved: true })
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...realLocation, reload },
  })
})

afterEach(() => {
  Object.defineProperty(window, 'location', { configurable: true, value: realLocation })
  document.body.replaceChildren()
  endConfigSave()
})

async function mountBoard(config: Record<string, unknown> = {}) {
  seed(config)
  const wrapper = mount(KanbanView, {
    props: { id: 'board' },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

function control(wrapper: VueWrapper) {
  return wrapper.find('[data-testid="kanban-add-column"]')
}

async function addColumn(wrapper: VueWrapper, label: string) {
  await control(wrapper).find('button').trigger('click')
  await flushPromises()
  const input = control(wrapper).find('input')
  await input.setValue(label)
  await input.trigger('keydown', { key: 'Enter' })
  await flushPromises()
  return input
}

describe('KanbanView add column', () => {
  it('saves the new value through the Configure API and reloads', async () => {
    const wrapper = await mountBoard()
    await addColumn(wrapper, 'In review')

    expect(saveConfigureMock).toHaveBeenCalledTimes(1)
    const draft = saveConfigureMock.mock.calls[0][0]
    expect(draft.base_version).toBe('v7')
    expect(getIn(draft.schema, [...stagePath, 'values'])).toEqual(['open', 'decided', 'in_review'])
    expect(getIn(draft.schema, [...stagePath, 'labels', 'in_review'])).toBe('In review')
    expect(reload).toHaveBeenCalledTimes(1)
    // The tab reloads itself, so the config-changed broadcast must not.
    expect(isConfigSaveInFlight()).toBe(true)
  })

  it('refuses an existing column before any save', async () => {
    const wrapper = await mountBoard()
    const input = await addColumn(wrapper, 'Open')
    expect(saveConfigureMock).not.toHaveBeenCalled()
    expect(control(wrapper).text()).toContain('The board already has a column named open')
    expect((input.element as HTMLInputElement).value).toBe('Open')
  })

  it('shows the problems of a refused save and keeps the text', async () => {
    saveConfigureMock.mockResolvedValue({
      version: 'v7',
      problems: [{ code: 'locked', message: 'This key cannot be changed here.' }],
      saved: false,
    })
    const wrapper = await mountBoard()
    const input = await addColumn(wrapper, 'Parked')
    expect(control(wrapper).text()).toContain('This key cannot be changed here.')
    expect((input.element as HTMLInputElement).value).toBe('Parked')
    expect(reload).not.toHaveBeenCalled()
    expect(isConfigSaveInFlight()).toBe(false)
  })

  it('shows a version conflict inline', async () => {
    saveConfigureMock.mockRejectedValue(
      new ApiError('The configuration changed since it was loaded.', {
        kind: 'http',
        status: 409,
        original: null,
      })
    )
    const wrapper = await mountBoard()
    await addColumn(wrapper, 'Parked')
    expect(control(wrapper).text()).toContain('The configuration changed since it was loaded.')
    expect(reload).not.toHaveBeenCalled()
  })

  it.each([
    ['without add_column', { add_column: undefined }, { can_configure: true }],
    ['for someone who cannot configure', {}, { can_configure: false }],
    [
      'on a relation-backed board',
      { column_property: '', columns_from: { relation: 'in-topic' } },
      { can_configure: true },
    ],
  ])('is absent %s', async (_name, config, me) => {
    getMeMock.mockResolvedValue({ user: 'alice', ...me })
    const wrapper = await mountBoard(config)
    expect(control(wrapper).exists()).toBe(false)
  })

  it('does not ask /_me on a board without add_column', async () => {
    await mountBoard({ add_column: undefined })
    expect(getMeMock).not.toHaveBeenCalled()
  })
})
