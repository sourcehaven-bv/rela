import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// A list's `query_scope:` decides which rows are MEMBERS of the list.
//
// The list endpoint is keyed by entity TYPE, so the server cannot tell which
// configured list is on screen — the client has to say, exactly as it does for
// `?world=`. When it does not, the request falls back to the entity type's
// `default` scope, so a list declaring `query_scope: archief` renders the
// non-archived rows instead. Nothing errors: the config validates, an index is
// derived for the scope, and the wrong rows appear. That is the fail-open the
// feature exists to prevent, and it is only visible from this side.
//
// The absence assertion below follows the anti-vacuity discipline of
// EntityList.world.test.ts: "no query_scope is sent" passes trivially against a
// component that threw during setup, so it is paired with rendersProof.

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

const mockRoute = {
  query: {} as Record<string, unknown>,
  path: '/list/taken',
  name: 'list',
}
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => mockRoute,
  RouterLink: {
    props: ['to'],
    template: '<a><slot /></a>',
  },
}))

describe('EntityList query scope', () => {
  const listId = 'taken'
  const entityType = 'taak'

  function seedSchema(queryScope?: string) {
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Taken',
      entity: entityType,
      columns: [{ property: 'title', label: 'Titel' }],
      ...(queryScope ? { query_scope: queryScope } : {}),
    } as never)
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'Taak',
      properties: { title: { type: 'string', values: null } },
    } as never)
  }

  const taak: Entity = {
    id: 'TAAK-1',
    type: entityType,
    properties: { title: 'Open werk' },
  }

  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(entityType, 'taken')
    listEntitiesMock.mockReset()
    mockRoute.query = {}
  })

  const mounted: VueWrapper[] = []
  afterEach(() => {
    for (const w of mounted.splice(0)) w.unmount()
  })

  async function mountList(queryScope?: string) {
    seedSchema(queryScope)
    listEntitiesMock.mockResolvedValue({
      data: [taak],
      meta: { total: 1, page: 1, per_page: 25, has_more: false },
      included: {},
    } as ListResponse<Entity>)
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

  // The anti-vacuity guard: an absence assertion against a list that never
  // rendered proves nothing.
  function rendersProof(wrapper: { text: () => string }) {
    expect(listEntitiesMock).toHaveBeenCalled()
    expect(wrapper.text()).toContain('Open werk')
  }

  it('sends the list’s configured query_scope', async () => {
    const wrapper = await mountList('archief')
    rendersProof(wrapper)
    expect(lastParams().query_scope).toBe('archief')
  })

  it('sends NO query_scope param when the list declares none', async () => {
    // Omitted rather than empty. The server refuses a name that does not
    // resolve, and "" is such a name; absent is what makes the entity type's
    // `default` scope apply.
    const wrapper = await mountList()
    rendersProof(wrapper)
    expect(lastParams().query_scope).toBeUndefined()
  })

  it('sends `all` verbatim so a list can withdraw the type’s default', async () => {
    const wrapper = await mountList('all')
    rendersProof(wrapper)
    expect(lastParams().query_scope).toBe('all')
  })
})
