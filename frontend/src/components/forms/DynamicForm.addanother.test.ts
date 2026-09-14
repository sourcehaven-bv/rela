// "Create & add another" — the create form's second terminal outcome
// (TKT-7YHKD1).
//
// The interesting behaviour is not the button, it is the RESET. The create
// form was single-use by construction (`createdEntityId` latches after a
// successful create so a second submit cannot mint a duplicate), and it holds
// create-mode state in a good deal more than the three obvious refs. Anything
// the reset misses leaks record N into record N+1 — a silent wrong-data write,
// which is exactly what these tests exist to catch.
//
// Design review (PLAN-ONV2PB) found the first draft of the reset missing six
// refs plus the incoming-picker path; the assertions below are the pins for
// those findings.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { useSchemaStore, useEntitiesStore, useUIStore } from '@/stores'
import DynamicForm from './DynamicForm.vue'
import type { Entity } from '@/types'
import { defineComponent, h, onMounted } from 'vue'

// Counts mounts. An incoming-direction RelationPicker holds its selection in
// its OWN ref and ignores the `value` prop, so clearing `relations` does not
// reach it — only a remount (driven by `saveGeneration` in its `:key`) does.
// Counting mounts is therefore the honest assertion: it tests the mechanism
// the real widget depends on, which a value-prop stub could not.
let pickerMounts = 0
// Mount-counting AND emitting. The emit half is load-bearing: an incoming
// picker's selection reaches DynamicForm through `incoming-changed` into
// `pendingCardChanges`, and a stub that only counts mounts validates the
// remount while being structurally incapable of exercising the state path
// that actually leaks between records.
const CountingPicker = defineComponent({
  name: 'RelationPicker',
  props: { field: { type: Object, required: true }, value: { type: Array, default: () => [] } },
  // Names match RelationPicker's real emits; FormFieldList re-emits them
  // upward with the relation name attached, so the stub must not add it.
  emits: ['update', 'update:types', 'incoming-changed'],
  setup(props, { emit }) {
    onMounted(() => {
      pickerMounts += 1
    })
    return () =>
      h('button', {
        class: 'stub-picker',
        onClick: () => {
          if (props.field.direction === 'incoming') {
            const entries = [{ type: 'task', id: PEER_ID }]
            emit('incoming-changed', {
              currentEntries: entries,
              added: entries,
              removed: [],
            })
          } else {
            emit('update:types', new Map([[PEER_ID, 'task']]))
            emit('update', [PEER_ID])
          }
        },
      })
  },
})

const PEER_ID = 'TASK-999'

const push = vi.fn()
const replace = vi.fn()

// The route query is read by initializeDefaults for prop.*/rel.*/link_*
// pre-fills. It is mutable here so one test can mount with a pre-fill and
// assert it survives the reset (the reset does NOT navigate, so the query is
// genuinely unchanged in the real app too).
let routeQuery: Record<string, string> = {}

vi.mock('vue-router', () => ({
  useRouter: () => ({ push, replace, back: vi.fn() }),
  useRoute: () => ({ query: routeQuery, params: {}, path: '/form/task-form' }),
  onBeforeRouteLeave: vi.fn(),
}))

const getTemplates = vi.fn()

// Extra `_fields` verdicts the dry-run should return, per test. A non-writable
// field makes adoptLockedFieldValues write the server's value into formData
// AFTER the reset's other steps — the RR-1LMJSQ dirty hazard.
let dryRunFields: Record<string, unknown> = {}
let dryRunExtraProps: Record<string, unknown> = {}

vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return {
    ...actual,
    getTemplates: (...args: unknown[]) => getTemplates(...args),
    // Echo the submitted properties back on top of the full key set, the way
    // the real dry-run does — echoing only submitted keys would mark every
    // untouched field policy-hidden and unrender the form.
    dryRunCreateEntity: vi.fn().mockImplementation(async (_t: string, body: unknown) => ({
      properties: {
        title: '',
        project: '',
        notes: '',
        flag: false,
        detail: '',
        ...((body as { properties?: Record<string, unknown> })?.properties ?? {}),
        ...dryRunExtraProps,
      },
      _fields: dryRunFields,
      _relations: {},
      warnings: [],
    })),
    createRelation: vi.fn().mockResolvedValue(undefined),
  }
})

const ENTITY_TYPE = {
  name: 'task',
  label: 'Task',
  id_type: 'sequential',
  properties: {
    title: { type: 'string' },
    // `project` carries a default so the "kept value beats the default" case
    // has something to beat.
    project: { type: 'string', default: 'inbox' },
    notes: { type: 'string' },
    flag: { type: 'boolean' },
    detail: { type: 'string' },
  },
}

// `project` is marked to carry over; `title` and `notes` are not.
const FORM = {
  id: 'task-form',
  entity: 'task',
  fields: [
    { property: 'title', label: 'Title' },
    { property: 'project', label: 'Project', keep_on_add_another: true },
    { property: 'notes', label: 'Notes' },
  ],
}

const EDIT_FORM = { id: 'task-edit', entity: 'task', mode: 'edit', fields: FORM.fields }

// `detail` is revealed only when `flag` is true, with the default
// clear_when_hidden ("no") — so hiding it RETAINS its value client-side for a
// lossless reveal. That retention map is form state like any other and must
// not survive into the next record.
const CONDITIONAL_FORM = {
  id: 'task-conditional',
  entity: 'task',
  fields: [
    { property: 'title' },
    { property: 'flag', widget: 'checkbox' },
    { property: 'detail', visible_when: 'form.flag == true' },
  ],
}

// A form with an incoming relation field, for the remount assertion.
const INCOMING_FORM = {
  id: 'task-incoming',
  entity: 'task',
  fields: [{ property: 'title' }],
  relations: [{ relation: 'blocks', direction: 'incoming' }],
}

// An OUTGOING relation marked to carry over — the case the docs promote
// hardest ("a project, an assignee").
const KEPT_REL_FORM = {
  id: 'task-kept-rel',
  entity: 'task',
  fields: [{ property: 'title' }],
  relations: [{ relation: 'blocks', keep_on_add_another: true }],
}

const WIZARD_FORM = {
  id: 'task-wizard',
  entity: 'task',
  steps: [
    { id: 's1', title: 'Basics', fields: [{ property: 'title' }] },
    { id: 's2', title: 'Detail', fields: [{ property: 'notes' }] },
  ],
}

let created = 0
function nextEntity(): Entity {
  created += 1
  return { id: `TASK-${created}`, type: 'task', properties: {}, warnings: [] }
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

async function mountCreate(form: { id: string } = FORM) {
  const schema = useSchemaStore()
  schema.forms.set(FORM.id, FORM as never)
  schema.forms.set(EDIT_FORM.id, EDIT_FORM as never)
  schema.forms.set(WIZARD_FORM.id, WIZARD_FORM as never)
  schema.forms.set(CONDITIONAL_FORM.id, CONDITIONAL_FORM as never)
  schema.forms.set(INCOMING_FORM.id, INCOMING_FORM as never)
  schema.forms.set(KEPT_REL_FORM.id, KEPT_REL_FORM as never)
  schema.entityTypes.set('task', ENTITY_TYPE as never)
  // The incoming-direction patch builder refuses a relation with no declared
  // inverse ("DynamicForm should have pre-flighted this"), so the picker tests
  // need a real relation type here.
  schema.relationTypes.set('blocks', {
    id: 'blocks',
    from: ['task'],
    to: ['task'],
    inverse: { id: 'blocked-by' },
  } as never)
  schema.loaded = true

  const entities = useEntitiesStore()
  const create = vi.spyOn(entities, 'create').mockImplementation(async () => nextEntity())

  const wrapper = mount(DynamicForm, {
    props: { formId: form.id },
    global: {
      stubs: {
        RouterLink: true,
        MarkdownEditor: true,
        RelationPicker: CountingPicker,
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

function addAnotherButton(wrapper: VueWrapper) {
  return wrapper.findAll('button').find((b) => b.text().includes('Create & add another'))
}

async function clickAddAnother(wrapper: VueWrapper) {
  const btn = addAnotherButton(wrapper)
  if (!btn) throw new Error('no "Create & add another" button')
  await btn.trigger('click')
  await flushPromises()
}

async function fill(wrapper: VueWrapper, property: string, value: string) {
  const input = wrapper.find(`#field-${property}`)
  await input.setValue(value)
  await flushPromises()
}

function valueOf(wrapper: VueWrapper, property: string): string {
  return (wrapper.find(`#field-${property}`).element as HTMLInputElement).value
}

describe('DynamicForm — Create & add another', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    created = 0
    routeQuery = {}
    dryRunFields = {}
    dryRunExtraProps = {}
    pickerMounts = 0
    getTemplates.mockResolvedValue([])
  })

  // AC-1
  it('renders the action on a create form', async () => {
    const { wrapper } = await mountCreate()
    expect(addAnotherButton(wrapper)).toBeDefined()
  })

  it('does not render the action on an edit form', async () => {
    const schema = useSchemaStore()
    schema.forms.set(EDIT_FORM.id, EDIT_FORM as never)
    schema.entityTypes.set('task', ENTITY_TYPE as never)
    schema.loaded = true
    const entities = useEntitiesStore()
    vi.spyOn(entities, 'fetchEntity').mockResolvedValue({
      id: 'TASK-1',
      type: 'task',
      properties: { title: 'x' },
    } as never)

    const wrapper = mount(DynamicForm, {
      props: { formId: EDIT_FORM.id, entityId: 'TASK-1' },
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
    expect(addAnotherButton(wrapper)).toBeUndefined()
  })

  it('does not render the action when embedded', async () => {
    const schema = useSchemaStore()
    schema.forms.set(FORM.id, FORM as never)
    schema.entityTypes.set('task', ENTITY_TYPE as never)
    schema.loaded = true

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
    expect(addAnotherButton(wrapper)).toBeUndefined()
  })

  // AC-2. Driving BOTH buttons in ONE test and comparing the two calls is the
  // point: two separate tests could only compare against duplicated literals,
  // which drift, and would not actually test "the buttons cannot diverge".
  it('builds the same payload as the primary Create', async () => {
    const { wrapper, create } = await mountCreate()
    await fill(wrapper, 'title', 'same')
    await fill(wrapper, 'notes', 'body')

    await clickAddAnother(wrapper)
    const viaAddAnother = create.mock.calls[0]

    // Refill identically and submit via the primary button.
    await fill(wrapper, 'title', 'same')
    await fill(wrapper, 'notes', 'body')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(create).toHaveBeenCalledTimes(2)
    expect(create.mock.calls[1]).toEqual(viaAddAnother)
  })

  // AC-3
  it('does not navigate', async () => {
    const { wrapper } = await mountCreate()
    await fill(wrapper, 'title', 'stay put')
    await clickAddAnother(wrapper)
    expect(push).not.toHaveBeenCalled()
  })

  // AC-4
  it('clears unmarked fields back to their defaults', async () => {
    const { wrapper } = await mountCreate()
    await fill(wrapper, 'title', 'first')
    await fill(wrapper, 'notes', 'scratch')
    await clickAddAnother(wrapper)

    expect(valueOf(wrapper, 'title')).toBe('')
    expect(valueOf(wrapper, 'notes')).toBe('')
  })

  it('re-applies prop.* query pre-fills after the reset', async () => {
    routeQuery = { 'prop.notes': 'from-url' }
    const { wrapper } = await mountCreate()
    expect(valueOf(wrapper, 'notes')).toBe('from-url')

    await fill(wrapper, 'title', 'first')
    await fill(wrapper, 'notes', 'typed over it')
    await clickAddAnother(wrapper)

    // The query string is unchanged (we never navigated), so the pre-fill is
    // re-derived rather than remembered.
    expect(valueOf(wrapper, 'notes')).toBe('from-url')
  })

  // AC-4b
  it('carries over a field marked keep_on_add_another, and only that field', async () => {
    const { wrapper } = await mountCreate()
    await fill(wrapper, 'title', 'first')
    await fill(wrapper, 'project', 'apollo')
    await fill(wrapper, 'notes', 'scratch')

    await clickAddAnother(wrapper)

    expect(valueOf(wrapper, 'project')).toBe('apollo')
    expect(valueOf(wrapper, 'title')).toBe('')
    expect(valueOf(wrapper, 'notes')).toBe('')
  })

  it('lets a kept value beat the default that would otherwise be restored', async () => {
    // `project` defaults to "inbox". Entering something else and keeping it
    // proves the re-apply happens AFTER initializeDefaults, not before.
    const { wrapper } = await mountCreate()
    expect(valueOf(wrapper, 'project')).toBe('inbox')

    await fill(wrapper, 'title', 'first')
    await fill(wrapper, 'project', 'apollo')
    await clickAddAnother(wrapper)

    expect(valueOf(wrapper, 'project')).toBe('apollo')
  })

  it('sends the kept value on the SECOND create, not just in the DOM', async () => {
    // The DOM assertions above would still pass if the kept value were
    // re-applied without being marked touched — in which case the commit
    // filter would drop it and record 2 would be written without it.
    const { wrapper, create } = await mountCreate()
    await fill(wrapper, 'title', 'first')
    await fill(wrapper, 'project', 'apollo')
    await clickAddAnother(wrapper)

    await fill(wrapper, 'title', 'second')
    await clickAddAnother(wrapper)

    const second = create.mock.calls[1][1] as { properties: Record<string, unknown> }
    expect(second.properties.project).toBe('apollo')
    expect(second.properties.title).toBe('second')
  })

  // AC-5 — the latch regression. The form is single-use by construction; the
  // reset must release that latch or the second create silently no-ops.
  it('allows a second create from the same mounted form', async () => {
    const { wrapper, create } = await mountCreate()

    await fill(wrapper, 'title', 'first')
    await clickAddAnother(wrapper)

    await fill(wrapper, 'title', 'second')
    await clickAddAnother(wrapper)

    expect(create).toHaveBeenCalledTimes(2)
    const titles = create.mock.calls.map(
      (c: unknown[]) => (c[1] as { properties: Record<string, unknown> }).properties.title
    )
    expect(titles).toEqual(['first', 'second'])
  })

  // AC-6
  it('names the created entity in the success toast', async () => {
    const ui = useUIStore()
    const success = vi.spyOn(ui, 'success')
    const { wrapper } = await mountCreate()

    await fill(wrapper, 'title', 'first')
    await clickAddAnother(wrapper)

    expect(success).toHaveBeenCalledTimes(1)
    expect(success.mock.calls[0][0]).toContain('TASK-1')
  })

  it('announces the create and reset to assistive tech', async () => {
    const { wrapper } = await mountCreate()
    await fill(wrapper, 'title', 'first')
    await clickAddAnother(wrapper)
    await flushPromises()

    const region = wrapper.find('[role="status"]')
    expect(region.exists()).toBe(true)
    expect(region.text()).toContain('TASK-1')
  })

  // AC-7 — must MOUNT the component; DynamicForm.guard.test.ts replicates the
  // guard in a stand-in component and would prove nothing here.
  it('leaves the form not dirty', async () => {
    const { wrapper } = await mountCreate()
    await fill(wrapper, 'title', 'first')

    const vm = wrapper.vm as unknown as { isDirty: () => boolean }
    expect(vm.isDirty()).toBe(true)

    await clickAddAnother(wrapper)
    await flushPromises()

    expect(vm.isDirty()).toBe(false)
  })

  // RR-1LMJSQ's substantive half. adoptLockedFieldValues writes the server's
  // value for an entry-locked field into formData from INSIDE the dry-run,
  // which the reset awaits — so a dirty baseline captured any earlier leaves
  // the form dirty the instant that resolves, and the user gets a spurious
  // "unsaved changes" prompt after every single record.
  it('is not left dirty by an entry-locked field the dry-run fills in', async () => {
    dryRunFields = { notes: { writable: false } }
    dryRunExtraProps = { notes: 'server-assigned' }

    const { wrapper } = await mountCreate()
    await fill(wrapper, 'title', 'first')
    await clickAddAnother(wrapper)
    await flushPromises()

    const vm = wrapper.vm as unknown as { isDirty: () => boolean }
    expect(vm.isDirty()).toBe(false)

    // `dirty` is only recomputed on the next edit, so the flag above lags.
    // Assert the BASELINE directly: it must already contain the locked value
    // the dry-run wrote. If originalData were captured before the awaited
    // dry-run, the locked value would be missing from it and the form would
    // report dirty on the user's very first keystroke, every record.
    //
    // (Compared as parsed objects: the serialization is key-order and
    // key-presence sensitive — an untouched field contributes no key at all —
    // so a raw string compare would assert an artifact rather than the value.)
    const baseline = JSON.parse(
      (wrapper.vm as unknown as { _originalData: () => string })._originalData()
    ) as { formData: Record<string, unknown> }
    expect(baseline.formData.notes).toBe('server-assigned')
  })

  // AC-8
  it('does not reset or claim success when the create fails', async () => {
    const ui = useUIStore()
    const success = vi.spyOn(ui, 'success')
    const { wrapper } = await mountCreate()
    const entities = useEntitiesStore()
    vi.spyOn(entities, 'create').mockRejectedValue(new Error('boom'))

    await fill(wrapper, 'title', 'survives')
    await fill(wrapper, 'project', 'apollo')
    await clickAddAnother(wrapper)

    expect(valueOf(wrapper, 'title')).toBe('survives')
    expect(valueOf(wrapper, 'project')).toBe('apollo')
    expect(success).not.toHaveBeenCalled()
    expect(push).not.toHaveBeenCalled()
  })

  // AC-9
  it('returns a wizard form to the first step', async () => {
    const { wrapper } = await mountCreate(WIZARD_FORM)

    // Advance to the last step, where the action renders.
    const next = wrapper.findAll('button').find((b) => b.text().includes('Next step'))
    await next!.trigger('click')
    await flushPromises()
    await fill(wrapper, 'notes', 'detail')

    await clickAddAnother(wrapper)

    // Back on step 1: its field is rendered and the last step's is not.
    expect(wrapper.find('#field-title').exists()).toBe(true)
    expect(wrapper.find('#field-notes').exists()).toBe(false)
  })

  // RR-ZYU2GC raised this as a retained-value hazard by analogy with
  // loadEntity. It does NOT apply on the create path — useChangePolicy is
  // gated on `enabled: () => isEdit.value`, so the retention map is never
  // populated here (verified: removing `hiddenPolicy.releaseAll()` from the
  // reset changes no test). The conditional-branch behaviour is still worth
  // pinning on its own terms: a value entered behind a condition on record 1
  // must not reappear when the same branch is revealed on record 2, whichever
  // mechanism would have carried it.
  it("does not carry a conditional field's value into the next record", async () => {
    const { wrapper, create } = await mountCreate(CONDITIONAL_FORM)

    // Record 1: reveal the branch, fill it, then hide it again so the value is
    // held in the retention map rather than in formData.
    await wrapper.find('#field-flag').setValue(true)
    await flushPromises()
    await fill(wrapper, 'detail', 'first-secret')
    await wrapper.find('#field-flag').setValue(false)
    await flushPromises()

    await fill(wrapper, 'title', 'first')
    await clickAddAnother(wrapper)

    // Record 2: reveal the same branch. It must come up empty.
    await wrapper.find('#field-flag').setValue(true)
    await flushPromises()
    expect(valueOf(wrapper, 'detail')).toBe('')

    await fill(wrapper, 'title', 'second')
    await clickAddAnother(wrapper)

    const second = create.mock.calls[1][1] as { properties: Record<string, unknown> }
    expect(second.properties.detail).not.toBe('first-secret')
  })

  // AC-4d / RR-VVLNSU. The duplicate-link regression. An incoming-direction
  // RelationPicker keeps its selection in its own `incomingValue` ref and
  // `effectiveValue` ignores the `value` prop entirely for that direction, so
  // clearing `relations` in the reset does NOT reach it. Its record-1 peers
  // would then re-emit as pure additions (diffed against an empty
  // `incomingOriginal`) and be written to record 2 as duplicate links.
  //
  // The remount is the fix, driven by `saveGeneration` in the widget's `:key`.
  // Note `saveGeneration` was declared but never incremented before this
  // ticket (RR-ICL8CW), so this is what makes it load-bearing.
  it('remounts relation widgets so an incoming picker cannot keep its selection', async () => {
    const { wrapper } = await mountCreate(INCOMING_FORM)
    const afterMount = pickerMounts
    expect(afterMount).toBeGreaterThan(0)

    await fill(wrapper, 'title', 'first')
    await clickAddAnother(wrapper)

    expect(pickerMounts).toBeGreaterThan(afterMount)
  })

  // A kept RELATION must survive the reset AND still be sendable. The reset
  // clears `pickerTypes` (the id -> type map the payload reshape needs), so
  // re-applying the ids alone leaves reshapeLegacyToModern unable to resolve a
  // type: it returns null and handleSubmit ABORTS with "unknown types, reload
  // the form". The DOM still shows the peer, so this looks like it worked
  // right up until the second save silently fails and the user loses record 2.
  it('can still save a kept outgoing relation on the next record', async () => {
    const { wrapper, create } = await mountCreate(KEPT_REL_FORM)
    const ui = useUIStore()
    const uiError = vi.spyOn(ui, 'error')

    await fill(wrapper, 'title', 'first')
    await wrapper.find('.stub-picker').trigger('click')
    await flushPromises()
    await clickAddAnother(wrapper)

    await fill(wrapper, 'title', 'second')
    await clickAddAnother(wrapper)

    expect(uiError).not.toHaveBeenCalled()
    expect(create).toHaveBeenCalledTimes(2)
    const second = create.mock.calls[1][1] as { relations: Record<string, unknown> }
    expect(second.relations.blocks).toEqual({ data: [{ type: 'task', id: PEER_ID }] })
  })

  // An incoming picker's selection is latched by DynamicForm into
  // `pendingCardChanges`, NOT into `relations` — so clearing `relations` and
  // remounting the widget does not reach it. The map must be cleared too, or
  // record N's incoming relation is written to record N+1 with the user having
  // touched nothing.
  it('does not carry an unkept incoming relation into the next record', async () => {
    const { wrapper, create } = await mountCreate(INCOMING_FORM)

    await fill(wrapper, 'title', 'first')
    await wrapper.find('.stub-picker').trigger('click')
    await flushPromises()
    await clickAddAnother(wrapper)

    // Record 2: the user touches no relation at all.
    await fill(wrapper, 'title', 'second')
    await clickAddAnother(wrapper)

    const first = create.mock.calls[0][1] as { relations: Record<string, unknown> }
    const second = create.mock.calls[1][1] as { relations: Record<string, unknown> }
    expect(Object.keys(first.relations).length).toBeGreaterThan(0)
    expect(second.relations).toEqual({})
  })

  // The primary Create must keep its original contract untouched.
  it('leaves the primary Create navigating as before', async () => {
    const { wrapper } = await mountCreate()
    await fill(wrapper, 'title', 'go')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(push).toHaveBeenCalledTimes(1)
  })
})
