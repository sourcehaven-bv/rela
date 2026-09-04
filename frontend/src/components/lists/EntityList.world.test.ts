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

  function seedSchema(opts: { relationColumn?: boolean } = {}) {
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

  async function mountList(opts: { relationColumn?: boolean } = {}) {
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

    it('does nothing under a world', async () => {
      mockRoute.query = { world: 'published' }
      await pressJThenE(await mountList())
      expect(routerPush).not.toHaveBeenCalledWith(expect.stringContaining('/form/'))
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
    it('offers the search box and says what it searches', async () => {
      const wrapper = await mountList()
      rendersProof(wrapper)

      expect(wrapper.findComponent({ name: 'SearchBox' }).exists()).toBe(true)
      const banner = wrapper.find('.world-banner')
      expect(banner.exists()).toBe(true)
      // The banner must still account for the world's effect on results —
      // rows missing from the list are missing from its search too. Dropping
      // that sentence would leave a reader with a short result set and no
      // explanation, which is what the note exists to prevent.
      expect(banner.text()).toContain('including from search')
    })

    // The banner is TWO things with different owners, the same split the
    // detail page already makes.
    //
    // It used to hardcode "Showing the <world> world", which on an ISMS
    // `published` list was noise: published IS the reader's normal state, so
    // announcing it says nothing. The announcement is now the operator's
    // `banner:` and renders nothing when unset.
    //
    // The NOTE stays unconditional because both its sentences are facts about
    // the REQUEST — the world really does filter rows out, and the search box
    // beside it searches those same faces. An operator who could suppress those
    // would leave a reader with a short list, and a search that looks broken
    // because it silently declines to find what the world excludes.
    describe('the banner announcement is operator config; the note is not', () => {
      it('renders the operator announcement when the world declares one', async () => {
        useSchemaStore().worlds.set('published', {
          readable: true,
          banner: 'These policies are in force',
        } as never)
        const wrapper = await mountList()
        rendersProof(wrapper)

        const banner = wrapper.find('.world-banner')
        expect(banner.text()).toContain('These policies are in force')
        expect(banner.text()).toContain('including from search')
      })

      it('announces NOTHING when the world declares no banner', async () => {
        useSchemaStore().worlds.set('published', { readable: true } as never)
        const wrapper = await mountList()
        rendersProof(wrapper)

        // The announcement element is absent — not an empty span, which would
        // still occupy the layout.
        expect(wrapper.find('.world-banner__label').exists()).toBe(false)
        // ...and the note survives, which is the half an operator must not be
        // able to turn off. Without this assertion, deleting the banner block
        // outright would pass.
        expect(wrapper.find('.world-banner__note').text()).toContain('including from search')
      })

      it('never announces the world NAME on its own', async () => {
        // The specific regression: the hardcoded string named the world, so a
        // `published` list announced "published" to a reader for whom that is
        // simply the normal state.
        useSchemaStore().worlds.set('published', { readable: true } as never)
        const wrapper = await mountList()
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
  // thing separating them, and until now the list rendered none of it, so an
  // editorial list showing a stand-in looked exactly like one showing the face
  // the reader asked for.
  //
  // The badge is an EXCEPTION marker: it renders ONLY for a stand-in. A
  // first-choice row is left unmarked, because a badge on every row is noise
  // that trains the reader to ignore it — so here the badge's PRESENCE is
  // itself the claim, and the no-badge cases below are what give it meaning.
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

    async function mountWith(rows: Entity[]) {
      seedSchema()
      seedEntities(rows)
      const wrapper = mount(EntityList, {
        props: { listId },
        attachTo: document.body,
        global: { plugins: [pinia, PiniaColada] },
      })
      await flushPromises()
      expect(listEntitiesMock).toHaveBeenCalled()
      return wrapper
    }

    it('badges a within-chain fallback row as a substitute, naming the face served', async () => {
      mockRoute.query = { world: 'editorial' }
      const wrapper = await mountWith([fallbackRow])
      expect(wrapper.text()).toContain('Remote Working Policy')

      const badge = wrapper.find('.world-badge')
      expect(badge.exists()).toBe(true)
      // The face, not the world name: "published" is what the reader is
      // actually looking at, and the reason the row may not say what they
      // expect.
      expect(badge.text()).toBe('published')
      // Mutation: drop chain_position from isSubstitute and this row stops
      // rendering a badge at all, i.e. the stand-in becomes indistinguishable
      // from the world's first choice — the exact bug the badge exists for.
      expect(badge.classes()).toContain('is-fallback')
      expect(badge.attributes('title')).toContain('No editorial face exists')
    })

    it('leaves a chain-position-0 row UNBADGED even inside a world', async () => {
      mockRoute.query = { world: 'editorial' }
      const wrapper = await mountWith([firstChoiceRow])
      // Anti-vacuity: the row rendered, and it rendered inside a world — so
      // the absent badge is the rule and not a list that failed to resolve.
      expect(wrapper.text()).toContain('Access Control Policy')

      // The positive control for the test above: without this, badging every
      // row would pass it.
      expect(wrapper.find('.world-badge').exists()).toBe(false)
    })

    it('badges only the stand-in when both kinds share one table', async () => {
      // The clearest statement of the rule: same list, same world, one badge.
      mockRoute.query = { world: 'editorial' }
      const wrapper = await mountWith([firstChoiceRow, fallbackRow])
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
      ])
      expect(wrapper.text()).toContain('Joiners Policy')

      const badge = wrapper.find('.world-badge')
      expect(badge.text()).toBe('default')
      expect(badge.classes()).toContain('is-fallback')
    })

    it('renders NO badge under the default world', async () => {
      mockRoute.query = {}
      const wrapper = await mountWith([policy])
      // Anti-vacuity: the rows really rendered, so the absence is about the
      // badge and not about a component that failed to mount.
      expect(wrapper.text()).toContain('Access Control Policy')
      expect(wrapper.find('.world-badge').exists()).toBe(false)
    })

    // One badge per ROW, not one per cell. A world verdict is a statement
    // about the entity, so repeating it in every column would be noise that
    // also grows with the list's width.
    it('renders exactly one badge per row regardless of column count', async () => {
      mockRoute.query = { world: 'editorial' }
      seedSchema({ relationColumn: true })
      seedEntities([fallbackRow])
      const wrapper = mount(EntityList, {
        props: { listId },
        attachTo: document.body,
        global: { plugins: [pinia, PiniaColada] },
      })
      await flushPromises()
      expect(wrapper.text()).toContain('Remote Working Policy')

      // Two columns are configured; still one badge.
      expect(wrapper.findAll('th').length).toBeGreaterThan(2)
      expect(wrapper.findAll('.world-badge')).toHaveLength(1)
    })
  })


  // TKT-6NCSSC: the row link used to drop the world, so following a row from a
  // world-bound list landed on the DEFAULT face — of the very entity whose row
  // the reader had just seen carrying a fallback badge.
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
  describe('write affordances are withdrawn under a world', () => {
    const deletable: Entity = {
      id: 'POL-1',
      type: entityType,
      properties: { title: 'Access Control Policy' },
      _actions: { update: true, delete: true },
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

    async function mountWith(query: Record<string, string>) {
      mockRoute.query = query
      seedActions()
      seedEntities([deletable])
      const wrapper = mount(EntityList, {
        props: { listId },
        attachTo: document.body,
        global: { plugins: [pinia, PiniaColada] },
      })
      await flushPromises()
      return wrapper
    }

    it('offers delete and the action bar under the DEFAULT world', async () => {
      const wrapper = await mountWith({})
      rendersProof(wrapper)
      // The control. Without it, the world-bound assertions below would pass
      // against a component that renders no buttons for an unrelated reason.
      expect(wrapper.find('.delete-btn').exists()).toBe(true)
    })

    it('withdraws the row delete button under a world', async () => {
      const wrapper = await mountWith({ world: 'published' })
      rendersProof(wrapper)
      expect(wrapper.find('.delete-btn').exists()).toBe(false)
    })

    // The action bar only renders once a row is SELECTED (`v-if=hasSelection`),
    // so the assertion has to select first. Without that, findAll returns an
    // empty list and a for-loop over it asserts nothing — the first version of
    // this test passed against the unfixed component for exactly that reason.
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
      // v-show renders the element and toggles display, so "offered" means
      // present AND not display:none.
      expect((buttons[0].element as HTMLElement).style.display).not.toBe('none')
    })

    it('withdraws the bulk-action bar under a world', async () => {
      const wrapper = await mountWith({ world: 'published' })
      await selectFirstRow(wrapper)
      const buttons = wrapper.findAll('.action-header-btn')
      // Anti-vacuity: the buttons must EXIST (so we know the bar rendered and
      // we are really testing v-show), and every one must be hidden.
      expect(buttons.length).toBeGreaterThan(0)
      for (const b of buttons) {
        expect((b.element as HTMLElement).style.display).toBe('none')
      }
    })
  })

})
