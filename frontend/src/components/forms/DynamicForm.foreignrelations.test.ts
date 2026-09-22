// Regression tests for BUG-KQSOJ2: an entity's relations that the form does
// not render must not ride along on the save.
//
// `relations.value` is seeded from the entity GET, which returns EVERY relation
// the entity has. The submit paths then sent that whole map. A relation with no
// field on the form has no `pickerTypes` entry — only a rendered RelationPicker
// fills one — so `reshapeLegacyToModern` returned null and the ENTIRE relations
// payload was dropped, including the edit the user had just made. The user saw
// "Some related entities have unknown types" and advice to reload, which could
// not help: the GET returns the same untyped key every time.
//
// The live shape this reproduces: a `taak` whose GET carries `gaat_over` (a
// rendered picker) and `onderdeel_van` (the inverse of `bestaat_uit`, rendered
// only as an INCOMING field, which keeps its state elsewhere and never types
// anything here). Editing "Gaat over" saved nothing.

import { describe, it, expect, beforeEach, afterEach, vi, type Mock } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import DynamicForm from './DynamicForm.vue'
import type { Entity } from '@/types'

// Mutable so the create-mode cases can drive `rel.*` / `link_*` prefills,
// which DynamicForm reads from the route rather than from props.
const routeQuery: Record<string, string> = {}

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: routeQuery, params: {}, path: '/form/edit_taak' }),
  onBeforeRouteLeave: vi.fn(),
}))

vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return { ...actual, getTemplates: vi.fn().mockResolvedValue([]) }
})

const ENTITY_TYPE = {
  name: 'taak',
  label: 'Taak',
  id_type: 'short',
  properties: { titel: { type: 'string' } },
}

// Mirrors atlas's edit_taak: an outgoing picker for `gaat_over`, and
// `bestaat_uit` rendered INCOMING — whose edges arrive from the API under the
// inverse name `onderdeel_van`.
const FORM = {
  id: 'edit_taak',
  entity: 'taak',
  fields: [{ property: 'titel', label: 'Titel' }],
  relations: [
    { relation: 'gaat_over', direction: 'outgoing' },
    { relation: 'bestaat_uit', direction: 'incoming', label: 'Valt onder' },
  ],
}

const RELATION_TYPES = {
  gaat_over: { name: 'gaat_over', from: ['taak'], to: ['ncr'], inverse: { id: 'heeft_planning' } },
  bestaat_uit: {
    name: 'bestaat_uit',
    from: ['project'],
    to: ['taak'],
    inverse: { id: 'onderdeel_van' },
  },
}

function stubStores() {
  const schema = useSchemaStore()
  schema.forms.set(FORM.id, FORM as never)
  schema.entityTypes.set('taak', ENTITY_TYPE as never)
  for (const [name, rt] of Object.entries(RELATION_TYPES)) {
    schema.relationTypes.set(name, rt as never)
  }
  schema.loaded = true
}

function stubEntity(relations: Record<string, string[]>) {
  const entities = useEntitiesStore()
  const entity: Entity = {
    id: 'TASK-7F8K',
    type: 'taak',
    properties: { titel: 'Jaarlijkse Externe Audit 2026' },
    relations,
    _actions: { update: true },
    _fields: {},
    _redacted: [],
    _relations: {},
  }
  vi.spyOn(entities, 'fetchEntity').mockResolvedValue(entity)
  const update = vi
    .spyOn(entities, 'update')
    .mockResolvedValue({ ...entity, warnings: [] } as Entity) as unknown as Mock
  return { update }
}

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

async function mountEdit(relations: Record<string, string[]>) {
  stubStores()
  const { update } = stubEntity(relations)
  const wrapper = mount(DynamicForm, {
    props: { formId: FORM.id, entityId: 'TASK-7F8K' },
    global: {
      stubs: {
        RouterLink: true,
        MarkdownEditor: true,
        // Stubbed on purpose: a stubbed picker emits no `update:types`, which
        // is exactly the state a real picker is in for a relation the form
        // does not render. That is the bug's precondition.
        RelationPicker: true,
        RelationCards: true,
        AutoSaveIndicator: true,
        SidePanel: true,
      },
      mocks: {
        $router: { push: vi.fn(), replace: vi.fn() },
        $route: { query: {}, params: {}, path: '/form/edit_taak' },
      },
    },
  })
  mounted.push(wrapper)
  await flushPromises()
  return { wrapper, update }
}

// Drive the outgoing picker the way RelationPicker does: ids via `update`,
// then the id -> type map via `update:types`. Reaching the stub through the
// component instance keeps this independent of the stub's rendered markup.
async function pickRelation(wrapper: VueWrapper, relation: string, ids: string[], type: string) {
  const form = wrapper.vm as unknown as {
    updateRelation: (r: string, v: string[]) => void
    updateRelationTypes: (r: string, t: Map<string, string>) => void
  }
  form.updateRelation(relation, ids)
  form.updateRelationTypes(relation, new Map(ids.map((id) => [id, type])))
  await flushPromises()
}

describe('DynamicForm foreign relations (BUG-KQSOJ2)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  // The bug, at the level it was reported: adding a procedure to "Gaat over"
  // produced an error toast and saved nothing.
  it('saves an edited relation even when the entity carries an unrendered one', async () => {
    const { wrapper, update } = await mountEdit({
      gaat_over: ['NCR-001'],
      onderdeel_van: ['PROJ-DP5C'],
    })
    const uiError = vi.spyOn(useUIStore(), 'error')

    await pickRelation(wrapper as VueWrapper, 'gaat_over', ['NCR-001', 'NCR-002'], 'ncr')
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()

    expect(uiError).not.toHaveBeenCalled()
    expect(update).toHaveBeenCalledTimes(1)
    const body = update.mock.calls[0][2] as { relations: Record<string, unknown> }
    expect(body.relations.gaat_over).toEqual({
      data: [
        { type: 'ncr', id: 'NCR-001' },
        { type: 'ncr', id: 'NCR-002' },
      ],
    })
  })

  // The second half of the fix, independent of the abort: a form must not
  // assert a full-replace value for edges it never showed the user.
  it('does not send a relation the form does not render', async () => {
    const { wrapper, update } = await mountEdit({
      gaat_over: ['NCR-001'],
      onderdeel_van: ['PROJ-DP5C'],
    })

    await pickRelation(wrapper as VueWrapper, 'gaat_over', ['NCR-002'], 'ncr')
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()

    // Asserted before indexing: a regression that stopped the save entirely
    // would otherwise fail on an `undefined` index rather than say so.
    expect(update).toHaveBeenCalledTimes(1)
    const body = update.mock.calls[0][2] as { relations: Record<string, unknown> }
    expect(body.relations).not.toHaveProperty('onderdeel_van')
  })

  // Ownership must not be so narrow that it drops the form's own edges: the
  // rendered outgoing relation has to survive alongside the dropped ones.
  it('keeps the rendered outgoing relation while dropping unrendered ones', async () => {
    const { wrapper, update } = await mountEdit({
      gaat_over: ['NCR-001'],
      spawnt: ['SCHED-1'],
    })

    await pickRelation(wrapper as VueWrapper, 'gaat_over', ['NCR-002'], 'ncr')
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()

    const body = update.mock.calls[0][2] as { relations: Record<string, unknown> }
    expect(body.relations).not.toHaveProperty('spawnt')
    expect(body.relations).toHaveProperty('gaat_over')
  })

  // The toast a user can still reach — a rendered picker whose own target has
  // no type — must name the relation instead of sending them to a reload with
  // no further information.
  it('names the offending relation when a rendered picker target has no type', async () => {
    const { wrapper, update } = await mountEdit({ gaat_over: ['NCR-001'] })
    const uiError = vi.spyOn(useUIStore(), 'error')

    // ids without the accompanying type map — a picker that has not resolved.
    const form = wrapper.vm as unknown as { updateRelation: (r: string, v: string[]) => void }
    form.updateRelation('gaat_over', ['NCR-001', 'NCR-002'])
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()

    expect(update).not.toHaveBeenCalled()
    expect(uiError).toHaveBeenCalledWith(expect.stringContaining('gaat_over'))
  })
})

// `rel.<name>=<id>` query params are a prefill channel INDEPENDENT of
// `link_relation`/`link_peer`, and nothing constrains the name to a relation
// the form renders. The ownership filter must therefore exempt them.
//
// Getting this wrong is worse than the bug being fixed: the create would
// succeed, write no edge, and say nothing — the caller returns to the
// originating entity and the section is still empty, which is indistinguishable
// from a stale page. The loud abort it replaced was at least diagnosable.
describe('DynamicForm prefilled relations survive the ownership filter', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
    for (const k of Object.keys(routeQuery)) delete routeQuery[k]
  })

  async function mountCreate() {
    stubStores()
    const entities = useEntitiesStore()
    const create = vi.spyOn(entities, 'create').mockResolvedValue({
      id: 'TASK-NEW',
      type: 'taak',
      properties: {},
      warnings: [],
    } as unknown as Entity)
    const wrapper = mount(DynamicForm, {
      props: { formId: FORM.id },
      global: {
        stubs: {
          RouterLink: true,
          MarkdownEditor: true,
          RelationPicker: true,
          RelationCards: true,
          AutoSaveIndicator: true,
          SidePanel: true,
          HelpModal: true,
        },
        mocks: { $router: { push: vi.fn(), replace: vi.fn() }, $route: { query: routeQuery } },
      },
    })
    mounted.push(wrapper)
    await flushPromises()
    return { wrapper, create }
  }

  async function submit(wrapper: VueWrapper) {
    await wrapper.find('form').trigger('submit')
    await flushPromises()
  }

  it('carries a rel.* prefill for a relation the form renders', async () => {
    routeQuery['rel.gaat_over'] = 'NCR-001'
    const { wrapper, create } = await mountCreate()
    const form = wrapper.vm as unknown as {
      updateRelationTypes: (r: string, t: Map<string, string>) => void
    }
    // The rendered picker resolves the prefilled id's type, as it does live.
    form.updateRelationTypes('gaat_over', new Map([['NCR-001', 'ncr']]))
    await flushPromises()

    await submit(wrapper as VueWrapper)

    expect(create).toHaveBeenCalledTimes(1)
    const body = create.mock.calls[0][1] as { relations: Record<string, unknown> }
    expect(body.relations.gaat_over).toEqual({ data: [{ type: 'ncr', id: 'NCR-001' }] })
  })

  // The dangerous case: the prefill names a relation with NO field on this
  // form, so nothing types it. Before the exemption it was filtered out, the
  // create succeeded, and the edge was never written — a silent no-write.
  //
  // The exemption keeps it in the payload, where the untypeable id then fails
  // LOUDLY. Both halves are asserted: no entity is created, and the user is
  // told. A quiet success here is the regression this guards.
  it('never silently drops a rel.* prefill for an unrendered relation', async () => {
    routeQuery['rel.onderdeel_van'] = 'PROJ-DP5C'
    const { wrapper, create } = await mountCreate()
    const uiError = vi.spyOn(useUIStore(), 'error')

    await submit(wrapper as VueWrapper)

    expect(create).not.toHaveBeenCalled()
    expect(uiError).toHaveBeenCalledTimes(1)
  })

  // An untypeable prefill on a relation the form does not render cannot be
  // helped by reloading, so the message must not say so — it is a config
  // problem, and the operator needs pointing at the create button.
  it('does not advise a reload for an untypeable unrendered prefill', async () => {
    routeQuery['rel.onderdeel_van'] = 'PROJ-DP5C'
    const { wrapper } = await mountCreate()
    const uiError = vi.spyOn(useUIStore(), 'error')

    await submit(wrapper as VueWrapper)

    const msg = uiError.mock.calls.map((c) => String(c[0])).join(' ')
    expect(msg).toContain('onderdeel_van')
    expect(msg).not.toContain('Reload the form')
  })
})
