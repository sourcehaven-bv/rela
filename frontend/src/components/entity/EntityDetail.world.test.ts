import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import { useSchemaStore } from '@/stores/schema'
import type { Entity, CopyOffer, EntityWorld } from '@/types'
import type { ViewResponse } from '@/api'

// The world-bound DETAIL surface (TKT-F2D5U5).
//
// Two properties are pinned here, and they are different in kind:
//
//  1. A world-bound page is READ-ONLY. The API refuses every write carrying
//     `?world=` (422 world_read_only) while `_actions` still reports
//     `update: true` — it answers "may this principal write this entity",
//     which is a question about the principal, not about the request. So the
//     page ANDs the world in. Without that the Edit button renders and its
//     save fails: an affordance that promises a verb the surface rejects.
//
//  2. Copy offers (RULING 9) ride `_copies` on the entry, and only ALLOWED
//     ones render, as ABSENT rather than disabled.
//
// Ruling 10 note — nearly every assertion below is an ABSENCE, the class that
// passes trivially against a component that failed to mount. `rendersProof`
// is the shared guard, and each absence is paired with the same assertion
// passing in the positive direction from a sibling mount.

const fetchViewMock = vi.fn()
const getCommandsMock = vi.fn()
const invokeCopyMock = vi.fn()

vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: (...a: unknown[]) => getCommandsMock(...a),
}))
vi.mock('@/api/copies', () => ({
  invokeCopy: (...a: unknown[]) => invokeCopyMock(...a),
}))

const routerPush = vi.fn()
const mockRoute: { query: Record<string, unknown>; path: string; name: string } = {
  query: {},
  path: '/entity/policy/POL-1',
  name: 'entity',
}
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush, replace: vi.fn() }),
  useRoute: () => mockRoute,
  RouterLink: { template: '<a><slot /></a>' },
}))

const entityType = 'policy'
const entityId = 'POL-1'

function promoteOffer(over: Partial<CopyOffer> = {}): CopyOffer {
  return {
    name: 'promote-policy',
    label: 'Publish this policy',
    targetFace: 'policy@published',
    sameEntity: true,
    allowed: true,
    ...over,
  }
}

function entry(over: Partial<Entity> = {}): Entity {
  return {
    id: entityId,
    type: entityType,
    _title: 'Access Control Policy',
    properties: { title: 'Access Control Policy' },
    content: '# Access Control Policy',
    // The map the server really sends under a world: writes are permitted for
    // this PRINCIPAL, which is true and is not the question the page asks.
    _actions: { update: true, delete: true, rename: true },
    ...over,
  }
}

function viewResponse(over: Partial<Entity> = {}): ViewResponse {
  return {
    entry: entry(over),
    sections: [],
  }
}

describe('EntityDetail world binding', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    const schemaStore = useSchemaStore()
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'Policy',
      properties: { title: { type: 'string', values: null } },
    } as never)
    // The Edit button also requires an edit form to exist
    // (`v-if="editFormId && !isInaccessible && canUpdate"`). Without this the
    // Edit assertions below pass VACUOUSLY — verified by mutation: dropping
    // `!isWorldBound` from canUpdate survived the suite until this was seeded.
    schemaStore.forms.set('policy-edit', {
      id: 'policy-edit',
      entity: entityType,
      mode: 'edit',
      title: 'Edit policy',
    } as never)
    fetchViewMock.mockReset()
    getCommandsMock.mockReset().mockResolvedValue([])
    invokeCopyMock.mockReset()
    routerPush.mockClear()
    mockRoute.query = {}
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  async function mountDetail(view: ViewResponse) {
    fetchViewMock.mockResolvedValue(view)
    const wrapper = mount(EntityDetail, {
      props: { entityType, entityId },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    return wrapper
  }

  // The anti-vacuity guard: proves the page actually rendered, so that an
  // "absent" assertion is a statement about the page rather than about a
  // crashed setup.
  function rendersProof(wrapper: { text: () => string }) {
    expect(fetchViewMock).toHaveBeenCalled()
    expect(wrapper.text()).toContain('Access Control Policy')
  }

  // Match a BUTTON by its label rather than searching whole-page text.
  // `wrapper.text().not.toContain('Edit')` is a substring match over the
  // entire render, so it is satisfied — or broken — by unrelated copy: the
  // fixture's own form is titled "Edit policy", and any future heading
  // containing the word would flip the result without the gate changing.
  function button(wrapper: VueWrapper, label: string) {
    return wrapper.findAll('button').find((b) => b.text().replace(/\s+/g, ' ').trim().startsWith(label))
  }

  describe('the world rides the request', () => {
    it('sends no world under the default world', async () => {
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(fetchViewMock).toHaveBeenCalledWith(entityType, entityId, undefined)
      expect(w.find('.world-banner').exists()).toBe(false)
    })

    it('sends the world named by the URL', async () => {
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(fetchViewMock).toHaveBeenCalledWith(entityType, entityId, 'published')
      expect(w.find('.world-banner').text()).toContain('published')
    })
  })

  describe('a world-bound page is read-only', () => {
    it('offers Edit and Delete under the default world', async () => {
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      // The positive control for both absence assertions below. `_actions` is
      // identical in the world-bound case, so this is the ONLY thing that
      // distinguishes "hidden because of the world" from "hidden always".
      expect(button(w, 'Delete')).toBeDefined()
    })

    it('hides Delete under a world even though _actions permits it', async () => {
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      // Same `_actions: {delete: true}` as the passing case above.
      expect(w.find('.btn-danger').exists()).toBe(false)
    })

    it('hides Edit under a world even though _actions permits it', async () => {
      // The Delete case above pins `canDelete`; this pins `canUpdate`, a
      // SEPARATE computed that the Delete assertion cannot cover. Mutating
      // `canUpdate` to drop its `!isWorldBound` term survives the suite
      // without this test — and survived it WITH this test too, until the
      // fixture grew an edit form, because the button has a second
      // precondition (`editFormId`) that the fixture did not satisfy. A test
      // asserting an absence against an element that could never appear
      // proves nothing.
      //
      // Edit is the WORSE half to leave unpinned. Delete under a world merely
      // 422s; Edit opens a form, the user types, and the save fails — work
      // lost to an affordance that promised a verb the surface rejects.
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      // Same `_actions: {update: true}` as the default-world case, which
      // renders Edit — so this distinguishes "hidden by the world" from
      // "hidden always".
      expect(button(w, 'Edit')).toBeUndefined()
    })

    it('offers Edit under the default world (the control for the case above)', async () => {
      // Without this, hiding Edit unconditionally — or a fixture that never
      // satisfies `editFormId` — passes the assertion above while proving
      // nothing. This is the half that was missing when the canUpdate mutant
      // survived.
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(button(w, 'Edit')).toBeDefined()
    })

    it('hides "Go to draft" when the principal cannot read the default world', async () => {
      // A GLOBAL role-level grant, reported per world by `/_schema`.worlds —
      // no per-entity probe, so no existence oracle and no extra request.
      // Offering the button to someone whose default-world request returns an
      // empty result would send them somewhere that looks broken.
      useSchemaStore().worlds.set('default', { readable: false, default: true })
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(w.text()).not.toContain('Go to draft')
      // The banner itself still renders — proving the absence is the button's
      // gate and not a failure to render the whole block.
      expect(w.find('.world-banner').exists()).toBe(true)
    })

    it('shows "Go to draft" when the world map is EMPTY (older server)', async () => {
      // Unknown world defaults to readable: hiding a working affordance
      // because the schema had not loaded would read as a permission problem,
      // which is the wrong answer in the direction nobody can debug.
      useSchemaStore().worlds.clear()
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(w.text()).toContain('Go to draft')
    })

    it('renders NO inline-edit surface under a world', async () => {
      // The autosave paths (handlePropertyApplied / handleRowPropertyApplied,
      // and the content channel) are reachable ONLY as props on a
      // SectionEditForm. All three of its call sites route through
      // sectionShouldRouteToInlineEdit / rowShouldRouteToInlineEdit, both of
      // which refuse under a world — so no pending save can be CREATED while
      // world-bound, and the two commitImmediately() flushes have nothing to
      // flush. This test is what makes that argument checkable rather than a
      // claim in a comment.
      mockRoute.query = { world: 'published' }
      const w = await mountDetail({
        entry: entry(),
        sections: [{
          heading: 'Properties',
          sectionId: 'props',
          display: 'properties',
          isEmpty: false,
          isGrouped: false,
          hasContent: false,
          fields: [{ property: 'title', label: 'Title', values: ['Access Control Policy'], render: 'input' }],
        }],
      } as never)
      rendersProof(w)
      expect(w.findComponent({ name: 'SectionEditForm' }).exists()).toBe(false)
    })

    it('offers the way back to the default world', async () => {
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)

      const back = w
        .findAll('button')
        .find((b) => b.text().includes('Go to draft'))
      expect(back).toBeDefined()
      await back!.trigger('click')
      // Returns to the same id with the world dropped — writes land there.
      expect(routerPush).toHaveBeenCalledWith({ query: {} })
    })
  })

  describe('copy affordances ride the entity response', () => {
    it('renders an allowed offer', async () => {
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      rendersProof(w)
      expect(w.text()).toContain('Publish this policy')
    })

    it('renders NOTHING for a denied offer', async () => {
      const denied = promoteOffer({ allowed: false, reason: 'nope' })
      const w = await mountDetail(viewResponse({ _copies: [denied] }))
      rendersProof(w)
      expect(w.text()).not.toContain('Publish this policy')
      // The reason is a debugging tooltip on the wire, never rendered text.
      expect(w.text()).not.toContain('nope')
    })

    it('hides copy offers under a world, even when the server offers them', async () => {
      // The server's offer is CORRECT here: under a world falling back to the
      // default face, the resolved face IS the draft, so promote applies. But
      // an invoke carries no `?world=`, so it would write the default face
      // while the reader looks at a resolved one — and a Publish button beside
      // a "read-only" banner is an affordance that contradicts the page.
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      rendersProof(w)
      expect(w.text()).not.toContain('Publish this policy')

      // The positive control: the SAME response under the default world DOES
      // render it. Without this, hiding copies unconditionally would pass.
      mockRoute.query = {}
      const dflt = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      expect(dflt.text()).toContain('Publish this policy')
    })

    it('invokes by NAME with the source id, and no target for a same-entity copy', async () => {
      invokeCopyMock.mockResolvedValue({
        definition: 'promote-policy',
        entityId,
        pointer: 'published',
        created: true,
      })
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      rendersProof(w)

      const btn = w
        .findAll('button')
        .find((b) => b.text() === 'Publish this policy')
      expect(btn).toBeDefined()
      await btn!.trigger('click')
      await flushPromises()

      // A same-entity copy targets the source by construction; the target id
      // is OMITTED rather than sent empty (the kernel rejects a target on a
      // same-entity copy).
      expect(invokeCopyMock).toHaveBeenCalledWith('promote-policy', entityId)
    })

    it('reloads after a copy so the offers recompute', async () => {
      invokeCopyMock.mockResolvedValue({
        definition: 'promote-policy',
        entityId,
        pointer: 'published',
        created: true,
      })
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      const before = fetchViewMock.mock.calls.length

      const btn = w.findAll('button').find((b) => b.text() === 'Publish this policy')
      await btn!.trigger('click')
      await flushPromises()

      // A face that now exists may no longer be offered.
      expect(fetchViewMock.mock.calls.length).toBeGreaterThan(before)
    })

    it('does not reload or misattribute when the entity changes mid-invoke', async () => {
      // `copyBusy` only disables THIS menu's button; the scope-nav shortcuts
      // (P/N), the back button and the sidebar stay live, so the page can be
      // showing a different entity by the time the invoke resolves.
      //
      // Before the fix this fired a SECOND fetchView for the entity the user
      // had navigated TO — an entity the copy never touched — and showed an
      // unqualified "Created the published face" beside it.
      let resolveInvoke: (v: unknown) => void = () => {}
      invokeCopyMock.mockReturnValue(new Promise((r) => { resolveInvoke = r }))
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      const btn = w.findAll('button').find((b) => b.text() === 'Publish this policy')
      await btn!.trigger('click')

      await w.setProps({ entityId: 'POL-2' })
      await flushPromises()
      const afterNav = fetchViewMock.mock.calls.length

      resolveInvoke({
        definition: 'promote-policy', entityId, pointer: 'published', created: true,
      })
      await flushPromises()

      // The copy still targeted the entity the user clicked on...
      expect(invokeCopyMock).toHaveBeenCalledWith('promote-policy', entityId)
      // ...and it did NOT reload the unrelated entity now on screen.
      expect(fetchViewMock.mock.calls.length).toBe(afterNav)
    })

    it('surfaces a refusal rather than pretending the copy ran', async () => {
      invokeCopyMock.mockRejectedValue(new Error('forbidden'))
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      const before = fetchViewMock.mock.calls.length

      const btn = w.findAll('button').find((b) => b.text() === 'Publish this policy')
      await btn!.trigger('click')
      await flushPromises()

      // `allowed` is a hint; the kernel re-authorizes. A 403 here is the
      // boundary working, and the page must not reload as if it succeeded.
      expect(fetchViewMock.mock.calls.length).toBe(before)
    })
  })

  describe('per-neighbour provenance', () => {
    const world = (via: EntityWorld['via'], pointer: string): EntityWorld => ({
      name: 'site-nl',
      pointer,
      via,
    })

    function withCollection(entities: unknown[]): ViewResponse {
      return {
        entry: entry(),
        sections: [
          {
            heading: 'Implements',
            sectionId: 'implements',
            display: 'list',
            isEmpty: false,
            isGrouped: false,
            hasContent: false,
            entities: entities as never,
          },
        ],
      }
    }

    it('distinguishes a real face from a fallback in the SAME section', async () => {
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail(
        withCollection([
          { id: 'CTL-1', type: 'control', title: 'Real Dutch', hasContent: false, _world: world('chain', 'nl') },
          { id: 'CTL-2', type: 'control', title: 'Fallback', hasContent: false, _world: world('fallback-default', '') },
        ]),
      )
      rendersProof(w)

      const badges = w.findAllComponents({ name: 'WorldBadge' })
      // Per-neighbour: each entity resolved independently, so one section
      // carries both kinds. This is the whole point of RULING 12.
      const rendered = badges.filter((b) => b.text() !== '')
      expect(rendered).toHaveLength(2)
      expect(w.find('.is-fallback').exists()).toBe(true)
      expect(w.find('.is-chain').exists()).toBe(true)
    })

    it('renders no badge under the default world', async () => {
      const w = await mountDetail(
        withCollection([
          { id: 'CTL-1', type: 'control', title: 'Plain', hasContent: false },
        ]),
      )
      rendersProof(w)
      expect(w.find('.world-badge').exists()).toBe(false)
    })
  })
})
