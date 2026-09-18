// Embedded-mode tests for DynamicForm (TKT-OMUD56).
//
// Inline creation mounts a SECOND DynamicForm inside a modal over the first.
// Everything DynamicForm does at page scope is therefore a hazard: a
// `router.push` on create would unmount the host form and destroy the draft
// this feature exists to protect; a document-level Cmd+Enter listener would
// have both forms act on one keypress; a second `onBeforeRouteLeave` would
// call the SINGLETON confirm, whose one answer is returned to both callers.
//
// `embedded` disables each of those. These tests pin that it does — and that
// the default (page) behaviour is untouched, which is what lets the existing
// DynamicForm.test.ts / .guard.test.ts pass unchanged.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import DynamicForm from './DynamicForm.vue'
import type { Entity } from '@/types'

const push = vi.fn()
const replace = vi.fn()
const onBeforeRouteLeave = vi.fn()

vi.mock('vue-router', () => ({
  useRouter: () => ({ push, replace, back: vi.fn() }),
  // A non-empty query is what a nested form would WRONGLY inherit from the
  // host page: these params belong to whatever route is behind the modal.
  useRoute: () => ({
    query: { 'prop.title': 'from-host-url', return_to: '/somewhere' },
    params: {},
    path: '/form/ticket-form',
  }),
  onBeforeRouteLeave: (...args: unknown[]) => onBeforeRouteLeave(...args),
}))

// Create mode issues real network calls the edit-mode suite never reaches:
// a templates fetch and the staged-affordance dry-run POST (awaited on mount,
// then debounced per keystroke). Both must be stubbed or the form never leaves
// its loading state.
vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return {
    ...actual,
    getTemplates: vi.fn().mockResolvedValue([]),
    // Echo the candidate's properties back, as the real endpoint does: it
    // returns the STRIPPED candidate, and that echo is what populates
    // `stagedVisibleProps`. Returning an empty object would mark every field
    // policy-hidden and unrender the form.
    dryRunCreateEntity: vi.fn().mockImplementation(async (_type: string, body: unknown) => ({
      properties: (body as { properties?: Record<string, unknown> })?.properties ?? {},
      _fields: {},
      _relations: {},
      warnings: [],
    })),
    createRelation: vi.fn().mockResolvedValue(undefined),
  }
})

const ENTITY_TYPE = {
  name: 'ticket',
  label: 'Ticket',
  id_type: 'short',
  properties: { title: { type: 'string' } },
}

const FORM = {
  id: 'ticket-form',
  entity: 'ticket',
  fields: [{ property: 'title', label: 'Title' }],
}

const CREATED: Entity = { id: 'TKT-9', type: 'ticket', properties: { title: 'x' }, warnings: [] }

// BUG-2OXEW0: unmount every component, or its in-flight async work logs
// after the file finishes and races vitest's worker teardown.
const mounted: VueWrapper[] = []

afterEach(() => {
  // splice first: if one unmount throws, the rest still tear down and the
  // array cannot leak into the next test.
  const wrappers = mounted.splice(0)
  wrappers.forEach((w) => {
    try {
      w.unmount()
    } catch {
      /* already torn down */
    }
  })
})

async function mountCreate(
  props: {
    embedded?: boolean
    embeddedLink?: { relation: string; peer: string; linkAs: 'from' | 'to' }
    embeddedTemplate?: string
    embeddedWorld?: string
  } = {}
) {
  const schema = useSchemaStore()
  schema.forms.set(FORM.id, FORM as never)
  schema.entityTypes.set('ticket', ENTITY_TYPE as never)
  schema.loaded = true

  const entities = useEntitiesStore()
  const create = vi.spyOn(entities, 'create').mockResolvedValue(CREATED)

  const wrapper = mount(DynamicForm, {
    props: { formId: FORM.id, ...props },
    global: {
      stubs: {
        RouterLink: true,
        MarkdownEditor: true,
        RelationPicker: true,
        RelationCards: true,
        AutoSaveIndicator: true,
        HelpModal: true,
      },
    },
  })
  mounted.push(wrapper)
  await flushPromises()
  return { wrapper, create }
}

describe('DynamicForm — embedded mode', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('emits the created entity instead of navigating', async () => {
    // The core of AC6: navigation is what would destroy the host's draft.
    const { wrapper } = await mountCreate({ embedded: true })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.emitted('inline-created')?.[0]?.[0]).toEqual(CREATED)
    expect(push).not.toHaveBeenCalled()
  })

  it('still navigates when not embedded', async () => {
    // The same path at page scope must be unchanged.
    const { wrapper } = await mountCreate()

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(push).toHaveBeenCalledWith('/somewhere')
    expect(wrapper.emitted('inline-created')).toBeUndefined()
  })

  it('does not register a route guard when embedded', () => {
    // Two guards on one route both call the singleton confirm(), which shares
    // its in-flight promise — one dialog's answer would decide for both forms.
    mountCreate({ embedded: true })

    expect(onBeforeRouteLeave).not.toHaveBeenCalled()
  })

  it('registers a route guard when not embedded', async () => {
    await mountCreate()

    expect(onBeforeRouteLeave).toHaveBeenCalled()
  })

  it('does not register the document Cmd+Enter listener when embedded', async () => {
    // AC7. The listener is document-level with no target check, so a second
    // one would submit the host form behind the modal.
    const addSpy = vi.spyOn(document, 'addEventListener')
    await mountCreate({ embedded: true })

    expect(addSpy.mock.calls.filter(([type]) => type === 'keydown')).toHaveLength(0)
    addSpy.mockRestore()
  })

  it('registers the document Cmd+Enter listener when not embedded', async () => {
    const addSpy = vi.spyOn(document, 'addEventListener')
    await mountCreate()

    expect(addSpy.mock.calls.filter(([type]) => type === 'keydown').length).toBeGreaterThan(0)
    addSpy.mockRestore()
  })

  // Asserted on the DRY-RUN REQUEST rather than the rendered field or the
  // commit payload. The dry-run body carries the form's live values before any
  // affordance filtering, so it observes the pre-fill directly — a rendered
  // field can be absent for unrelated reasons, which would let the embedded
  // assertion pass even with the guard removed.
  it('ignores the host page query string when embedded', async () => {
    // A nested form is mounted over whatever route the host is on, so
    // `route.query` belongs to that page. Honouring `prop.*` would pre-fill
    // the nested entity from the host's parameters, and `link_*` would
    // auto-link it to the host's peer.
    await mountCreate({ embedded: true })

    const { dryRunCreateEntity } = await import('@/api')
    const body = vi.mocked(dryRunCreateEntity).mock.calls[0]?.[1] as {
      properties: Record<string, unknown>
    }
    expect(body.properties.title).toBeUndefined()
  })

  it('honours the query string when not embedded', async () => {
    await mountCreate()

    const { dryRunCreateEntity } = await import('@/api')
    const body = vi.mocked(dryRunCreateEntity).mock.calls[0]?.[1] as {
      properties: Record<string, unknown>
    }
    expect(body.properties.title).toBe('from-host-url')
  })

  it('emits inline-cancelled instead of going back', async () => {
    const { wrapper } = await mountCreate({ embedded: true })

    const cancel = wrapper.findAll('button').find((b) => b.text().includes('Cancel'))
    await cancel?.trigger('click')

    expect(wrapper.emitted('inline-cancelled')).toHaveLength(1)
  })

  it('hides the page header so the dialog title is not duplicated', async () => {
    const { wrapper } = await mountCreate({ embedded: true })

    expect(wrapper.find('.form-header').exists()).toBe(false)
  })

  it('renders the page header when not embedded', async () => {
    const { wrapper } = await mountCreate()

    expect(wrapper.find('.form-header').exists()).toBe(true)
  })
})

// The prop channel for pre-link / template / world (TKT-R4BMJM).
//
// An embedded form reads an EMPTY query — the mocked route above carries
// `prop.title=from-host-url`, which belongs to the page behind the modal and
// would silently pre-fill the nested entity. A host that genuinely holds this
// context therefore has to pass it explicitly.
//
// The risk these pin is that the props reopen the hole the empty-query rule
// closed: they must supply context the HOST chose, never resurrect the host
// page's own URL params.
describe('DynamicForm — embedded pre-link props', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('links via the prop, and still ignores the host page URL', async () => {
    // The peer's type is resolved from its id prefix, so the schema has to know
    // the prefix — as it does in the real app. Without it the relation carries
    // an untypeable id and reshapeLegacyToModern aborts the whole create.
    const schema = useSchemaStore()
    schema.entityTypes.set('feature', {
      name: 'feature',
      label: 'Feature',
      id_prefix: 'FEAT',
      properties: {},
    } as never)

    const { wrapper, create } = await mountCreate({
      embedded: true,
      embeddedLink: { relation: 'implements', peer: 'FEAT-1', linkAs: 'to' },
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(create.mock.calls.length, 'create should have been called').toBe(1)
    const payload = create.mock.calls[0][1] as {
      relations?: Record<string, { data: { type: string; id: string }[] }>
      properties?: Record<string, unknown>
    }
    // The edge rides the create payload for linkAs: 'to', in the modern
    // JSON:API resource-identifier shape. The TYPE matters as much as the id:
    // an untypeable peer makes reshapeLegacyToModern return null, which aborts
    // the entire create — so a pre-linked relation with no picker field on the
    // form could not save at all until the prefill registered its type.
    expect(payload.relations?.implements).toEqual({
      data: [{ type: 'feature', id: 'FEAT-1' }],
    })
    // And the host page's `prop.title` is still ignored — the prop channel
    // supplies context, it does not re-enable the URL overlay.
    expect(payload.properties?.title).not.toBe('from-host-url')
  })

  it('link_as=from creates the edge FROM the new entity, not from the peer', async () => {
    // The direction that had no coverage, and was therefore inverted.
    //
    // `link_as` names the NEW entity's role (the server's definition, in
    // internal/dataentry/sections.go). So `from` means new --relation--> peer,
    // and the edge must be addressed from the new entity: the endpoint is
    // /{plural}/{from}/relations/{rel} with the TO in the body.
    //
    // Two bugs hid here. The payload prefill ALSO added the peer, so an
    // incoming section wrote a backwards edge in addition to the correct one.
    // And the call was addressed from the peer, needing the peer's type from its
    // id prefix — which fails for a legitimately prefix-less id, so linking to
    // one was impossible.
    const api = await import('@/api')
    const createRelationMock = vi.mocked(api.createRelation)

    const { wrapper, create } = await mountCreate({
      embedded: true,
      embeddedLink: { relation: 'implements', peer: 'no-prefix-id', linkAs: 'from' },
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    // The payload must NOT carry the edge: it can only express
    // new --relation--> peer as `relations`, which is the `to` direction.
    const payload = create.mock.calls[0][1] as { relations?: Record<string, unknown> }
    expect(payload.relations?.implements).toBeUndefined()

    // (type, entityId, relation, targetId) => entityId --relation--> targetId.
    expect(createRelationMock).toHaveBeenCalledWith('ticket', CREATED.id, 'implements', 'no-prefix-id')
  })

  it('surfaces a link failure instead of silently creating an unlinked entity', async () => {
    // RR-8SP2UG: the old code skipped the link when it could not resolve a peer
    // type and said nothing, so the user got an orphan and no indication.
    const api = await import('@/api')
    vi.mocked(api.createRelation).mockRejectedValueOnce(new Error('nope'))

    const { wrapper } = await mountCreate({
      embedded: true,
      embeddedLink: { relation: 'implements', peer: 'FEAT-1', linkAs: 'from' },
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    // The entity is still reported created — it exists — but the failed link
    // must be visible rather than swallowed.
    const ui = useUIStore()
    expect(ui.toasts.some((t) => /linking it failed/i.test(t.message))).toBe(true)
  })

  it('creates in the world the host passed', async () => {
    // A faced type has no default row to fall back to, so a dropped world is a
    // refusal at the server (BUG-HC6I2T) — the failure is a 4xx, not a silent
    // wrong face, but the user sees a create that inexplicably failed.
    const { wrapper, create } = await mountCreate({
      embedded: true,
      embeddedWorld: 'published',
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect((create.mock.calls[0][1] as { world?: string }).world).toBe('published')
  })

  it('carries no world when the host had none', async () => {
    const { wrapper, create } = await mountCreate({ embedded: true })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect((create.mock.calls[0][1] as { world?: string }).world).toBeUndefined()
  })
})
