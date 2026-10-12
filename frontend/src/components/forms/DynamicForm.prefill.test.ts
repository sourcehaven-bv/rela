// Duplicate-prefill tests for DynamicForm (TKT-Z8K2FS).
//
// These cover `applyEmbeddedPrefill` and `routePrefilledCardRelations`, which
// were the two least-tested and most dangerous functions on the branch: both
// fail by producing a SUCCESSFUL create with missing data and no error. The
// card-routing bug that motivated them was caught only by e2e, and e2e can
// only ever exercise one form configuration — the permutations below are where
// the remaining risk lives.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import DynamicForm from './DynamicForm.vue'
import { getTemplates } from '@/api'
import type { Entity } from '@/types'

const push = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push, replace: vi.fn(), back: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/form/ticket-form' }),
  onBeforeRouteLeave: vi.fn(),
}))

vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return {
    ...actual,
    getTemplates: vi.fn().mockResolvedValue([]),
    dryRunCreateEntity: vi.fn().mockImplementation(async (_t: string, body: unknown) => ({
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
  properties: { title: { type: 'string' }, count: { type: 'integer' } },
}

// `blocks` is card-managed in both directions and declares an inverse, which
// is the shape that broke: card-delivered edges bypass `relations.value`.
// `refs` is a plain picker, so it takes the other path. `mirrors` is symmetric
// and its own inverse — legal, and the case where "is this key an inverse?"
// cannot be answered by map membership alone.
const RELATION_TYPES: Record<string, unknown> = {
  blocks: { label: 'blocks', inverse: { id: 'blockedBy' } },
  refs: { label: 'refs' },
  mirrors: { label: 'mirrors', symmetric: true, inverse: { id: 'mirrors' } },
  has_status: { label: 'has status', to: ['status'], max_outgoing: 1 },
}

const FORM = {
  id: 'ticket-form',
  entity: 'ticket',
  fields: [{ property: 'title', label: 'Title' }],
  relations: [
    { relation: 'blocks', direction: 'outgoing', widget: 'cards' },
    { relation: 'blocks', direction: 'incoming', widget: 'cards' },
    { relation: 'mirrors', direction: 'outgoing', widget: 'cards' },
    { relation: 'refs', direction: 'outgoing', widget: 'select' },
    { relation: 'has_status', direction: 'outgoing', widget: 'select' },
  ],
}

const CREATED: Entity = { id: 'TKT-9', type: 'ticket', properties: {}, warnings: [] }

const mounted: VueWrapper[] = []
afterEach(() => {
  mounted.splice(0).forEach((w) => {
    try {
      w.unmount()
    } catch {
      /* already torn down */
    }
  })
})

async function mountWithPrefill(prefill: {
  properties: Record<string, unknown>
  content?: string
  relations: Record<string, { id: string; type: string }[]>
}) {
  const schema = useSchemaStore()
  schema.forms.set(FORM.id, FORM as never)
  schema.entityTypes.set('ticket', ENTITY_TYPE as never)
  for (const [name, def] of Object.entries(RELATION_TYPES)) {
    schema.relationTypes.set(name, def as never)
  }
  schema.loaded = true

  const entities = useEntitiesStore()
  const create = vi.spyOn(entities, 'create').mockResolvedValue(CREATED)

  const wrapper = mount(DynamicForm, {
    props: { formId: FORM.id, embedded: true, embeddedPrefill: prefill },
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

/** The relations body of the single create call. */
function relationsBody(create: ReturnType<typeof vi.spyOn>) {
  const payload = create.mock.calls[0]?.[1] as { relations?: Record<string, unknown> }
  return payload?.relations ?? {}
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
})

describe('DynamicForm — duplicate prefill', () => {
  it('carries prefilled properties and content into the create payload', async () => {
    const { wrapper, create } = await mountWithPrefill({
      properties: { title: 'Copy', count: 3 },
      content: '# body',
      relations: {},
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const payload = create.mock.calls[0]?.[1] as {
      properties: Record<string, unknown>
      content?: string
    }
    expect(payload.properties.title).toBe('Copy')
    // Typed, not stringified — the reason the URL transport was rejected.
    expect(payload.properties.count).toBe(3)
    expect(payload.content).toBe('# body')
  })

  // The bug the e2e caught. A card-managed relation is excluded from
  // `relations.value` at submit and taken from `pendingCardChanges` instead, so
  // a prefill that wrote only the former produced a create with no edges.
  it('routes a card-managed outgoing relation into the create body', async () => {
    const { wrapper, create } = await mountWithPrefill({
      properties: { title: 'Copy' },
      content: '',
      relations: { blocks: [{ id: 'TKT-2', type: 'ticket' }] },
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(relationsBody(create).blocks).toEqual({
      add: [{ type: 'ticket', id: 'TKT-2' }],
    })
  })

  // An incoming edge arrives under the INVERSE key and must stay there: the
  // server resolves the direction, so translating it client-side would invert
  // it twice and write the edge backwards.
  it('keeps an incoming edge under its inverse key', async () => {
    const { wrapper, create } = await mountWithPrefill({
      properties: { title: 'Copy' },
      content: '',
      relations: { blockedBy: [{ id: 'TKT-3', type: 'ticket' }] },
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(relationsBody(create).blockedBy).toEqual({
      add: [{ type: 'ticket', id: 'TKT-3' }],
    })
    expect(relationsBody(create).blocks).toBeUndefined()
  })

  // A non-card relation takes the picker path, where an id with no registered
  // type makes reshapeLegacyToModern return null and ABORT the whole create.
  // The prefill seeds pickerTypes from the peer type it already carries.
  it('seeds picker types so a non-card relation does not abort the create', async () => {
    const { wrapper, create } = await mountWithPrefill({
      properties: { title: 'Copy' },
      content: '',
      relations: { refs: [{ id: 'TKT-5', type: 'ticket' }] },
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(create).toHaveBeenCalled()
    expect(relationsBody(create).refs).toEqual({
      add: [{ type: 'ticket', id: 'TKT-5' }],
    })
  })

  it('carries card and picker relations together in one create', async () => {
    const { wrapper, create } = await mountWithPrefill({
      properties: { title: 'Copy' },
      content: '',
      relations: {
        blocks: [{ id: 'TKT-2', type: 'ticket' }],
        blockedBy: [{ id: 'TKT-3', type: 'ticket' }],
        refs: [{ id: 'TKT-5', type: 'ticket' }],
      },
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const body = relationsBody(create)
    expect(Object.keys(body).sort()).toEqual(['blockedBy', 'blocks', 'refs'])
  })

  // The dialog's discard guard is only useful if it distinguishes a touched
  // form from an untouched one. applyTemplate baselines mid-prefill and the
  // card routing then mutates further, so the baseline has to be retaken.
  it('is not dirty before the user edits anything', async () => {
    const { wrapper } = await mountWithPrefill({
      properties: { title: 'Copy' },
      content: '',
      relations: { blocks: [{ id: 'TKT-2', type: 'ticket' }] },
    })

    const vm = wrapper.vm as unknown as { isDirty: () => boolean }
    expect(vm.isDirty()).toBe(false)
  })

  it('creates nothing extra when there is no prefill', async () => {
    const schema = useSchemaStore()
    schema.forms.set(FORM.id, FORM as never)
    schema.entityTypes.set('ticket', ENTITY_TYPE as never)
    schema.loaded = true
    const entities = useEntitiesStore()
    const create = vi.spyOn(entities, 'create').mockResolvedValue(CREATED)

    const wrapper = mount(DynamicForm, {
      props: { formId: FORM.id, embedded: true },
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

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(relationsBody(create)).toEqual({})
  })

  // A Duplicate deliberately carries whatever the source entity had, including
  // relation types this create form never renders — the peers arrive fully
  // typed, so these edges were written before the ownership filter existed
  // (BUG-KQSOJ2). Dropping them would silently narrow what "duplicate" means,
  // which is a behaviour change rather than a fix, so the filter exempts them.
  it('carries a prefilled relation the form does not render', async () => {
    const { wrapper, create } = await mountWithPrefill({
      properties: {},
      content: '',
      relations: { unrendered: [{ id: 'TKT-77', type: 'ticket' }] },
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(create).toHaveBeenCalledTimes(1)
    expect(relationsBody(create).unrendered).toEqual({
      add: [{ type: 'ticket', id: 'TKT-77' }],
    })
  })
})

// A board column's or list section's Add prefills a single-valued relation
// (TKT-KJ3Q07). A template that defaults the same relation must not add a
// second target: the server refuses two targets on `max_outgoing: 1`.
describe('DynamicForm — single-valued relation prefill', () => {
  const templates = [
    { name: 'default', properties: {}, content: '', relations: [{ relation: 'has_status', target: 'ST-1' }] },
    { name: 'urgent', properties: {}, content: 'urgent body', relations: [{ relation: 'has_status', target: 'ST-2' }] },
  ]

  it('replaces the template value instead of adding to it', async () => {
    vi.mocked(getTemplates).mockResolvedValueOnce(templates as never)
    const { wrapper, create } = await mountWithPrefill({
      properties: {},
      relations: { has_status: [{ id: 'ST-3', type: 'status' }] },
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(relationsBody(create).has_status).toEqual({
      add: [{ type: 'status', id: 'ST-3' }],
    })
  })

  it('keeps the prefill when the user picks another template', async () => {
    vi.mocked(getTemplates).mockResolvedValueOnce(templates as never)
    const { wrapper, create } = await mountWithPrefill({
      properties: {},
      relations: { has_status: [{ id: 'ST-3', type: 'status' }] },
    })

    const pill = wrapper.findAll('.template-pill').find((b) => b.text() === 'Urgent')
    expect(pill).toBeDefined()
    await pill?.trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    const payload = create.mock.calls[0]?.[1] as { content?: string }
    expect(relationsBody(create).has_status).toEqual({
      add: [{ type: 'status', id: 'ST-3' }],
    })
    // The picked template's body stands; the prefill brought none.
    expect(payload.content).toBe('urgent body')
  })

  it('still adds to a multi-valued relation', async () => {
    vi.mocked(getTemplates).mockResolvedValueOnce([
      { name: 'default', properties: {}, content: '', relations: [{ relation: 'refs', target: 'TKT-1' }] },
    ] as never)
    const { wrapper } = await mountWithPrefill({
      properties: {},
      relations: { refs: [{ id: 'TKT-5', type: 'ticket' }] },
    })

    // The baseline is taken after the prefill, so it holds the form's relations.
    const vm = wrapper.vm as unknown as { _originalData: () => string }
    const baseline = JSON.parse(vm._originalData()) as { relations: Record<string, string[]> }
    expect(baseline.relations.refs).toEqual(['TKT-1', 'TKT-5'])
  })
})
