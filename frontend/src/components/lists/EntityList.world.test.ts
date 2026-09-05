import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
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
const deleteEntityMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
  deleteEntity: (...args: unknown[]) => deleteEntityMock(...args),
}))
// The confirm dialog is a host-bound singleton (App.vue); here it answers
// yes, so the delete test reaches the write it is about.
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: async () => true }),
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
  // Renders `to` as a real href so a test can assert the DESTINATION. `to` may
  // be a string or a RouteLocationRaw; the object form is flattened to
  // path?query rather than stringified (which would yield "[object Object]").
  RouterLink: {
    props: ['to'],
    template: '<a :href="href"><slot /></a>',
    computed: {
      href(this: { to: unknown }) {
        const to = this.to
        if (typeof to === 'string') return to
        if (to && typeof to === 'object') {
          const loc = to as { path?: string; query?: Record<string, unknown> }
          const qs = new URLSearchParams(
            Object.entries(loc.query ?? {}).map(([k, v]) => [k, String(v)]),
          ).toString()
          return qs ? `${loc.path}?${qs}` : (loc.path ?? '')
        }
        return ''
      },
    },
  },
}))

describe('EntityList world binding', () => {
  const listId = 'policies'
  const entityType = 'policy'

  function seedSchema(opts: { relationColumn?: boolean; faces?: boolean } = {}) {
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(listId, {
      id: listId,
      title: 'Policies',
      entity: entityType,
      edit_form: 'policy-edit',
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
      // The world NOTE is a fact about a faced type only; tests that assert
      // on it seed faces, and one asserts its absence without them.
      ...(opts.faces ? { faces: { draft: {}, published: {} }, bare_face: 'draft' } : {}),
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

  // Every mounted list is torn down after its test. The keyboard composable
  // listens on `document`, so a component left mounted by an earlier case
  // keeps answering keys — and pushing routes — during later ones.
  const mounted: VueWrapper[] = []
  afterEach(() => {
    for (const w of mounted.splice(0)) w.unmount()
    document.body.innerHTML = ''
  })

  async function mountList(opts: { relationColumn?: boolean; faces?: boolean } = {}) {
    seedSchema(opts)
    seedEntities([policy])
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

  // The `e` shortcut is a write affordance like the row's Edit button. It
  // was world-blind: j then e on a world-bound list opened the edit form on
  // the DEFAULT face of a row the reader saw at another.
  describe('the e shortcut', () => {
    async function pressJThenE(wrapper: VueWrapper) {
      rendersProof(wrapper)
      for (const key of ['j', 'e']) {
        document.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }))
        await flushPromises()
      }
    }

    it('opens the edit form in the default world (the control)', async () => {
      await pressJThenE(await mountList())
      expect(routerPush).toHaveBeenCalledWith('/form/policy-edit/POL-1')
    })

    it('opens the edit form at the row\'s ADDRESS under a world', async () => {
      // The row is a resolved face; the form must edit that face and not the
      // bare id, which the world resolves away from it.
      mockRoute.query = { world: 'published' }
      seedSchema()
      seedEntities([{ ...policy, _self: '/api/v1/policys/POL-1@published' }])
      const wrapper = mount(EntityList, {
        props: { listId },
        attachTo: document.body,
        global: { plugins: [pinia, PiniaColada] },
      })
      mounted.push(wrapper)
      await flushPromises()
      await pressJThenE(wrapper)
      expect(routerPush).toHaveBeenCalledWith('/form/policy-edit/POL-1@published')
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

    // Search is world-scoped now (TKT-9KZGJO), so `q` and `world` travel
    // TOGETHER — the pair the API used to refuse with world_search_unsupported.
    //
    // BOTH params are asserted, and the `world` half is what makes this
    // meaningful: sending `q` alone would be the old leak wearing new clothes,
    // since the server would then answer in the default world and hand a
    // published-world page its draft hits.
    it('sends q alongside world', async () => {
      mockRoute.query = { world: 'published', q: 'access' }
      const wrapper = await mountList()
      rendersProof(wrapper)

      expect(lastParams().q).toBe('access')
      expect(lastParams().world).toBe('published')
    })

    // The UI half of the same property: the affordance is real, so it is
    // offered. Paired with the default-world control above, which proves the
    // box's presence is not simply unconditional.
    it('offers the search box, and says nothing about it unless the operator does', async () => {
      const wrapper = await mountList({ faces: true })
      rendersProof(wrapper)

      expect(wrapper.findComponent({ name: 'SearchBox' }).exists()).toBe(true)
      // No banner: nothing declared for this world. The app has no sentence
      // of its own about what a world filters (TKT-5SZG2L).
      expect(wrapper.find('.world-banner').exists()).toBe(false)
    })

    // The banner is TWO things with different owners, the same split the
    // detail page already makes.
    //
    // It used to hardcode "Showing the <world> world", which on an ISMS
    // `published` list was noise: published IS the reader's normal state, so
    // announcing it says nothing. The announcement is now the operator's
    // `banner:` and renders nothing when unset.
    //
    // Both halves of the banner are operator config: the ANNOUNCEMENT
    // (`banner:`) and the NOTE (`messages.projection`). The note is rendered
    // only on a list of a type that declares faces — a type without faces has
    // one state in every world, so nothing on its list is filtered by the
    // world and the note would be false (atlas worlds issue 1).
    describe('the banner is the operator\'s words or nothing', () => {
      it('renders the announcement and the projection note when both are declared', async () => {
        useSchemaStore().worlds.set('published', {
          readable: true,
          banner: 'These policies are in force',
          messages: { projection: 'Only what is in force is listed.' },
        } as never)
        const wrapper = await mountList({ faces: true })
        rendersProof(wrapper)

        const banner = wrapper.find('.world-banner')
        expect(banner.find('.world-banner__label').text()).toBe('These policies are in force')
        expect(banner.find('.world-banner__note').text()).toBe('Only what is in force is listed.')
      })

      it('renders the announcement alone on a list of a type WITHOUT faces', async () => {
        useSchemaStore().worlds.set('published', {
          readable: true,
          banner: 'These policies are in force',
          messages: { projection: 'Only what is in force is listed.' },
        } as never)
        const wrapper = await mountList()
        rendersProof(wrapper)

        const banner = wrapper.find('.world-banner')
        expect(banner.text()).toContain('These policies are in force')
        expect(banner.text()).not.toContain('Only what is in force')
      })

      it('renders the projection note alone when no banner is declared', async () => {
        useSchemaStore().worlds.set('published', {
          readable: true,
          messages: { projection: 'Only what is in force is listed.' },
        } as never)
        const wrapper = await mountList({ faces: true })
        rendersProof(wrapper)
        expect(wrapper.find('.world-banner__label').exists()).toBe(false)
        expect(wrapper.find('.world-banner__note').text()).toBe('Only what is in force is listed.')
      })

      it('renders no banner at all when nothing is declared', async () => {
        useSchemaStore().worlds.set('published', { readable: true } as never)
        const wrapper = await mountList({ faces: true })
        rendersProof(wrapper)
        expect(wrapper.find('.world-banner').exists()).toBe(false)
      })

      it('never announces the world NAME on its own', async () => {
        // The specific regression: a hardcoded string named the world, so a
        // `published` list announced "published" to a reader for whom that is
        // simply the normal state.
        useSchemaStore().worlds.set('published', { readable: true, banner: 'In force' } as never)
        const wrapper = await mountList({ faces: true })
        rendersProof(wrapper)
        expect(wrapper.find('.world-banner').text()).not.toContain('published')
      })
    })

    // INVERTED by TKT-WRLDAPI item 4, like its sibling in stores/entities.ts.
    //
    // This asserted that include=* was SUPPRESSED under a world, because the
    // API refused the combination and neighbour resolution was default-world.
    // Item 4 made neighbour resolution world-scoped and removed the refusal,
    // so suppressing it now costs relation columns their content for nothing.
    //
    // Inverted rather than deleted: a regression that reinstated the
    // suppression would be INVISIBLE — relation columns would simply render
    // empty again under a world, which is indistinguishable from "these rows
    // have no relations". That invisibility is exactly how the workaround
    // survived item 4 in the first place.
    //
    // Mutation: restore `&& !isWorldBound.value` — include goes missing and
    // the first assertion fails.
    it('sends include=* WITH the world when there are relation columns', async () => {
      const wrapper = await mountList({ relationColumn: true })
      rendersProof(wrapper)

      expect(lastParams().include).toBe('*')
      expect(lastParams().world).toBe('published')
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

  // Per-row face provenance on the list (the gap TKT-F2D5U5 left open).
  //
  // A list row that fell back to a later chain candidate is BYTE-IDENTICAL to
  // a first-choice hit — same id, same title, same cells. `_world` is the only
  // thing separating them. The badge is an EXCEPTION marker: it renders ONLY
  // for a stand-in, and only in the operator's words (`messages.stand_in`);
  // with nothing declared it renders nothing (TKT-5SZG2L).
  describe('per-row world badge', () => {
    const fallbackRow: Entity = {
      id: 'POL-3',
      type: entityType,
      properties: { title: 'Remote Working Policy' },
      _world: { name: 'editorial', face: 'published', via: 'chain', chain_position: 1 },
    }
    const firstChoiceRow: Entity = {
      id: 'POL-1',
      type: entityType,
      properties: { title: 'Access Control Policy' },
      _world: { name: 'editorial', face: 'draft', via: 'chain', chain_position: 0 },
    }

    async function mountWith(rows: Entity[], standIn?: string) {
      seedSchema({ faces: true })
      useSchemaStore().worlds.set('editorial', {
        readable: true, ...(standIn ? { messages: { stand_in: standIn } } : {}),
      } as never)
      seedEntities(rows)
      const wrapper = mount(EntityList, {
        props: { listId },
        attachTo: document.body,
        global: { plugins: [pinia, PiniaColada] },
      })
      mounted.push(wrapper)
      await flushPromises()
      expect(listEntitiesMock).toHaveBeenCalled()
      return wrapper
    }

    it('badges a within-chain fallback row in the operator\'s words', async () => {
      mockRoute.query = { world: 'editorial' }
      const wrapper = await mountWith([fallbackRow], 'Stand-in: {face}')
      expect(wrapper.text()).toContain('Remote Working Policy')

      const badge = wrapper.find('.world-badge')
      expect(badge.exists()).toBe(true)
      expect(badge.text()).toBe('Stand-in: published')
      expect(badge.classes()).toContain('is-fallback')
      expect(badge.attributes('title')).toBeUndefined()
    })

    it('renders NO badge for a stand-in when the world declares no stand_in text', async () => {
      mockRoute.query = { world: 'editorial' }
      const wrapper = await mountWith([fallbackRow])
      expect(wrapper.text()).toContain('Remote Working Policy')
      expect(wrapper.find('.world-badge').exists()).toBe(false)
    })

    it('leaves a chain-position-0 row UNBADGED even inside a world', async () => {
      mockRoute.query = { world: 'editorial' }
      const wrapper = await mountWith([firstChoiceRow], '{face}')
      expect(wrapper.text()).toContain('Access Control Policy')
      expect(wrapper.find('.world-badge').exists()).toBe(false)
    })

    it('badges only the stand-in when both kinds share one table', async () => {
      mockRoute.query = { world: 'editorial' }
      const wrapper = await mountWith([firstChoiceRow, fallbackRow], '{face}')
      expect(wrapper.text()).toContain('Access Control Policy')
      expect(wrapper.text()).toContain('Remote Working Policy')

      const badges = wrapper.findAll('.world-badge')
      expect(badges).toHaveLength(1)
      expect(badges[0].text()).toBe('published')
    })

    it('badges `otherwise: default` fallback rows too', async () => {
      mockRoute.query = { world: 'editorial' }
      const wrapper = await mountWith([
        {
          id: 'POL-4',
          type: entityType,
          properties: { title: 'Joiners Policy' },
          _world: { name: 'editorial', face: '', via: 'fallback-default' },
        },
      ], '{bare_face}')
      expect(wrapper.text()).toContain('Joiners Policy')

      const badge = wrapper.find('.world-badge')
      expect(badge.text()).toBe('draft')
      expect(badge.classes()).toContain('is-fallback')
    })

    it('renders NO badge under the default world', async () => {
      mockRoute.query = {}
      const wrapper = await mountWith([policy], '{face}')
      expect(wrapper.text()).toContain('Access Control Policy')
      expect(wrapper.find('.world-badge').exists()).toBe(false)
    })

    it('renders exactly one badge per row regardless of column count', async () => {
      mockRoute.query = { world: 'editorial' }
      seedSchema({ faces: true, relationColumn: true })
      useSchemaStore().worlds.set('editorial', { readable: true, messages: { stand_in: '{face}' } } as never)
      seedEntities([fallbackRow])
      const wrapper = mount(EntityList, {
        props: { listId },
        attachTo: document.body,
        global: { plugins: [pinia, PiniaColada] },
      })
      mounted.push(wrapper)
      await flushPromises()
      expect(wrapper.text()).toContain('Remote Working Policy')
      expect(wrapper.findAll('.world-badge')).toHaveLength(1)
    })
  })

  describe('row links carry the world', () => {
    const linkRow: Entity = {
      id: 'POL-3',
      type: entityType,
      properties: { title: 'Remote Working Policy' },
    }

    it('adds the world to the row target', async () => {
      mockRoute.query = { world: 'editorial' }
      seedSchema()
      seedEntities([linkRow])
      const wrapper = mount(EntityList, {
        props: { listId },
        attachTo: document.body,
        global: { plugins: [pinia, PiniaColada] },
      })
      await flushPromises()

      const link = wrapper.find('a.row-link')
      expect(link.exists()).toBe(true)
      expect(link.attributes('href')).toContain('/entity/')
      expect(link.attributes('href')).toContain('world=editorial')
    })

    it('adds no world param under the default world', async () => {
      mockRoute.query = {}
      seedSchema()
      seedEntities([linkRow])
      const wrapper = mount(EntityList, {
        props: { listId },
        attachTo: document.body,
        global: { plugins: [pinia, PiniaColada] },
      })
      await flushPromises()

      const link = wrapper.find('a.row-link')
      expect(link.exists()).toBe(true)
      expect(link.attributes('href')).toContain('/entity/')
      expect(link.attributes('href')).not.toContain('world=')
    })
  })

  // BUG-Y0GNSB follow-up (point 4): EntityList's write affordances were
  // world-blind. `canCreate` had a world term but `canDelete`/`canUpdate` did
  // not, so a world-bound list still offered row delete, the Delete/Backspace
  // shortcut, and the bulk-action bar.
  //
  // Why that mattered enough to guard client-side: a bare write carries no
  // `?world=`, so `attachWorld` has no parameter to refuse — the server
  // returns 200 and the write lands on the DEFAULT face while the reader is
  // looking at a resolved one. There is no error to show, which is precisely
  // what makes the silent case worth preventing at the affordance.
  // Under a world every write goes to the row's ADDRESS (`_self`, face
  // included), so the affordances are `_actions` alone — the server computes
  // them for the face each row shows. The world itself withdraws nothing.
  describe('write affordances follow _actions and the address under a world', () => {
    const deletable: Entity = {
      id: 'POL-1',
      type: entityType,
      properties: { title: 'Access Control Policy' },
      _actions: { update: true, delete: true },
      _self: '/api/v1/policys/POL-1@published',
    }

    function seedActions() {
      const schemaStore = useSchemaStore()
      schemaStore.lists.set(listId, {
        id: listId,
        title: 'Policies',
        entity: entityType,
        columns: [{ property: 'title', label: 'Title' }],
        actions: ['approve'],
      } as never)
      schemaStore.entityTypes.set(entityType, {
        name: entityType,
        label: 'Policy',
        properties: { title: { type: 'string', values: null } },
      } as never)
      schemaStore.actions.set('approve', {
        id: 'approve',
        label: 'Approve',
        key: 'a',
        set: { status: 'approved' },
      } as never)
    }

    async function mountWith(query: Record<string, string>, rows: Entity[] = [deletable]) {
      mockRoute.query = query
      seedActions()
      seedEntities(rows)
      const wrapper = mount(EntityList, {
        props: { listId },
        attachTo: document.body,
        global: { plugins: [pinia, PiniaColada] },
      })
      mounted.push(wrapper)
      await flushPromises()
      return wrapper
    }

    it('offers delete and the action bar under the DEFAULT world', async () => {
      const wrapper = await mountWith({})
      rendersProof(wrapper)
      expect(wrapper.find('.delete-btn').exists()).toBe(true)
    })

    it('keeps the row delete button under a world when _actions permits it', async () => {
      const wrapper = await mountWith({ world: 'published' })
      rendersProof(wrapper)
      expect(wrapper.find('.delete-btn').exists()).toBe(true)
    })

    it('withdraws the row delete button when the served face is not deletable', async () => {
      // Same fixture, the server's verdict flipped: the difference is the
      // verdict and nothing else.
      const wrapper = await mountWith({ world: 'published' }, [
        { ...deletable, _actions: { update: true, delete: false } },
      ])
      rendersProof(wrapper)
      expect(wrapper.find('.delete-btn').exists()).toBe(false)
    })

    it('deletes by the row\'s ADDRESS, face included', async () => {
      deleteEntityMock.mockReset().mockResolvedValue(undefined)
      const wrapper = await mountWith({ world: 'published' })
      rendersProof(wrapper)
      await wrapper.find('.delete-btn').trigger('click')
      await flushPromises()
      expect(deleteEntityMock).toHaveBeenCalledWith(entityType, 'POL-1@published')
    })

    // The action bar only renders once a row is SELECTED (`v-if=hasSelection`),
    // so the assertion has to select first.
    async function selectFirstRow(wrapper: VueWrapper) {
      const box = wrapper.find('.select-cell input[type="checkbox"]')
      expect(box.exists()).toBe(true)
      await box.setValue(true)
      await flushPromises()
    }

    it('shows the bulk-action bar under the DEFAULT world', async () => {
      const wrapper = await mountWith({})
      await selectFirstRow(wrapper)
      const buttons = wrapper.findAll('.action-header-btn')
      expect(buttons.length).toBeGreaterThan(0)
      expect((buttons[0].element as HTMLElement).style.display).not.toBe('none')
    })

    it('keeps the bulk-action bar under a world when _actions permits it', async () => {
      const wrapper = await mountWith({ world: 'published' })
      await selectFirstRow(wrapper)
      const buttons = wrapper.findAll('.action-header-btn')
      expect(buttons.length).toBeGreaterThan(0)
      expect((buttons[0].element as HTMLElement).style.display).not.toBe('none')
    })

    it('withdraws the bulk-action bar when no selected row is updatable', async () => {
      const wrapper = await mountWith({ world: 'published' }, [
        { ...deletable, _actions: { update: false, delete: true } },
      ])
      await selectFirstRow(wrapper)
      const buttons = wrapper.findAll('.action-header-btn')
      expect(buttons.length).toBeGreaterThan(0)
      for (const b of buttons) {
        expect((b.element as HTMLElement).style.display).toBe('none')
      }
    })
  })

})
