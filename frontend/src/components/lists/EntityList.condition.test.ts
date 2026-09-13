import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// A list's `condition:` is evaluated SERVER-side; the SPA's only job is to
// name which list it is rendering, so the server can look the expression up in
// its own config (TKT-LPLZ1V).
//
// The expression must never cross the wire: sending it would let a caller
// forge a predicate the operator never declared. These tests pin that the id
// goes and nothing else does.
//
// Both assertions here are parameter-shaped, the class that passes trivially
// when a component fails to mount — so each also asserts the row rendered,
// proving the list actually ran.

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: async () => true }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/list/tasks', name: 'list' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

describe('EntityList condition wiring', () => {
  const listId = 'open_tasks'
  const entityType = 'task'

  const task: Entity = { id: 'T-1', type: entityType, properties: { title: 'Write docs' } }

  function seed() {
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Open tasks',
      entity: entityType,
      columns: [{ property: 'title', label: 'Title' }],
    } as never)
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'Task',
      properties: { title: { type: 'string', values: null } },
    } as never)

    const response: ListResponse<Entity> = {
      data: [task],
      meta: { total: 1, page: 1, per_page: 25, has_more: false },
      included: {},
    }
    listEntitiesMock.mockResolvedValue(response)
  }

  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(entityType, 'tasks')
    listEntitiesMock.mockReset()
  })

  const mounted: VueWrapper[] = []
  afterEach(() => {
    for (const w of mounted.splice(0)) w.unmount()
    document.body.innerHTML = ''
  })

  async function mountList() {
    seed()
    const wrapper = mount(EntityList, {
      props: { listId },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    mounted.push(wrapper)
    await flushPromises()
    return wrapper
  }

  function lastParams(): Record<string, unknown> {
    expect(listEntitiesMock).toHaveBeenCalled()
    const call = listEntitiesMock.mock.calls[listEntitiesMock.mock.calls.length - 1]
    return call[1] as Record<string, unknown>
  }

  it('sends list_id so the server can apply that list’s condition', async () => {
    const wrapper = await mountList()
    expect(wrapper.text()).toContain('Write docs') // the list really rendered
    expect(lastParams().list_id).toBe(listId)
  })

  it('never sends the condition expression itself', async () => {
    const wrapper = await mountList()
    expect(wrapper.text()).toContain('Write docs')

    // The server owns the expression; a request carrying one would be a
    // caller-supplied predicate, which is precisely what naming an id avoids.
    const keys = Object.keys(lastParams())
    expect(keys).not.toContain('condition')
    expect(keys.some((k) => k.startsWith('condition'))).toBe(false)
  })
})
