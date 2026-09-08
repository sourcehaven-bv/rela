import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import { useSchemaStore } from '@/stores/schema'
import type { Entity, CopyOffer, EntityWorld } from '@/types'
import type { ViewEntity, ViewResponse, ViewSection } from '@/api'
import type { CopyInvokeResult } from '@/api/copies'

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
// The WRITE mock. Asserting on this rather than on an internal spy is what
// makes the world guards testable at the boundary that matters: whether a
// PATCH leaves the client at all.
const updateEntityMock = vi.fn()

vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: (...a: unknown[]) => getCommandsMock(...a),
}))
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  updateEntity: (...a: unknown[]) => updateEntityMock(...a),
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
  // Renders `to` as a real href so a test can assert the DESTINATION, not just
  // that a link exists. `to` may be a string or a RouteLocationRaw, so the
  // object form is flattened to path?query rather than stringified (which
  // would yield "[object Object]").
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

const entityType = 'policy'
const entityId = 'POL-1'

function promoteOffer(over: Partial<CopyOffer> = {}): CopyOffer {
  return {
    name: 'promote-policy',
    label: 'Publish this policy',
    targetFace: 'policy@published',
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

// A view section with the booleans every section carries defaulted, so a test
// names only what it is about.
function section(over: Partial<ViewSection> & Pick<ViewSection, 'sectionId' | 'display'>): ViewSection {
  return { heading: '', isEmpty: false, isGrouped: false, hasContent: false, ...over }
}

// One collection entity — a neighbour resolved through the world.
function collectionEntity(over: Partial<ViewEntity> & Pick<ViewEntity, 'id'>): ViewEntity {
  return { type: 'control', title: over.id, hasContent: false, ...over }
}

// A settled invoke: the promote CREATED the published face.
function copyResult(): CopyInvokeResult {
  return { definition: 'promote-policy', entityId, face: 'published', created: true }
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
    updateEntityMock.mockReset().mockResolvedValue({
      id: entityId, type: entityType, properties: {}, content: '', _actions: {},
    })
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
  //
  // SCOPE: it proves the ENTRY rendered, nothing more. Tests asserting on
  // COLLECTION entities are not carried by it — their own positive controls
  // (e.g. the badge-count assertions in the provenance block) are what make
  // them non-vacuous. Do not read a rendersProof call as blanket cover.
  function rendersProof(wrapper: { text: () => string }) {
    expect(fetchViewMock).toHaveBeenCalled()
    expect(wrapper.text()).toContain('Access Control Policy')
  }

  // Match a BUTTON by its label rather than searching whole-page text.
  // `wrapper.text().not.toContain('Edit')` is a substring match over the
  // entire render, so it is satisfied — or broken — by unrelated copy: the
  // fixture's own form is titled "Edit policy", and any future heading
  // containing the word would flip the result without the gate changing.
  // Header affordances are a MIX since TKT-3CSZRG: pure navigation (Edit,
  // History) renders as a real link so cmd/middle-click works, while mutations
  // (Delete) stay <button>. A caller asking for "the Edit control" should not
  // have to know which, so this searches both.
  function button(wrapper: VueWrapper, label: string) {
    return wrapper
      .findAll('button, a')
      .find((b) => b.text().replace(/\s+/g, ' ').trim().startsWith(label))
  }

  // Clicks the promote offer and lets the invoke settle. Asserting the button
  // exists first is what keeps the assertions after it from passing vacuously.
  async function clickPromote(wrapper: VueWrapper) {
    const btn = button(wrapper, 'Publish this policy')
    expect(btn).toBeDefined()
    await btn!.trigger('click')
    await flushPromises()
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
      // The banner RENDERS, but its announcement is now operator config
      // (`banner:` on the world) rather than a hardcoded "Showing the X
      // world". This fixture declares no banner, so only the read-only note
      // and the way back appear — both unconditional on a non-default world.
      expect(w.find('.world-banner').exists()).toBe(true)
      expect(w.find('.world-banner').text()).toContain('Read-only in this world')
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

    it('offers the way back when the world map is EMPTY (older server)', async () => {
      // Unknown world defaults to readable: hiding a working affordance
      // because the schema had not loaded would read as a permission problem,
      // which is the wrong answer in the direction nobody can debug.
      useSchemaStore().worlds.clear()
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      // "default" is the fallback wording for a type that declares no name for
      // its default face — jargon, but type-neutral and never wrong. The
      // labelled spellings are asserted below.
      expect(w.text()).toContain('Go to default')
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
        sections: [section({
          heading: 'Properties',
          sectionId: 'props',
          display: 'properties',
          fields: [{ property: 'title', label: 'Title', values: ['Access Control Policy'], render: 'input' }],
        })],
      })
      rendersProof(w)
      expect(w.findComponent({ name: 'SectionEditForm' }).exists()).toBe(false)
    })

    it('refuses a checkbox toggle under a world (G1)', async () => {
      // The checkbox lives inside v-html markdown and is caught by a delegated
      // handler, so there is no element to `v-if` — the guard has to be in
      // handleCheckboxToggle, and this is what holds it there.
      //
      // The server is NOT a backstop: `attachWorld` refuses a write only when
      // `?world=` is on the WRITE request, and useAutoSave never attaches it.
      // So without the guard this silently PATCHes the DEFAULT face while the
      // user is reading a resolved one — no 422, no error, wrong state.
      const contentSection = section({
        sectionId: 'content',
        display: 'content',
        hasContent: true,
        content: '- [ ] a task',
      })
      mockRoute.query = { world: 'published' }
      const w = await mountDetail({
        entry: entry({ content: '- [ ] a task' }),
        sections: [contentSection],
      })
      rendersProof(w)

      const box = w.find('input[type="checkbox"][data-cb-idx]')
      expect(box.exists()).toBe(true) // the control renders; it just must not write
      await box.trigger('click')
      // Flush the SAME way as the positive control below. Without this the
      // absence would only prove the debounce had not elapsed.
      w.unmount()
      await flushPromises()
      expect(updateEntityMock).not.toHaveBeenCalled()

      // Positive control: the SAME click on the SAME markup under the default
      // world DOES schedule a write. Without this the assertion above passes
      // against a checkbox that was never wired.
      mockRoute.query = {}
      const dflt = await mountDetail({
        entry: entry({ content: '- [ ] a task' }),
        sections: [contentSection],
      })
      const dfltBox = dflt.find('input[type="checkbox"][data-cb-idx]')
      expect(dfltBox.exists()).toBe(true)
      await dfltBox.trigger('click')
      // The content channel debounces, so flush it the way the component
      // itself does on navigation — unmount triggers commitImmediately().
      dflt.unmount()
      await flushPromises()
      expect(updateEntityMock).toHaveBeenCalled()
    })

    it('renders NO inline-edit surface on a collection ROW under a world (G3)', async () => {
      // rowShouldRouteToInlineEdit is a SEPARATE predicate from the section
      // one, so the entry-level test does not cover it. Rows matter doubly:
      // under a world each row is a NEIGHBOUR's resolved face, so an edit
      // would address the default state of an entity the page is not showing.
      const cardSection = section({
        heading: 'Implements',
        sectionId: 'implements',
        display: 'cards',
        entities: [collectionEntity({
          id: 'CTL-1',
          title: 'MFA enforcement',
          editFormId: 'control-edit',
          fields: [{ property: 'title', label: 'Title', values: ['MFA'], render: 'input' }],
          _props: { title: 'MFA' },
          _fields: {},
        })],
      })
      mockRoute.query = { world: 'published' }
      const w = await mountDetail({ entry: entry(), sections: [cardSection] })
      rendersProof(w)
      expect(w.findComponent({ name: 'SectionEditForm' }).exists()).toBe(false)

      // Positive control under the default world — otherwise this passes
      // against a row that could never have routed to inline edit anyway.
      mockRoute.query = {}
      const dflt = await mountDetail({ entry: entry(), sections: [cardSection] })
      expect(dflt.findComponent({ name: 'SectionEditForm' }).exists()).toBe(true)
    })

    it('hides the per-row EDIT BUTTON under a world (G4)', async () => {
      // A THIRD surface, distinct from both the entry Edit button (gated on
      // canUpdate) and the inline SectionEditForm above (gated on
      // rowShouldRouteToInlineEdit). The server sends `edit_form_id` on every
      // section row — a config lookup that knows nothing about the request's
      // world — so without a client-side gate the entry Edit correctly
      // disappears under a world while the row buttons stay, offering an edit
      // whose save the write path would reject (RULING 11).
      const cardSection = () => ({
        heading: 'Implements',
        sectionId: 'implements',
        display: 'cards',
        isEmpty: false,
        isGrouped: false,
        hasContent: false,
        entities: [{
          id: 'CTL-1',
          type: 'control',
          title: 'MFA enforcement',
          hasContent: true,
          editFormId: 'control-edit',
          fields: [],
          _props: { title: 'MFA' },
          _fields: {},
        }],
      })

      mockRoute.query = { world: 'published' }
      const w = await mountDetail({ entry: entry(), sections: [cardSection()] } as never)
      rendersProof(w)
      expect(w.findAll('.edit-btn').length).toBe(0)

      // Positive control: the same fixture under the default world DOES render
      // the button — otherwise this passes against a row that never had one.
      mockRoute.query = {}
      const dflt = await mountDetail({ entry: entry(), sections: [cardSection()] } as never)
      expect(dflt.findAll('.edit-btn').length).toBeGreaterThan(0)
    })

    it('treats ?world=default as writable and unbannered (S2)', async () => {
      // The default world IS where writes land, so this is correct — but the
      // invariant "banner shown <=> writes blocked" rests on TWO independently
      // maintained expressions (`isWorldBound` in useWorld, and the banner's
      // own v-if). This pins them together for the one spelling where they
      // could most plausibly drift: the explicit `default`, which the API
      // accepts and which round-trips through the URL.
      mockRoute.query = { world: 'default' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(w.find('.world-banner').exists()).toBe(false)
      expect(button(w, 'Delete')).toBeDefined()
      // And the param is OMITTED rather than sent as the reserved name.
      expect(fetchViewMock).toHaveBeenCalledWith(entityType, entityId, undefined)
    })

    it('hides operator COMMANDS under a world (S1)', async () => {
      // A command pipes a rendered view to a shell script's stdin, and the
      // server passes defaultViewWorld() explicitly there — so under a world
      // the script gets DRAFT content while the user reads published. What a
      // world-bound command should mean is another ticket; rendering the
      // button beside a "read-only" banner in the meantime is not.
      getCommandsMock.mockResolvedValue([{ id: 'publish', label: 'Run publish script' }])
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(w.text()).not.toContain('Run publish script')

      // Positive control: same command list, default world, button renders.
      mockRoute.query = {}
      const dflt = await mountDetail(viewResponse())
      expect(dflt.text()).toContain('Run publish script')
    })

    it('offers the way back to the default world', async () => {
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)

      const back = w
        .findAll('button')
        .find((b) => b.text().includes('Go to default'))
      expect(back).toBeDefined()
      await back!.trigger('click')
      // Returns to the same id with the world dropped — writes land there.
      expect(routerPush).toHaveBeenCalledWith({ query: {} })
    })

    // The button NAMES A DESTINATION, so it must be labelled from the
    // destination rather than from a guess about the project's vocabulary.
    //
    // It used to render the literal "Go to draft" on every type in every
    // project. A blog post has no draft — its faces are a LANGUAGE axis with
    // no lifecycle — so the button was ISMS vocabulary applied to content that
    // has none. Both spellings below come from the SAME code path; only the
    // operator's `faces:` differs.
    describe('the way back is labelled from the destination face', () => {
      // `bare_face` names which declared face the bare id addresses; it is one
      // field on the TYPE, not a flag repeated on each face.
      function seedFaces(bareFace: string, faces: Record<string, { label?: string }>) {
        useSchemaStore().entityTypes.set(entityType, {
          name: entityType,
          label: 'Policy',
          properties: { title: { type: 'string', values: null } },
          faces,
          bare_face: bareFace,
        } as never)
      }

      it("uses the operator's label for the default face", async () => {
        seedFaces('draft', { draft: { label: 'the draft' }, published: {} })
        mockRoute.query = { world: 'published' }
        const w = await mountDetail(viewResponse())
        rendersProof(w)
        expect(w.text()).toContain('Go to the draft')
      })

      it('reads the language vocabulary on a type with no lifecycle', async () => {
        // The regression in one assertion: an unlabelled ISMS word must not
        // appear on a type whose faces are languages.
        seedFaces('en', { en: { label: 'English' }, nl: {} })
        mockRoute.query = { world: 'published' }
        const w = await mountDetail(viewResponse())
        rendersProof(w)
        expect(w.text()).toContain('Go to English')
        expect(w.text()).not.toContain('Go to draft')
      })

      it('falls back to the FACE NAME when the operator declared no label', async () => {
        // The name is itself operator-authored config, so it is an honest (if
        // terse) display string — better than the type-neutral "default".
        seedFaces('draft', { draft: {}, published: {} })
        mockRoute.query = { world: 'published' }
        const w = await mountDetail(viewResponse())
        rendersProof(w)
        expect(w.text()).toContain('Go to draft')
      })

      it('falls back to "default" when the type names no bare_face', async () => {
        // A type may declare faces and name none of them as the bare one, in
        // which case the row a bare id addresses has no declared name. There
        // is nothing to label it with, and inventing a word would be worse
        // than the jargon.
        seedFaces('', { published: {} })
        mockRoute.query = { world: 'published' }
        const w = await mountDetail(viewResponse())
        rendersProof(w)
        expect(w.text()).toContain('Go to default')
      })
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
      invokeCopyMock.mockResolvedValue(copyResult())
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      rendersProof(w)

      await clickPromote(w)

      // A same-entity copy targets the source by construction; the target id
      // is OMITTED rather than sent empty (the kernel rejects a target on a
      // same-entity copy).
      expect(invokeCopyMock).toHaveBeenCalledWith('promote-policy', entityId)
    })

    it('reloads after a copy so the offers recompute', async () => {
      invokeCopyMock.mockResolvedValue(copyResult())
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      const before = fetchViewMock.mock.calls.length

      await clickPromote(w)

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
      await clickPromote(w)

      await w.setProps({ entityId: 'POL-2' })
      await flushPromises()
      const afterNav = fetchViewMock.mock.calls.length

      resolveInvoke(copyResult())
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

      await clickPromote(w)

      // `allowed` is a hint; the kernel re-authorizes. A 403 here is the
      // boundary working, and the page must not reload as if it succeeded.
      expect(fetchViewMock.mock.calls.length).toBe(before)
    })
  })

  describe('per-neighbour provenance', () => {
    const world = (via: EntityWorld['via'], face: string): EntityWorld => ({
      name: 'site-nl',
      face,
      via,
    })

    function withCollection(entities: ViewEntity[]): ViewResponse {
      return {
        entry: entry(),
        sections: [section({ heading: 'Implements', sectionId: 'implements', display: 'list', entities })],
      }
    }

    // TKT-6NCSSC: the neighbour link used to drop the world, so following a row
    // from a world-bound page landed on the DEFAULT face — of an entity whose
    // row the reader had just seen badged as a fallback.
    it('carries the world on a neighbour link', async () => {
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail(
        withCollection([
          { id: 'CTL-1', type: 'control', title: 'Real Dutch', hasContent: false, _world: world('chain', 'nl') },
        ]),
      )
      rendersProof(w)

      const link = w.findAll('a').find((a) => a.text().includes('Real Dutch'))
      expect(link).toBeDefined()
      const href = link!.attributes('href') ?? ''
      expect(href).toContain('/entity/control/CTL-1')
      expect(href).toContain('world=site-nl')
    })

    it('adds no world param under the default world', async () => {
      const w = await mountDetail(
        withCollection([
          { id: 'CTL-1', type: 'control', title: 'Plain', hasContent: false },
        ]),
      )
      rendersProof(w)

      const link = w.findAll('a').find((a) => a.text().includes('Plain'))
      expect(link).toBeDefined()
      const href = link!.attributes('href') ?? ''
      expect(href).toContain('/entity/control/CTL-1')
      expect(href).not.toContain('world=')
    })

    it('distinguishes a real face from a fallback in the SAME section', async () => {
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail(
        withCollection([
          collectionEntity({ id: 'CTL-1', title: 'Real Dutch', _world: world('chain', 'nl') }),
          collectionEntity({ id: 'CTL-2', title: 'Fallback', _world: world('fallback-default', '') }),
        ]),
      )
      rendersProof(w)

      // Per-neighbour: each entity resolved independently, so one section can
      // carry both kinds. This is the whole point of RULING 12 — but the
      // badge is an EXCEPTION marker, so only the stand-in renders one. The
      // Dutch neighbour got the face site-nl asked for and stays unmarked.
      const badges = w.findAll('.world-badge')
      expect(badges).toHaveLength(1)
      expect(badges[0].classes()).toContain('is-fallback')
      expect(badges[0].text()).toBe('default')
      // Anti-vacuity: both neighbours really rendered, so the single badge is
      // a statement about provenance and not about a section that came up
      // empty.
      expect(w.text()).toContain('Real Dutch')
      expect(w.text()).toContain('Fallback')
    })

    it('renders no badge under the default world', async () => {
      const w = await mountDetail(
        withCollection([
          collectionEntity({ id: 'CTL-1', title: 'Plain' }),
        ]),
      )
      rendersProof(w)
      expect(w.find('.world-badge').exists()).toBe(false)
    })
  })

  // BUG-1: a draft with no face in the requested world used to render
  // "Error — entry entity not found", which is how the demo ended up with
  // duplicate policies: the natural response to "not found" is to create it
  // again. The server now answers 200 + `_world_absent` with the DEFAULT face.
  describe('no face in this world (BUG-1)', () => {
    function absentResponse(): ViewResponse {
      return {
        entry: entry(),
        sections: [],
        _world_absent: true,
        _world_absent_name: 'published',
      }
    }

    it('renders the entity instead of a terminal error', async () => {
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(absentResponse())
      // The positive control: the page really rendered the entity, so the
      // absence of an error below is a statement about the page.
      rendersProof(w)
      expect(w.find('.error-state').exists()).toBe(false)
    })

    it('names the world that has no face for it', async () => {
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(absentResponse())
      const banner = w.find('.world-banner--absent')
      expect(banner.exists()).toBe(true)
      expect(banner.text()).toContain('published')
    })

    it('does NOT claim the page is read-only', async () => {
      // What is on screen is the DEFAULT face, which is exactly where writes
      // land. Showing the ordinary world banner's "This face is read-only"
      // beside it would be false, and would discourage the very edit the user
      // came to make.
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(absentResponse())
      rendersProof(w)
      // The banner's actual text — an earlier version of this assertion
      // checked a phrase the banner had stopped using, and passed against
      // every build.
      expect(w.text()).not.toContain('Read-only in this world')
    })

    it('offers Edit and Delete, because the default face IS what is on screen', async () => {
      // The banner says "the default face, which is where edits are saved";
      // every write guard used to key on `isWorldBound` alone and hid them
      // anyway. The seam is `readOnly = isWorldBound && !worldAbsent`.
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(absentResponse())
      rendersProof(w)
      expect(button(w, 'Edit')).toBeDefined()
      expect(button(w, 'Delete')).toBeDefined()
    })

    it('still shows the read-only banner for an ordinary world-bound page', async () => {
      // The discriminating half: without this, the assertion above would pass
      // against a build that simply deleted the read-only banner.
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(w.text()).toContain('Read-only in this world')
      expect(w.find('.world-banner--absent').exists()).toBe(false)
    })
  })

  // BUG-2: History is PER-FACE, so navigating to it must carry the world or
  // the reader is shown the default face's history — a genuinely different
  // record, presented as the right one.
  describe('history carries the world (BUG-2)', () => {
    it('pushes the world with the history route', async () => {
      // History is gated on the backend HAVING history (postgres-only), so the
      // button must be enabled for this test to have a button to click.
      useSchemaStore().historyEnabled = true
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      const historyBtn = button(w, 'History')
      expect(historyBtn).toBeDefined()
      // History is a real link now (TKT-3CSZRG), so the destination lives in
      // its `to` rather than in a router.push call. The contract under test is
      // unchanged: the world must ride along, or the reader lands on the
      // DEFAULT face's history from a world-bound page (BUG-2).
      expect(historyBtn!.attributes('to') ?? historyBtn!.attributes('href')).toContain(
        `/history/${entityType}/${entityId}`,
      )
      expect(historyBtn!.attributes('to') ?? historyBtn!.attributes('href')).toContain(
        'world=published',
      )
    })

    it('pushes no world param in the default world', async () => {
      useSchemaStore().historyEnabled = true
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      const target =
        button(w, 'History')!.attributes('to') ?? button(w, 'History')!.attributes('href')
      expect(target).toContain(`/history/${entityType}/${entityId}`)
      expect(target).not.toContain('world=')
    })
  })

  // Every header affordance that is not Edit or Delete must ALSO have a home
  // in the mobile overflow menu.
  //
  // Three features shipped to the desktop row alone (FaceMenu, CopyMenu,
  // History), so at phone width they vanished entirely — not degraded layout,
  // LOST FUNCTIONALITY: no way to publish a policy or switch language, with
  // nothing on screen saying those actions exist. Both blocks are always in
  // the DOM (CSS picks one per breakpoint), so these assertions scope to
  // `.mobile-actions` rather than to the whole render, which would pass on the
  // desktop copy alone.
  describe('the mobile overflow menu is a home for every header affordance', () => {
    async function openOverflow(view: ViewResponse) {
      const w = await mountDetail(view)
      rendersProof(w)
      const toggle = w.find('.mobile-actions .mobile-overflow-btn')
      expect(toggle.exists()).toBe(true)
      await toggle.trigger('click')
      return w
    }

    function overflowText(w: VueWrapper) {
      return w.find('.mobile-actions .overflow-menu').text()
    }

    it('offers COPIES, which used to be desktop-only', async () => {
      const w = await openOverflow(viewResponse({ _copies: [promoteOffer()] }))
      expect(overflowText(w)).toContain('Publish this policy')
    })

    it('invokes a copy from the overflow with the same arguments as the desktop menu', async () => {
      invokeCopyMock.mockResolvedValue({
        definition: 'promote-policy', entityId, face: 'published', created: true,
      })
      const w = await openOverflow(viewResponse({ _copies: [promoteOffer()] }))
      const btn = w
        .findAll('.mobile-actions .overflow-menu-item')
        .find((b) => b.text().includes('Publish this policy'))
      expect(btn).toBeDefined()
      await btn!.trigger('click')
      await flushPromises()
      // Same handler, so a copy invoked from a phone goes through the
      // identical guard rather than a parallel path.
      expect(invokeCopyMock).toHaveBeenCalledWith('promote-policy', entityId)
    })

    it('does NOT offer a denied copy, matching the desktop rule', async () => {
      // A denied copy is ABSENT, not disabled — and the mobile list must not
      // be the place that quietly reintroduces it.
      //
      // History is enabled here purely so the overflow menu EXISTS to be
      // inspected: with a denied copy as the only candidate, `hasOverflow` is
      // correctly false and the assertion below would pass against a menu that
      // was never rendered.
      useSchemaStore().historyEnabled = true
      const w = await openOverflow(
        viewResponse({ _copies: [promoteOffer({ allowed: false, reason: 'nope' })] }),
      )
      expect(overflowText(w)).toContain('History')
      expect(overflowText(w)).not.toContain('Publish this policy')
    })

    it('offers FACES, which used to be desktop-only', async () => {
      useSchemaStore().worlds.set('published', { readable: true, select: ['published'] } as never)
      const w = await openOverflow(
        viewResponse({ _faces: [{ face: 'published', label: 'Published' }] }),
      )
      expect(overflowText(w)).toContain('View Published')
    })

    it('renders no overflow button when there is nothing to put in it', async () => {
      // The paired positive control for every absence above: without this,
      // an overflow that never rendered would satisfy the denied-copy test.
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(w.find('.mobile-actions .mobile-overflow-btn').exists()).toBe(false)
    })
  })

  // History is a per-DEPLOYMENT capability, not a permission: version history
  // is postgres-only, and `/_history` answers fs/mem with a named 501. An
  // ungated button could therefore only fail — the affordance-that-lies shape
  // this app refuses elsewhere.
  //
  // Asserted in BOTH blocks. That pairing is the point: this component has
  // already shipped an affordance to one render site and not the other three
  // times, so a test that checked only the desktop copy would let the mobile
  // one keep lying.
  describe('History is gated on the backend capability, in both blocks', () => {
    it('renders in neither block when the deployment has no history', async () => {
      useSchemaStore().historyEnabled = false
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      rendersProof(w)
      expect(w.find('.desktop-actions').text()).not.toContain('History')
      // The overflow is open-able here (the copy offer keeps it rendered), so
      // this is a real absence rather than a menu that never existed.
      await w.find('.mobile-actions .mobile-overflow-btn').trigger('click')
      expect(w.find('.mobile-actions .overflow-menu').text()).not.toContain('History')
    })

    it('renders in BOTH blocks when the deployment has history', async () => {
      useSchemaStore().historyEnabled = true
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(w.find('.desktop-actions').text()).toContain('History')
      await w.find('.mobile-actions .mobile-overflow-btn').trigger('click')
      expect(w.find('.mobile-actions .overflow-menu').text()).toContain('History')
    })
  })

  // Switching face goes through setWorld, not a bare router.push, and these
  // are the two things the bare push got wrong at once.
  describe('switching face preserves the rest of the URL', () => {
    beforeEach(() => {
      useSchemaStore().worlds.set('site-nl', { readable: true, select: ['nl'] } as never)
    })

    it('keeps unrelated query params', async () => {
      // The old `{ query: { world: w } }` REPLACED the whole query object, so
      // switching language from a list-scoped page silently dropped
      // `from`/`scope` and broke the back button and prev/next navigation.
      mockRoute.query = { from: 'posts', scope: 'list:posts' }
      const w = await mountDetail(viewResponse({ _faces: [{ face: 'nl', label: 'Nederlands' }] }))
      rendersProof(w)
      const btn = w.findAll('button').find((b) => b.text().includes('View Nederlands'))
      expect(btn).toBeDefined()
      await btn!.trigger('click')
      expect(routerPush).toHaveBeenCalledWith({
        query: { from: 'posts', scope: 'list:posts', world: 'site-nl' },
      })
    })

    it('spells the default face explicitly when a default world is configured', async () => {
      // `worldForFace` returns '' for the default face, but on a deployment
      // with `default_world: published` an absent param means PUBLISHED, not
      // the default face — so DROPPING the param would navigate somewhere the
      // user did not ask for. setWorld owns that rule; goToFace must not
      // reimplement it.
      const store = useSchemaStore()
      store.defaultWorld = 'published'
      mockRoute.query = { world: 'site-nl' }
      // The server's `_faces` already excludes the served face, so the
      // response carries the OTHER face only — the `_views` entry never
      // carries `_world`, and the page must not depend on it.
      const w = await mountDetail(viewResponse({
        _faces: [{ face: '', label: 'English' }],
      }))
      rendersProof(w)
      const btn = w.findAll('button').find((b) => b.text().includes('View English'))
      expect(btn).toBeDefined()
      await btn!.trigger('click')
      expect(routerPush).toHaveBeenCalledWith({ query: { world: 'default' } })
    })

    it('DROPS the param for the default face when no default world is configured', async () => {
      // The cosmetic half: '' and 'default' name the same world, so returning
      // to it should drop the param rather than write the noisier explicit
      // spelling.
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail(viewResponse({
        _faces: [{ face: '', label: 'English' }],
      }))
      rendersProof(w)
      const btn = w.findAll('button').find((b) => b.text().includes('View English'))
      expect(btn).toBeDefined()
      await btn!.trigger('click')
      expect(routerPush).toHaveBeenCalledWith({ query: {} })
    })
  })
})
