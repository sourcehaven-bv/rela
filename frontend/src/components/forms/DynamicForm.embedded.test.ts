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
import { useSchemaStore, useEntitiesStore } from '@/stores'
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

async function mountCreate(props: { embedded?: boolean } = {}) {
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
