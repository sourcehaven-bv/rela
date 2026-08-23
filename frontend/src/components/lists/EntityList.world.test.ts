import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// World-bound list surface (TKT-F2D5U5).
//
// A list is one of only TWO paths the API can serve under a non-default world
// (worldCapablePath, internal/dataentry/world.go). These tests pin what the
// list sends and what it offers under a world.
//
// Ruling 10 note — every assertion here is either a NEGATIVE ("no q is sent",
// "no search box") or a permission/parameter-shaped property, i.e. exactly the
// class that passes trivially when the component failed to render. So each
// test that asserts an absence ALSO asserts a matching presence (the rows
// rendered, the request fired), and `rendersProof` below is the shared guard:
// if the list did not actually mount and fetch, the absence proves nothing.

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

const routerPush = vi.fn()
const routerReplace = vi.fn()
const mockRoute = {
  query: {} as Record<string, unknown>,
  path: '/list/policies',
  name: 'list',
}
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush, replace: routerReplace }),
  useRoute: () => mockRoute,
  RouterLink: { template: '<a><slot /></a>' },
}))

describe('EntityList world binding', () => {
  const listId = 'policies'
  const entityType = 'policy'

  function seedSchema(opts: { relationColumn?: boolean } = {}) {
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Policies',
      entity: entityType,
      columns: opts.relationColumn
        ? [
            { property: 'title', label: 'Title' },
            { relation: 'owned-by', label: 'Owner' },
          ]
        : [{ property: 'title', label: 'Title' }],
    } as never)
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'Policy',
      properties: { title: { type: 'string', values: null } },
    } as never)
  }

  function seedEntities(entities: Entity[]) {
    const response: ListResponse<Entity> = {
      data: entities,
      meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
      included: {},
    }
    listEntitiesMock.mockResolvedValue(response)
  }

  const policy: Entity = {
    id: 'POL-1',
    type: entityType,
    properties: { title: 'Access Control Policy' },
  }

  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(entityType, 'policys')
    listEntitiesMock.mockReset()
    routerPush.mockClear()
    routerReplace.mockClear()
    mockRoute.query = {}
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  async function mountList(opts: { relationColumn?: boolean } = {}) {
    seedSchema(opts)
    seedEntities([policy])
    const wrapper = mount(EntityList, {
      props: { listId },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    return wrapper
  }

  function lastParams(): Record<string, unknown> {
    expect(listEntitiesMock).toHaveBeenCalled()
    const call = listEntitiesMock.mock.calls[listEntitiesMock.mock.calls.length - 1]
    return call[1] as Record<string, unknown>
  }

  // The anti-vacuity guard. Asserting "no search box" against a component that
  // threw during setup would pass while proving nothing, so every absence
  // assertion is paired with this.
  function rendersProof(wrapper: { text: () => string }) {
    expect(listEntitiesMock).toHaveBeenCalled()
    expect(wrapper.text()).toContain('Access Control Policy')
  }

  describe('default world', () => {
    it('sends no world param and offers search', async () => {
      const wrapper = await mountList()
      rendersProof(wrapper)

      expect(lastParams().world).toBeUndefined()
      expect(wrapper.find('.world-banner').exists()).toBe(false)
      // The positive control for every "search is hidden" assertion below:
      // without this, hiding search unconditionally would pass them all.
      expect(wrapper.findComponent({ name: 'SearchBox' }).exists()).toBe(true)
    })

    it('sends include=* for relation columns', async () => {
      const wrapper = await mountList({ relationColumn: true })
      rendersProof(wrapper)
      expect(lastParams().include).toBe('*')
    })
  })

  describe('non-default world', () => {
    beforeEach(() => {
      mockRoute.query = { world: 'published' }
    })

    it('sends the world param', async () => {
      const wrapper = await mountList()
      rendersProof(wrapper)
      expect(lastParams().world).toBe('published')
    })

    // Mutation: drop `&& !isWorldBound.value` from the q arm — the assertion
    // sees q sent alongside world, the combination the API answers with 422
    // world_search_unsupported, and which would otherwise surface DRAFT hits
    // on a published surface (the index holds default-face documents).
    it('never sends q, even when the URL carries one', async () => {
      mockRoute.query = { world: 'published', q: 'access' }
      const wrapper = await mountList()
      rendersProof(wrapper)

      expect(lastParams().world).toBe('published')
      expect(lastParams().q).toBeUndefined()
    })

    // The UI half of the same property. Paired with the default-world control
    // above, which proves the box renders when it should.
    it('omits the search box and explains why', async () => {
      const wrapper = await mountList()
      rendersProof(wrapper)

      expect(wrapper.findComponent({ name: 'SearchBox' }).exists()).toBe(false)
      const banner = wrapper.find('.world-banner')
      expect(banner.exists()).toBe(true)
      expect(banner.text()).toContain('published')
      expect(banner.text()).toContain('Search is unavailable')
    })

    // Mutation: drop `&& !isWorldBound.value` from the include arm — include
    // is sent with world, which the API answers 422 world_include_unsupported.
    it('suppresses include=* even with relation columns', async () => {
      const wrapper = await mountList({ relationColumn: true })
      rendersProof(wrapper)

      expect(lastParams().world).toBe('published')
      expect(lastParams().include).toBeUndefined()
    })
  })

  // An unknown world is a 400 (unknown_world) — the API names it, because a
  // world name is operator config and not a secret.
  describe('when the world request fails', () => {
    it('does not claim to be showing the world', async () => {
      mockRoute.query = { world: 'nonexistent' }
      seedSchema()
      listEntitiesMock.mockRejectedValue(
        Object.assign(new Error('no world named "nonexistent" is declared'), {
          status: 400,
        }),
      )
      const wrapper = mount(EntityList, {
        props: { listId },
        attachTo: document.body,
        global: { plugins: [pinia, PiniaColada] },
      })
      await flushPromises()

      // Anti-vacuity: the request really was attempted and really failed, so
      // the absence below is about the banner and not about a dead component.
      expect(listEntitiesMock).toHaveBeenCalled()
      expect(wrapper.text()).toContain('nonexistent')

      // Mutation: drop `&& !loadError` from the banner's v-if — the page then
      // says "Showing the nonexistent world" directly above an error saying
      // no such world exists.
      expect(wrapper.find('.world-banner').exists()).toBe(false)
    })
  })

  // `?world=default` is the API's explicit spelling of the default world, so
  // it must behave exactly like no param at all — not like a bound world.
  describe('explicit default world', () => {
    it('behaves as the default world', async () => {
      mockRoute.query = { world: 'default' }
      const wrapper = await mountList()
      rendersProof(wrapper)

      expect(lastParams().world).toBeUndefined()
      expect(wrapper.findComponent({ name: 'SearchBox' }).exists()).toBe(true)
      expect(wrapper.find('.world-banner').exists()).toBe(false)
    })
  })
})
