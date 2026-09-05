import { describe, it, expect, vi, beforeEach, afterEach, type MockInstance } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
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
// The confirm dialog is a host-bound singleton (App.vue); here it records
// what it was asked and answers no, so a delete test can assert the WORDING
// without a write leaving the client.
type ConfirmOpts = { title: string; message: string }
const confirmMock = vi.fn<(opts: ConfirmOpts) => Promise<boolean>>(async () => false)
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: (opts: ConfirmOpts) => confirmMock(opts) }),
  withConfirmError: (fn: unknown) => fn,
}))

const routerPush = vi.fn()
// The success toast, spied per test so a copy's wording can be asserted.
let successMock: MockInstance
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
    confirmMock.mockReset().mockResolvedValue(false)
    routerPush.mockClear()
    mockRoute.query = {}
    successMock = vi.spyOn(useUIStore(), 'success')
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
      // No banner: this fixture declares no `banner:` for the world, and the
      // served row is the BARE face (no `_self` face) with `update: true`, so
      // there is nothing to announce and nothing to explain. A world no
      // longer makes a page read-only; only a grant does.
      expect(w.find('.world-banner').exists()).toBe(false)
    })

    it('renders the operator announcement alone when the world declares one', async () => {
      useSchemaStore().worlds.set('published', { readable: true, banner: 'In force' } as never)
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(w.find('.world-banner__label').text()).toBe('In force')
      expect(w.find('.world-banner').text()).not.toContain('read-only')
    })
  })

  // The address rule: what you look at is what you edit is what you save.
  //
  //     view[entity@face]  --Edit-->  form[entity@face]  --Save-->  PATCH entity@face
  //
  // Every write goes to the row's `_self` address, and whether it is allowed
  // is `_actions`, which the server computes for that same face. The page no
  // longer ANDs the world in: that lock re-derived a decision the server had
  // made and got it wrong for every unfaced type and every chain hit on the
  // bare face (atlas worlds issues 2, 3, 4, 10).
  describe('writes follow _actions and the address', () => {
    // A row served at a NON-bare face this principal may not write — the ISMS
    // "adopted text" case.
    const standIn = () => entry({
      _self: '/api/v1/policys/POL-1@published',
      _actions: { update: false, delete: false, rename: false },
    })
    // A NON-bare face this principal MAY write — the translator on `nl`.
    const writableFace = (over: Partial<Entity> = {}) => entry({
      _self: '/api/v1/policys/POL-1@nl',
      _actions: { update: true, delete: true, rename: true },
      ...over,
    })

    async function pressE(w: VueWrapper) {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'e', bubbles: true }))
      await flushPromises()
      w.unmount()
    }

    it('offers Edit and Delete under the default world', async () => {
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(button(w, 'Edit')).toBeDefined()
      expect(button(w, 'Delete')).toBeDefined()
    })

    it('keeps Edit and Delete under a world when _actions permits them', async () => {
      // The bare face served through a chain (`select: [published, draft]`
      // with no published face): the server says writable, and it is.
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(button(w, 'Edit')).toBeDefined()
      expect(button(w, 'Delete')).toBeDefined()
      expect(w.find('.world-banner').exists()).toBe(false)
    })

    // The words for a read-only non-bare face are the OPERATOR's
    // (`faces.<name>.messages.read_only`); the app has none of its own. A
    // page with nothing declared shows a denial the way every denial looks:
    // no Edit, no explanation (TKT-5SZG2L).
    function seedReadOnlyText(text: string) {
      useSchemaStore().entityTypes.set(entityType, {
        name: entityType,
        label: 'Policy',
        properties: { title: { type: 'string', values: null } },
        faces: { draft: { label: 'Concept' }, published: { label: 'Vastgesteld', messages: { read_only: text } } },
        bare_face: 'draft',
      } as never)
    }

    it('hides Edit and Delete for a stand-in face _actions denies, and says nothing by default', async () => {
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse(standIn()))
      rendersProof(w)
      expect(button(w, 'Edit')).toBeUndefined()
      expect(w.find('.btn-danger').exists()).toBe(false)
      expect(w.find('.world-banner').exists()).toBe(false)
    })

    it("explains a read-only face in the operator's words, placeholders substituted", async () => {
      seedReadOnlyText('Dit is {face} van {title}. Bewerken doe je in {bare_face}.')
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse(standIn()))
      rendersProof(w)
      const banner = w.find('.world-banner')
      expect(banner.exists()).toBe(true)
      expect(banner.text()).toBe('Dit is Vastgesteld van Access Control Policy. Bewerken doe je in Concept.')
      // No button: the face menu is the way to the bare face (issue 5).
      expect(banner.find('button').exists()).toBe(false)
    })

    it('explains the stand-in even without a world in the URL', async () => {
      // `/entity/policy/POL-1@published?world=default` reaches the same row
      // by address; the note is about the face, not the world.
      seedReadOnlyText('Alleen lezen')
      mockRoute.query = { world: 'default' }
      const w = await mountDetail(viewResponse(standIn()))
      rendersProof(w)
      expect(w.find('.world-banner').text()).toBe('Alleen lezen')
    })

    it('gives a bare face without update no note — an ordinary denial', async () => {
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse(entry({ _actions: { update: false, delete: false } })))
      rendersProof(w)
      expect(button(w, 'Edit')).toBeUndefined()
      expect(w.find('.world-banner').exists()).toBe(false)
    })

    it('opens the form on the served ADDRESS, face included', async () => {
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail(viewResponse(writableFace()))
      rendersProof(w)
      expect(button(w, 'Edit')).toBeDefined()
      await pressE(w)
      expect(routerPush).toHaveBeenCalledWith({
        name: 'form-edit', params: { id: 'policy-edit', entityId: 'POL-1@nl' },
      })
    })

    it('opens the form on the bare id when the bare face is on screen', async () => {
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      await pressE(w)
      expect(routerPush).toHaveBeenCalledWith({
        name: 'form-edit', params: { id: 'policy-edit', entityId: 'POL-1' },
      })
    })

    it('writes a checkbox toggle to the served ADDRESS', async () => {
      const contentSection = section({
        sectionId: 'content', display: 'content', hasContent: true, content: '- [ ] a task',
      })
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail({
        entry: writableFace({ content: '- [ ] a task' }),
        sections: [contentSection],
      })
      rendersProof(w)
      const box = w.find('input[type="checkbox"][data-cb-idx]')
      expect(box.exists()).toBe(true)
      await box.trigger('click')
      // The content channel debounces, so flush it the way the component
      // itself does on navigation — unmount triggers commitImmediately().
      w.unmount()
      await flushPromises()
      expect(updateEntityMock).toHaveBeenCalled()
      expect(updateEntityMock.mock.calls[0][1]).toBe('POL-1@nl')
    })

    it('refuses a checkbox toggle when _actions denies update', async () => {
      const contentSection = section({
        sectionId: 'content', display: 'content', hasContent: true, content: '- [ ] a task',
      })
      mockRoute.query = { world: 'published' }
      const w = await mountDetail({
        entry: { ...standIn(), content: '- [ ] a task' },
        sections: [contentSection],
      })
      rendersProof(w)
      const box = w.find('input[type="checkbox"][data-cb-idx]')
      expect(box.exists()).toBe(true)
      await box.trigger('click')
      w.unmount()
      await flushPromises()
      expect(updateEntityMock).not.toHaveBeenCalled()
    })

    it('renders the inline-edit surface under a world, addressed to the served row', async () => {
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail({
        entry: writableFace(),
        sections: [section({
          heading: 'Properties',
          sectionId: 'props',
          display: 'properties',
          fields: [{ property: 'title', label: 'Title', values: ['Access Control Policy'], render: 'input' }],
        })],
      })
      rendersProof(w)
      const form = w.findComponent({ name: 'SectionEditForm' })
      expect(form.exists()).toBe(true)
      expect(form.props('entityId')).toBe('POL-1@nl')
    })

    it('addresses a collection ROW by its own _self', async () => {
      // Under a world each row is a NEIGHBOUR's resolved face; its inline
      // form and Edit button write to that face, never to the bare id.
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
          _self: '/api/v1/controls/CTL-1@published',
        })],
      })
      mockRoute.query = { world: 'published' }
      const w = await mountDetail({ entry: entry(), sections: [cardSection] })
      rendersProof(w)
      const form = w.findComponent({ name: 'SectionEditForm' })
      expect(form.exists()).toBe(true)
      expect(form.props('entityId')).toBe('CTL-1@published')
      const edit = w.find('.edit-btn')
      expect(edit.exists()).toBe(true)
      await edit.trigger('click')
      expect(routerPush).toHaveBeenCalledWith({
        name: 'form-edit', params: { id: 'control-edit', entityId: 'CTL-1@published' },
      })
    })

    it('treats ?world=default as writable and unbannered (S2)', async () => {
      mockRoute.query = { world: 'default' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(w.find('.world-banner').exists()).toBe(false)
      expect(button(w, 'Delete')).toBeDefined()
      // And the param is OMITTED rather than sent as the reserved name.
      expect(fetchViewMock).toHaveBeenCalledWith(entityType, entityId, undefined)
    })

    it('hides operator COMMANDS while a NON-bare face is on screen (S1)', async () => {
      // A command pipes a rendered view to a shell script's stdin, and the
      // server passes defaultViewWorld() explicitly there — so with the
      // published face on screen the script gets the BARE face's content.
      getCommandsMock.mockResolvedValue([{ id: 'publish', label: 'Run publish script' }])
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse(standIn()))
      rendersProof(w)
      expect(w.text()).not.toContain('Run publish script')

      // The bare face under the SAME world is what the script gets, so the
      // button renders — the world is not what gates it.
      const bare = await mountDetail(viewResponse())
      expect(bare.text()).toContain('Run publish script')
    })

    it('deletes by the served ADDRESS and names the face', async () => {
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail(viewResponse(writableFace()))
      rendersProof(w)
      const del = button(w, 'Delete')
      expect(del).toBeDefined()
      await del!.trigger('click')
      await flushPromises()
      expect(confirmMock).toHaveBeenCalled()
      const opts = confirmMock.mock.calls[0][0]
      expect(opts.title).toBe('Delete Face?')
      expect(opts.message).toContain('nl face')
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

    it('keeps the offers the server made under a world', async () => {
      // The offers are computed for the face SERVED and the invoke names the
      // source by id, so a promote offered on the bare face copies the bytes
      // on screen. Blanking them under every world hid a correctly-offered
      // promote on a chain hit (atlas worlds issue 4).
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      rendersProof(w)
      expect(w.text()).toContain('Publish this policy')
    })

    it('invokes by NAME with the source id, and no target for a same-entity copy', async () => {
      invokeCopyMock.mockResolvedValue(copyResult())
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      rendersProof(w)

      await clickPromote(w)

      expect(invokeCopyMock).toHaveBeenCalledWith('promote-policy', entityId)
    })

    it('lands on the face it wrote, and the toast is the copy\'s own label', async () => {
      // With no `on_success` declared: the written face is where an editor
      // who just adopted a policy expects to be, and the toast says only what
      // the button said — never rela's "Created the X face" (TKT-5SZG2L).
      invokeCopyMock.mockResolvedValue(copyResult())
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      await clickPromote(w)
      expect(routerPush).toHaveBeenCalledWith({
        path: '/entity/policy/POL-1@published', query: { world: 'published' },
      })
      expect(successMock).toHaveBeenCalledWith('Publish this policy')
    })

    it("toasts the operator's on_success.message with {face} as the face written", async () => {
      useSchemaStore().entityTypes.set(entityType, {
        name: entityType,
        label: 'Policy',
        properties: { title: { type: 'string', values: null } },
        faces: { draft: {}, published: { label: 'Vastgesteld' } },
        bare_face: 'draft',
      } as never)
      invokeCopyMock.mockResolvedValue(copyResult())
      const w = await mountDetail(viewResponse({
        _copies: [promoteOffer({ onSuccess: { message: '{title} is nu {face}.', landing: { mode: 'written' } } })],
      }))
      await clickPromote(w)
      expect(successMock).toHaveBeenCalledWith('Access Control Policy is nu Vastgesteld.')
    })

    it('stays in place when landing is `stay`', async () => {
      invokeCopyMock.mockResolvedValue(copyResult())
      const w = await mountDetail(viewResponse({
        _copies: [promoteOffer({ onSuccess: { landing: { mode: 'stay' } } })],
      }))
      const before = fetchViewMock.mock.calls.length
      await clickPromote(w)
      expect(routerPush).not.toHaveBeenCalled()
      expect(fetchViewMock.mock.calls.length).toBeGreaterThan(before)
    })

    it('lands in a declared world when landing names one', async () => {
      invokeCopyMock.mockResolvedValue(copyResult())
      mockRoute.query = { from: 'posts' }
      const w = await mountDetail(viewResponse({
        _copies: [promoteOffer({ onSuccess: { landing: { mode: 'world', world: 'published' } } })],
      }))
      await clickPromote(w)
      // The bare id in that world, with the rest of the query kept.
      expect(routerPush).toHaveBeenCalledWith({
        path: '/entity/policy/POL-1', query: { from: 'posts', world: 'published' },
      })
    })

    it('lands on a declared face when landing names one', async () => {
      invokeCopyMock.mockResolvedValue(copyResult())
      mockRoute.query = { world: 'editorial' }
      const w = await mountDetail(viewResponse({
        _copies: [promoteOffer({ onSuccess: { landing: { mode: 'face', face: 'draft' } } })],
      }))
      await clickPromote(w)
      expect(routerPush).toHaveBeenCalledWith({
        path: '/entity/policy/POL-1@draft', query: { world: 'editorial' },
      })
    })

    it('reloads in place after a copy INTO the bare face', async () => {
      // A bare address would re-resolve under a world, so a revert stays put
      // and the offers recompute from the reload.
      invokeCopyMock.mockResolvedValue({ ...copyResult(), face: '' })
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      const before = fetchViewMock.mock.calls.length
      await clickPromote(w)
      expect(routerPush).not.toHaveBeenCalled()
      expect(fetchViewMock.mock.calls.length).toBeGreaterThan(before)
    })

    it('does not reload or misattribute when the entity changes mid-invoke', async () => {
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
      // ...and it neither reloaded nor navigated the unrelated entity now on screen.
      expect(fetchViewMock.mock.calls.length).toBe(afterNav)
      expect(routerPush).not.toHaveBeenCalledWith(expect.objectContaining({ path: expect.stringContaining('POL-1@') }))
    })

    it('surfaces a refusal rather than pretending the copy ran', async () => {
      invokeCopyMock.mockRejectedValue(new Error('forbidden'))
      const w = await mountDetail(viewResponse({ _copies: [promoteOffer()] }))
      const before = fetchViewMock.mock.calls.length

      await clickPromote(w)

      expect(fetchViewMock.mock.calls.length).toBe(before)
      expect(routerPush).not.toHaveBeenCalled()
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
      useSchemaStore().worlds.set('site-nl', { readable: true, messages: { stand_in: 'vervangend' } } as never)
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
      expect(badges[0].text()).toBe('vervangend')
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
      rendersProof(w)
      expect(w.find('.error-state').exists()).toBe(false)
    })

    it('says nothing about the absence unless the operator declared text', async () => {
      // The app has no sentence of its own for "no face in this world"; a
      // page with nothing declared is simply the bare face (TKT-5SZG2L).
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(absentResponse())
      rendersProof(w)
      expect(w.find('.world-banner').exists()).toBe(false)
      expect(w.text()).not.toContain('Go to')
    })

    it("renders the world's messages.absent in the operator's words", async () => {
      useSchemaStore().worlds.set('published', {
        readable: true, messages: { absent: 'Nog niet vastgesteld: {title}.' },
      } as never)
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(absentResponse())
      rendersProof(w)
      const banner = w.find('.world-banner--absent')
      expect(banner.exists()).toBe(true)
      expect(banner.text()).toBe('Nog niet vastgesteld: Access Control Policy.')
      expect(banner.find('button').exists()).toBe(false)
    })

    it('offers Edit and Delete, because the default face IS what is on screen', async () => {
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(absentResponse())
      rendersProof(w)
      expect(button(w, 'Edit')).toBeDefined()
      expect(button(w, 'Delete')).toBeDefined()
    })

    it('redirects to the declared world when on_absent.redirect names one', async () => {
      // The operator would rather send the reader to the concept than
      // explain an absence. `default` is spelled through setWorld, so the
      // param is dropped here (no configured default world).
      useSchemaStore().worlds.set('published', {
        readable: true, on_absent: { redirect: 'default' },
      } as never)
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(absentResponse())
      rendersProof(w)
      expect(routerPush).toHaveBeenCalledWith({ query: {} })
    })

    it('redirects to a non-default world by name', async () => {
      useSchemaStore().worlds.set('published', {
        readable: true, on_absent: { redirect: 'editorial' },
      } as never)
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(absentResponse())
      rendersProof(w)
      expect(routerPush).toHaveBeenCalledWith({ query: { world: 'editorial' } })
    })

    it('does not redirect when the page is not absent', async () => {
      useSchemaStore().worlds.set('published', {
        readable: true, on_absent: { redirect: 'default' },
      } as never)
      mockRoute.query = { world: 'published' }
      const w = await mountDetail(viewResponse())
      rendersProof(w)
      expect(routerPush).not.toHaveBeenCalled()
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

  // Switching face is a plain link to the face's ADDRESS (`_faces[].ref`),
  // which the server serves literally under any world — so the menu never
  // works out which declared world leads with a face, and never switches
  // world to reach one (atlas worlds issue 5).
  describe('switching face navigates to the face address', () => {
    it('keeps the world and the rest of the query', async () => {
      mockRoute.query = { from: 'posts', scope: 'list:posts', world: 'site-nl' }
      const w = await mountDetail(viewResponse({
        _faces: [{ face: 'nl', label: 'Nederlands', ref: 'POL-1@nl' }],
      }))
      rendersProof(w)
      const btn = w.findAll('button').find((b) => b.text().includes('View Nederlands'))
      expect(btn).toBeDefined()
      await btn!.trigger('click')
      expect(routerPush).toHaveBeenCalledWith({
        path: '/entity/policy/POL-1@nl',
        query: { from: 'posts', scope: 'list:posts', world: 'site-nl' },
      })
    })

    it('reaches the bare face by its explicit address, without switching world', async () => {
      // `POL-1@en` is literal under `site-nl`; the reader stays where they
      // were browsing.
      useSchemaStore().defaultWorld = 'published'
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail(viewResponse({
        _faces: [{ face: '', label: 'English', ref: 'POL-1@en' }],
      }))
      rendersProof(w)
      const btn = w.findAll('button').find((b) => b.text().includes('View English'))
      expect(btn).toBeDefined()
      await btn!.trigger('click')
      expect(routerPush).toHaveBeenCalledWith({
        path: '/entity/policy/POL-1@en', query: { world: 'site-nl' },
      })
    })

    it('names the default world for a bare face that has NO explicit address', async () => {
      // A type with faces but no `bare_face` name: the bare row is literal
      // only in the default world, spelled `default` when a configured
      // default would otherwise apply.
      useSchemaStore().defaultWorld = 'published'
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail(viewResponse({
        _faces: [{ face: '', label: 'English', ref: 'POL-1' }],
      }))
      rendersProof(w)
      const btn = w.findAll('button').find((b) => b.text().includes('View English'))
      await btn!.trigger('click')
      expect(routerPush).toHaveBeenCalledWith({
        path: '/entity/policy/POL-1', query: { world: 'default' },
      })
    })

    it('DROPS the param for such a bare face when no default world is configured', async () => {
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail(viewResponse({
        _faces: [{ face: '', label: 'English', ref: 'POL-1' }],
      }))
      rendersProof(w)
      const btn = w.findAll('button').find((b) => b.text().includes('View English'))
      await btn!.trigger('click')
      expect(routerPush).toHaveBeenCalledWith({ path: '/entity/policy/POL-1', query: {} })
    })

    it('spells a non-bare address itself for an older server that sends no ref', async () => {
      mockRoute.query = { world: 'site-nl' }
      const w = await mountDetail(viewResponse({ _faces: [{ face: 'nl', label: 'Nederlands' }] }))
      rendersProof(w)
      const btn = w.findAll('button').find((b) => b.text().includes('View Nederlands'))
      await btn!.trigger('click')
      expect(routerPush).toHaveBeenCalledWith({
        path: '/entity/policy/POL-1@nl', query: { world: 'site-nl' },
      })
    })
  })
})
