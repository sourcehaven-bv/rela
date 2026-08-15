// Component-level tests for the propose/commit seam (TKT-7S5735).
//
// Why this file exists, and why it mounts the real component:
//
// Every BUG-FB0LN8 bug lived in DynamicForm's ORCHESTRATION — the interaction
// between a widget edit, the visibility watcher, the retention map and the
// autosave debounce. None were caught by ~1600 unit tests (which drive
// composables in isolation) or by 239 e2e tests. Three fixes each passed their
// tests and each then failed in manual use.
//
// So these tests drive the real path: a real widget's `update:model-value`
// event, through FieldRenderer and FormFieldList, into `proposeChange`, and
// out to an asserted `entitiesStore.update` call. Spying the STORE, not
// `fetch`: writes go through the Pinia store (useAutoSave.ts) and the api
// client is not reachable from here (BUG-2OXEW0's global guard blocks real
// HTTP anyway).

import { describe, it, expect, beforeEach, afterEach, vi, type Mock } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { useSchemaStore, useEntitiesStore } from '@/stores'
import DynamicForm from './DynamicForm.vue'
import type { Entity } from '@/types'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/form/opportunity-form' }),
  onBeforeRouteLeave: vi.fn(),
}))

vi.mock('@/api', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@/api')
  return { ...actual, getTemplates: vi.fn().mockResolvedValue([]) }
})

// The reporter's configuration, reduced: two deadlines conditioned on a
// procurement-route enum. `inkooproute` is the trigger; toggling it away from
// 'aanbesteding' hides both deadlines.
const ENTITY_TYPE = {
  name: 'opportunity',
  label: 'Opportunity',
  id_type: 'short',
  properties: {
    inkooproute: { type: 'string' },
    inschrijfdeadline: { type: 'string' },
    vragenronde_deadline: { type: 'string' },
  },
}

function formConfig(clearWhenHidden?: string) {
  return {
    id: 'opportunity-form',
    entity: 'opportunity',
    fields: [
      { property: 'inkooproute' },
      {
        property: 'inschrijfdeadline',
        visible_when: "form.inkooproute == 'aanbesteding'",
        ...(clearWhenHidden ? { clear_when_hidden: clearWhenHidden } : {}),
      },
      {
        property: 'vragenronde_deadline',
        visible_when: "form.inkooproute == 'aanbesteding'",
      },
    ],
  }
}

const STORED = {
  inkooproute: 'aanbesteding',
  inschrijfdeadline: '2026-09-15',
  vragenronde_deadline: '2026-08-20',
}

const mounted: VueWrapper[] = []

afterEach(() => {
  const wrappers = mounted.splice(0)
  wrappers.forEach((w) => {
    try {
      w.unmount()
    } catch {
      /* already torn down */
    }
  })
  vi.useRealTimers()
})

async function mountEdit(cfg: object) {
  const schema = useSchemaStore()
  schema.forms.set('opportunity-form', cfg as never)
  schema.entityTypes.set('opportunity', ENTITY_TYPE as never)
  schema.loaded = true

  const entities = useEntitiesStore()
  const entity: Entity = {
    id: 'OPP-1',
    type: 'opportunity',
    properties: { ...STORED },
    _actions: { update: true },
    _fields: {},
    _redacted: [],
  }
  vi.spyOn(entities, 'fetchEntity').mockResolvedValue(entity)
  const update = vi
    .spyOn(entities, 'update')
    .mockResolvedValue({ ...entity, warnings: [] } as Entity) as unknown as Mock

  const wrapper = mount(DynamicForm, {
    props: { formId: 'opportunity-form', entityId: 'OPP-1' },
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
      mocks: {
        $router: { push: vi.fn(), replace: vi.fn() },
        $route: { query: {}, params: {}, path: '/form/opportunity-form' },
      },
    },
  })
  mounted.push(wrapper)
  await flushPromises()
  return { wrapper, update }
}

/** Drive a REAL widget edit, the way a user does. */
async function typeInto(wrapper: VueWrapper, property: string, value: string) {
  const input = wrapper.find(`#field-${property}`)
  expect(input.exists(), `#field-${property} should render`).toBe(true)
  await input.setValue(value)
  await flushPromises()
}

const rendered = (wrapper: VueWrapper, property: string) =>
  wrapper.find(`#field-${property}`).exists()

describe('DynamicForm propose/commit seam', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  it('renders the conditional fields when the condition holds', async () => {
    const { wrapper } = await mountEdit(formConfig())
    expect(rendered(wrapper, 'inschrijfdeadline')).toBe(true)
    expect(rendered(wrapper, 'vragenronde_deadline')).toBe(true)
  })

  // BUG-FB0LN8, the regression this whole arc exists to prevent. Default
  // policy is `no`: hiding is presentation, so nothing is written.
  it('writes NO unset for a hidden field under the default policy', async () => {
    const { wrapper, update } = await mountEdit(formConfig())
    vi.useFakeTimers()

    await typeInto(wrapper, 'inkooproute', 'onderhands')
    expect(rendered(wrapper, 'inschrijfdeadline')).toBe(false) // it hid

    await vi.advanceTimersByTimeAsync(2000) // well past the 800ms debounce

    const unsets = update.mock.calls.flatMap(
      (c) => (c[2] as { properties_unset?: string[] })?.properties_unset ?? []
    )
    expect(unsets).not.toContain('inschrijfdeadline')
    expect(unsets).not.toContain('vragenronde_deadline')
  })

  // The reveal half. Hide → reveal must be lossless WITHOUT a server round
  // trip: the value was retained client-side, outside formData.
  it('restores the stored value when the branch is revealed again', async () => {
    const { wrapper } = await mountEdit(formConfig())

    await typeInto(wrapper, 'inkooproute', 'onderhands')
    expect(rendered(wrapper, 'inschrijfdeadline')).toBe(false)

    await typeInto(wrapper, 'inkooproute', 'aanbesteding')
    expect(rendered(wrapper, 'inschrijfdeadline')).toBe(true)
    expect(
      (wrapper.find('#field-inschrijfdeadline').element as HTMLInputElement).value
    ).toBe('2026-09-15')
  })

  // The opt-in destructive policy still works — this is the escape hatch for
  // operators who genuinely want the old behaviour.
  it('clears a hidden field when clear_when_hidden is yes', async () => {
    const { wrapper, update } = await mountEdit(formConfig('yes'))
    vi.useFakeTimers()

    await typeInto(wrapper, 'inkooproute', 'onderhands')
    await vi.advanceTimersByTimeAsync(2000)

    const unsets = update.mock.calls.flatMap(
      (c) => (c[2] as { properties_unset?: string[] })?.properties_unset ?? []
    )
    expect(unsets).toContain('inschrijfdeadline')
    // The sibling keeps its default policy and must be untouched.
    expect(unsets).not.toContain('vragenronde_deadline')
  })

  // AC2: the payoff of merging. An accepted clear decision is a set of changes
  // the user approved TOGETHER — the trigger's new value and the unset of what
  // it hid. Emitting them as two requests leaves a window in which the entity
  // holds a state nobody approved, and if the second fails that state persists.
  it('commits an accepted clear as ONE atomic patch', async () => {
    const { wrapper, update } = await mountEdit(formConfig('yes'))
    vi.useFakeTimers()

    await typeInto(wrapper, 'inkooproute', 'onderhands')
    await vi.advanceTimersByTimeAsync(2000)

    expect(update).toHaveBeenCalledTimes(1)
    const patch = update.mock.calls[0][2] as {
      properties?: Record<string, unknown>
      properties_unset?: string[]
    }
    expect(patch.properties?.inkooproute).toBe('onderhands')
    expect(patch.properties_unset).toContain('inschrijfdeadline')
  })

  // The trigger's own edit must still commit normally — the seam must not
  // swallow the very change the user made.
  it('commits the triggering edit itself', async () => {
    const { wrapper, update } = await mountEdit(formConfig())
    vi.useFakeTimers()

    await typeInto(wrapper, 'inkooproute', 'onderhands')
    await vi.advanceTimersByTimeAsync(2000)

    const wrote = update.mock.calls.some(
      (c) => (c[2] as { properties?: Record<string, unknown> })?.properties?.inkooproute === 'onderhands'
    )
    expect(wrote).toBe(true)
  })
})
