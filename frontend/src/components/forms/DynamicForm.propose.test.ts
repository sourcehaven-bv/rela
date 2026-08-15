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


// Capture the registered leave-guard so a test can invoke it the way the
// router would, without standing up a real router.
const leaveGuards: Array<() => unknown> = []
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, params: {}, path: '/form/opportunity-form' }),
  onBeforeRouteLeave: (fn: () => unknown) => leaveGuards.push(fn),
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

// `DynamicForm` destructures `confirm` at setup, so the module has to be
// mocked before mount — a post-mount spyOn would never be seen.
const confirmMock = vi.fn(async () => true)
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: confirmMock }),
}))

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

async function mountEdit(
  cfg: object,
  entityOpts: { properties?: Record<string, unknown>; redacted?: string[] } = {}
) {
  const schema = useSchemaStore()
  schema.forms.set('opportunity-form', cfg as never)
  schema.entityTypes.set('opportunity', ENTITY_TYPE as never)
  schema.loaded = true

  const entities = useEntitiesStore()
  const entity: Entity = {
    id: 'OPP-1',
    type: 'opportunity',
    properties: entityOpts.properties ?? { ...STORED },
    _actions: { update: true },
    _fields: {},
    _redacted: entityOpts.redacted ?? [],
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

  // clear_when_hidden: confirm, through the real widget path. This is the
  // feature four previous attempts failed to ship: each passed its unit tests
  // and then misbehaved in a browser, because the decision happened after the
  // write. These drive the actual dialog.
  describe('clear_when_hidden: confirm', () => {
    beforeEach(() => {
      confirmMock.mockReset()
      confirmMock.mockResolvedValue(true)
    })

    it('declining leaves the trigger AND the hidden field untouched', async () => {
      confirmMock.mockResolvedValue(false)
      const { wrapper, update } = await mountEdit(formConfig('confirm'))

      await typeInto(wrapper, 'inkooproute', 'onderhands')
      await flushPromises()

      // The trigger never changed, so the deadlines never hid.
      expect(rendered(wrapper, 'inschrijfdeadline')).toBe(true)
      expect(
        (wrapper.find('#field-inschrijfdeadline').element as HTMLInputElement).value
      ).toBe('2026-09-15')
      expect(update).not.toHaveBeenCalled()
    })

    // THE FIFTH FAILURE MODE, found by writing this test.
    //
    // Declining leaves `formData` untouched — correct — but the widget had
    // already moved its OWN DOM when the user interacted with it. Because the
    // bound `model-value` never changed, Vue has nothing to patch back, so the
    // control kept displaying the declined value while the form held the old
    // one. The user sees 'onderhands' on a form that means 'aanbesteding':
    // exactly the class of mismatch that killed the four earlier attempts.
    it('snaps the widget back to the previous value on decline', async () => {
      confirmMock.mockResolvedValue(false)
      const { wrapper } = await mountEdit(formConfig('confirm'))

      await typeInto(wrapper, 'inkooproute', 'onderhands')
      await flushPromises()

      expect(
        (wrapper.find('#field-inkooproute').element as HTMLInputElement).value
      ).toBe('aanbesteding')
    })

    it('approving commits the change and the clear together', async () => {
      confirmMock.mockResolvedValue(true)
      const { wrapper, update } = await mountEdit(formConfig('confirm'))
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

    // A field the principal cannot READ is absent from formData, so it is
    // indistinguishable from empty — which meant no dialog was shown AND it
    // was cleared anyway. BUG-FB0LN8's original symptom, resurrected through
    // the ACL path: data destroyed that the user never saw and never approved.
    it('never clears a redacted confirm field, and never asks about it', async () => {
      const { wrapper, update } = await mountEdit(formConfig('confirm'), {
        properties: { inkooproute: 'aanbesteding' }, // inschrijfdeadline withheld
        redacted: ['inschrijfdeadline'],
      })
      vi.useFakeTimers()

      await typeInto(wrapper, 'inkooproute', 'onderhands')
      await flushPromises()
      await vi.advanceTimersByTimeAsync(2000)

      expect(confirmMock).not.toHaveBeenCalled() // nothing displayable to ask about
      const unsets = update.mock.calls.flatMap(
        (c) => (c[2] as { properties_unset?: string[] })?.properties_unset ?? []
      )
      expect(unsets).not.toContain('inschrijfdeadline')
    })

    // useConfirm resolves its pending promise on APP-shell unmount, not ours,
    // so a route change with the dialog open would resume the awaited
    // continuation on a dead component and PATCH a form the user has left.
    it('does not write when the form unmounts while the dialog is open', async () => {
      let release!: (v: boolean) => void
      confirmMock.mockImplementation(() => new Promise<boolean>((r) => (release = r)))
      const { wrapper, update } = await mountEdit(formConfig('confirm'))
      vi.useFakeTimers()

      await typeInto(wrapper, 'inkooproute', 'onderhands')
      await flushPromises()
      wrapper.unmount()
      release(true) // user answers AFTER navigating away
      await flushPromises()
      await vi.advanceTimersByTimeAsync(2000)

      expect(update).not.toHaveBeenCalled()
    })

    // RR-YWIN6T. `useConfirm` is a singleton that hands a concurrent caller
    // the SAME in-flight promise. Without this fence the navigation guard
    // would receive the clear-dialog's answer: clicking "Clear" would also
    // answer "Leave anyway", navigating the user away from a decision they
    // were still making.
    it('blocks navigation while the clear dialog is open', async () => {
      let release!: (v: boolean) => void
      confirmMock.mockImplementation(() => new Promise<boolean>((r) => (release = r)))
      leaveGuards.length = 0
      const { wrapper } = await mountEdit(formConfig('confirm'))
      const guard = leaveGuards[leaveGuards.length - 1]

      await typeInto(wrapper, 'inkooproute', 'onderhands')
      await flushPromises()

      expect(await guard()).toBe(false) // dialog open → navigation refused

      release(false)
      await flushPromises()
    })

    // The debounce must not outrun the dialog. This is the exact failure from
    // the earlier attempt: thinking too long committed the change.
    it('emits nothing while the dialog is open, however long it stays open', async () => {
      const { wrapper, update } = await mountEdit(formConfig('confirm'))
      let release!: (v: boolean) => void
      confirmMock.mockImplementation(() => new Promise<boolean>((r) => (release = r)))
      vi.useFakeTimers()

      await typeInto(wrapper, 'inkooproute', 'onderhands')
      await vi.advanceTimersByTimeAsync(5000) // far beyond the 800ms debounce

      expect(update).not.toHaveBeenCalled() // still undecided → still nothing sent

      release(false)
      await flushPromises()
      expect(update).not.toHaveBeenCalled()
    })
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
