import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// Toggle columns (TKT-FJG779Q): a checkbox whose click writes the value the
// server put in `_toggles`.

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

const updateEntityMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
}))

vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/list/todos', name: 'list' }),
}))

describe('EntityList toggle column', () => {
  const listId = 'inbox'

  function seed(other: string | undefined, entities: Entity[]) {
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Inbox',
      entity: 'todo',
      columns: [
        { property: 'title' },
        { property: 'status', label: 'Done', widget: 'toggle', on: 'completed', off: 'open', other },
      ],
    } as never)
    schemaStore.entityTypes.set('todo', {
      name: 'todo',
      label: 'Todo',
      properties: { title: { type: 'string' }, status: { type: 'enum', values: ['open', 'completed', 'canceled'] } },
    } as never)
    const response: ListResponse<Entity> = {
      data: entities,
      meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
      included: {},
    }
    listEntitiesMock.mockResolvedValue(response)
  }

  function row(id: string, status: string | undefined, toggles?: Record<string, string | boolean>): Entity {
    return {
      id,
      type: 'todo',
      _title: `Todo ${id}`,
      _self: `/api/v1/todos/${id}`,
      properties: status === undefined ? { title: id } : { title: id, status },
      ...(toggles ? { _toggles: toggles } : {}),
    } as unknown as Entity
  }

  async function mountList() {
    const wrapper = mount(EntityList, { props: { listId }, global: { plugins: [pinia, PiniaColada] } })
    await flushPromises()
    return wrapper
  }

  function box(wrapper: Awaited<ReturnType<typeof mountList>>, title: string) {
    return wrapper.find(`input[data-testid="toggle-cell"][aria-label="Done: ${title}"]`)
  }

  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest('todo', 'todos')
    listEntitiesMock.mockReset()
    updateEntityMock.mockReset()
    updateEntityMock.mockResolvedValue({})
  })
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('PATCHes the value the server offered', async () => {
    seed('disabled', [row('T1', 'open', { status: 'completed' }), row('T2', 'completed', { status: 'open' })])
    const wrapper = await mountList()
    const open = box(wrapper, 'Todo T1')
    expect(open.exists()).toBe(true)
    expect((open.element as HTMLInputElement).checked).toBe(false)
    expect((box(wrapper, 'Todo T2').element as HTMLInputElement).checked).toBe(true)
    await open.trigger('click')
    await flushPromises()
    expect(updateEntityMock).toHaveBeenCalledWith('todo', 'T1', { properties: { status: 'completed' } })
    await box(wrapper, 'Todo T2').trigger('click')
    await flushPromises()
    expect(updateEntityMock).toHaveBeenLastCalledWith('todo', 'T2', { properties: { status: 'open' } })
  })

  it('disables the checkbox without a toggle entry', async () => {
    seed('disabled', [row('T1', 'open'), row('T2', 'canceled')])
    const wrapper = await mountList()
    expect((box(wrapper, 'Todo T1').element as HTMLInputElement).disabled).toBe(true)
    const other = box(wrapper, 'Todo T2').element as HTMLInputElement
    expect(other.disabled).toBe(true)
    expect(other.checked).toBe(false)
    await box(wrapper, 'Todo T1').trigger('click')
    expect(updateEntityMock).not.toHaveBeenCalled()
  })

  it('renders no checkbox for another value with other: hidden', async () => {
    seed('hidden', [row('T1', 'canceled')])
    const wrapper = await mountList()
    expect(box(wrapper, 'Todo T1').exists()).toBe(false)
  })

  it('renders another value checked with other: on', async () => {
    seed('on', [row('T1', 'canceled', { status: 'open' })])
    const wrapper = await mountList()
    const el = box(wrapper, 'Todo T1').element as HTMLInputElement
    expect(el.checked).toBe(true)
    expect(el.disabled).toBe(false)
  })

  it('reads an unset value as off', async () => {
    seed('disabled', [row('T1', undefined, { status: 'completed' })])
    const wrapper = await mountList()
    expect((box(wrapper, 'Todo T1').element as HTMLInputElement).checked).toBe(false)
  })
})
