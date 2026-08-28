import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { reactive, ref, nextTick, effectScope, type EffectScope } from 'vue'
import type { LocationQuery } from 'vue-router'
import { useFormWizard } from './useFormWizard'
import type { Bindings } from '@/utils/conditions'
import type { FormConfig } from '@/types'

const mockRoute = reactive<{ query: LocationQuery }>({ query: {} })
const mockReplace = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => mockRoute,
  useRouter: () => ({ replace: mockReplace }),
}))

mockReplace.mockImplementation(({ query }: { query: LocationQuery }) => {
  mockRoute.query = query
})

let scope: EffectScope

function wizardForm(): FormConfig {
  return {
    entity: 'processing-record',
    title: 'New record',
    steps: [
      { title: 'Controller', fields: [{ property: 'name', required: true }] },
      {
        title: 'Processor',
        visible_when: 'form.has_processors == true',
        fields: [{ property: 'processor_name', required_when: 'form.has_processors == true' }],
      },
      { title: 'Publish', fields: [{ property: 'published' }] },
    ],
  }
}

describe('useFormWizard', () => {
  beforeEach(() => {
    mockRoute.query = {}
    mockReplace.mockClear()
    scope = effectScope()
  })
  afterEach(() => scope.stop())

  function setup(cfg: FormConfig | undefined, form: Record<string, unknown> = {}) {
    const config = ref(cfg)
    const formData = ref(form)
    const getBindings = (): Bindings => ({
      form: formData.value,
      entity: {},
      current_user: {},
    })
    const wiz = scope.run(() => useFormWizard(config, getBindings))!
    return { wiz, formData, config }
  }

  it('isMultiStep is true only for >1 authored step', () => {
    expect(setup(wizardForm()).wiz.isMultiStep.value).toBe(true) // 3 steps
    // A flat form (one implicit step) and a one-step form are NOT multi-step.
    expect(setup({ entity: 'x', fields: [{ property: 'a' }] }).wiz.isMultiStep.value).toBe(false)
    expect(
      setup({ entity: 'x', steps: [{ title: 'Only', fields: [{ property: 'a' }] }] }).wiz
        .isMultiStep.value
    ).toBe(false)
    expect(setup(undefined).wiz.isMultiStep.value).toBe(false)
  })

  it('a flat form is modelled as one visible step', () => {
    const { wiz } = setup({
      entity: 'x',
      fields: [{ property: 'a' }, { property: 'b' }],
      relations: [{ relation: 'r' }],
    })
    expect(wiz.visibleSteps.value.length).toBe(1)
    expect(
      wiz.visibleFieldsOf(wiz.visibleSteps.value[0]).map((f) => f.property || f.relation)
    ).toEqual(['a', 'b', 'r'])
  })

  it('filters visible steps by visible_when against form values', () => {
    const { wiz, formData } = setup(wizardForm(), { has_processors: false })
    // Processor step hidden when the toggle is off.
    expect(wiz.visibleSteps.value.map((s) => s.title)).toEqual(['Controller', 'Publish'])
    formData.value = { has_processors: true }
    expect(wiz.visibleSteps.value.map((s) => s.title)).toEqual([
      'Controller',
      'Processor',
      'Publish',
    ])
  })

  it('isFieldRequired honors authored required and required_when', () => {
    const { wiz, formData } = setup(wizardForm(), { has_processors: true })
    const controller = wiz.visibleSteps.value[0]
    const processor = wiz.visibleSteps.value[1]
    expect(wiz.isFieldRequired(controller.fields![0])).toBe(true) // authored required
    expect(wiz.isFieldRequired(processor.fields![0])).toBe(true) // required_when true
    formData.value = { has_processors: false }
    // processor step is now hidden; but the field's required_when is false anyway
    const proc = wizardForm().steps![1].fields![0]
    expect(wiz.isFieldRequired(proc)).toBe(false)
  })

  it('activeProperties excludes hidden-branch fields', () => {
    const { wiz, formData } = setup(wizardForm(), { has_processors: false })
    expect([...wiz.activeProperties.value].sort()).toEqual(['name', 'published'])
    formData.value = { has_processors: true }
    expect([...wiz.activeProperties.value].sort()).toEqual(['name', 'processor_name', 'published'])
  })

  it('managedProperties covers every step field regardless of visibility', () => {
    // Independent of current form values — includes the hidden branch's field.
    const { wiz } = setup(wizardForm(), { has_processors: false })
    expect([...wiz.managedProperties.value].sort()).toEqual(['name', 'processor_name', 'published'])
  })

  it('active/managedRelations track relations under conditional branches', () => {
    const cfg: FormConfig = {
      entity: 'x',
      steps: [
        { title: 'A', relations: [{ relation: 'always_rel' }] },
        {
          title: 'B',
          visible_when: 'form.show == true',
          relations: [{ relation: 'cond_rel' }],
        },
      ],
    }
    const { wiz, formData } = setup(cfg, { show: false })
    // managed = every relation named; active = only under a visible step.
    expect([...wiz.managedRelations.value].sort()).toEqual(['always_rel', 'cond_rel'])
    expect([...wiz.activeRelations.value].sort()).toEqual(['always_rel'])
    formData.value = { show: true }
    expect([...wiz.activeRelations.value].sort()).toEqual(['always_rel', 'cond_rel'])
  })

  it('visibleStepIndexForProperty maps a property to its visible step', () => {
    const { wiz, formData } = setup(wizardForm(), { has_processors: true })
    expect(wiz.visibleStepIndexForProperty('name')).toBe(0)
    expect(wiz.visibleStepIndexForProperty('processor_name')).toBe(1)
    expect(wiz.visibleStepIndexForProperty('published')).toBe(2)
    expect(wiz.visibleStepIndexForProperty('nope')).toBe(-1)
    // When the processor step is hidden, its field maps to no visible step.
    formData.value = { has_processors: false }
    expect(wiz.visibleStepIndexForProperty('processor_name')).toBe(-1)
    expect(wiz.visibleStepIndexForProperty('published')).toBe(1) // shifted up
  })

  it('visibleFieldsOf honors per-field visible_when', () => {
    const cfg: FormConfig = {
      entity: 'x',
      steps: [
        {
          title: 'S',
          fields: [
            { property: 'always' },
            { property: 'maybe', visible_when: "form.mode == 'advanced'" },
          ],
        },
      ],
    }
    const { wiz, formData } = setup(cfg, { mode: 'basic' })
    expect(wiz.visibleFieldsOf(cfg.steps![0]).map((f) => f.property)).toEqual(['always'])
    formData.value = { mode: 'advanced' }
    expect(wiz.visibleFieldsOf(cfg.steps![0]).map((f) => f.property)).toEqual(['always', 'maybe'])
  })

  describe('navigation + URL sync', () => {
    it('seeds currentStep from ?step= on setup', () => {
      mockRoute.query = { step: '2' }
      const { wiz } = setup(wizardForm(), { has_processors: true })
      expect(wiz.currentStep.value).toBe(2)
    })

    it('seedFromUrl re-seeds after loaded data reveals an earlier step (RR-TXMU6)', () => {
      // Deep link ?step=2. At construction `has_processors` is absent, so the
      // conditional Processor step is hidden -> visibleSteps has 2 -> clamp(2)=1.
      mockRoute.query = { step: '2' }
      const { wiz, formData } = setup(wizardForm(), {})
      expect(wiz.currentStep.value).toBe(1) // clamped against the 2 visible steps
      // Loaded/toggled data reveals the earlier step; a re-seed lands on 2.
      formData.value = { has_processors: true }
      wiz.seedFromUrl()
      expect(wiz.currentStep.value).toBe(2)
    })

    it('clamps an out-of-range ?step= to the first step', () => {
      mockRoute.query = { step: '99' }
      const { wiz } = setup(wizardForm(), { has_processors: true })
      expect(wiz.currentStep.value).toBe(2) // clamped to last visible index (3 steps)
    })

    it('clamps a non-numeric ?step= to 0', () => {
      mockRoute.query = { step: 'abc' }
      const { wiz } = setup(wizardForm(), { has_processors: true })
      expect(wiz.currentStep.value).toBe(0)
    })

    it('next/back move and write ?step= via router.replace', () => {
      const { wiz } = setup(wizardForm(), { has_processors: true })
      expect(wiz.isFirstStep.value).toBe(true)
      wiz.next()
      expect(wiz.currentStep.value).toBe(1)
      expect(mockRoute.query.step).toBe('1')
      wiz.back()
      expect(wiz.currentStep.value).toBe(0)
      expect(mockRoute.query.step).toBe('0')
    })

    it('next does not advance past the last step', () => {
      mockRoute.query = { step: '2' }
      const { wiz } = setup(wizardForm(), { has_processors: true })
      expect(wiz.isLastStep.value).toBe(true)
      wiz.next()
      expect(wiz.currentStep.value).toBe(2)
    })

    it('clamps currentStep when the visible set shrinks under it', async () => {
      const { wiz, formData } = setup(wizardForm(), { has_processors: true })
      wiz.goTo(2) // Publish (index 2 of 3)
      expect(wiz.currentStep.value).toBe(2)
      // Hide the Processor step -> only 2 steps remain; index 2 is out of range.
      formData.value = { has_processors: false }
      await nextTick()
      expect(wiz.currentStep.value).toBe(1)
    })
  })

  it('treats a malformed condition as always-false + warns', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const cfg: FormConfig = {
      entity: 'x',
      steps: [
        { title: 'A', fields: [{ property: 'a' }] },
        { title: 'B', visible_when: 'form.x ==', fields: [{ property: 'b' }] }, // parse error
      ],
    }
    const { wiz } = setup(cfg, {})
    expect(wiz.visibleSteps.value.map((s) => s.title)).toEqual(['A'])
    expect(warn).toHaveBeenCalled()
    warn.mockRestore()
  })

  // TKT-7S5735: answer "what would be visible if the user made this edit?"
  // WITHOUT applying it. Visibility is a pure function of the form values, so
  // before this the only way to find out was to write formData first — which is
  // the propose/commit conflation BUG-FB0LN8 kept failing on.
  describe('activePropertiesFor (hypothetical evaluation)', () => {
    // The reporter's actual configuration, reduced: two deadline fields
    // conditioned on a procurement-route enum.
    function conditionalForm(): FormConfig {
      return {
        entity: 'opportunity',
        fields: [
          { property: 'inkooproute' },
          { property: 'vragenronde_deadline', visible_when: "form.inkooproute == 'aanbesteding'" },
          { property: 'inschrijfdeadline', visible_when: "form.inkooproute == 'aanbesteding'" },
        ],
      }
    }

    it('reports what would be visible after a hypothetical change', () => {
      const { wiz } = setup(conditionalForm(), { inkooproute: 'aanbesteding' })
      const after = wiz.activePropertiesFor({
        form: { inkooproute: 'onderhands' },
        entity: {},
        current_user: {},
      })
      expect([...after]).toEqual(['inkooproute'])
    })

    // The load-bearing property. If asking the question changed the answer,
    // the seam would be decorative.
    it('does not disturb live state', () => {
      const { wiz, formData } = setup(conditionalForm(), { inkooproute: 'aanbesteding' })
      const before = [...wiz.activeProperties.value]

      wiz.activePropertiesFor({
        form: { inkooproute: 'onderhands' },
        entity: {},
        current_user: {},
      })

      expect([...wiz.activeProperties.value]).toEqual(before)
      expect(formData.value).toEqual({ inkooproute: 'aanbesteding' })
    })

    it('reports a reveal as well as a hide', () => {
      const { wiz } = setup(conditionalForm(), { inkooproute: 'onderhands' })
      expect([...wiz.activeProperties.value]).toEqual(['inkooproute'])

      const after = wiz.activePropertiesFor({
        form: { inkooproute: 'aanbesteding' },
        entity: {},
        current_user: {},
      })
      expect([...after].sort()).toEqual([
        'inkooproute',
        'inschrijfdeadline',
        'vragenronde_deadline',
      ])
    })

    // A step's own visible_when reads the same bindings, so a proposal can hide
    // a whole step — not just fields within a still-visible one. This is why
    // the implementation re-filters steps rather than reusing `visibleSteps`.
    it('accounts for a step hidden by the proposal', () => {
      const { wiz } = setup(wizardForm(), { has_processors: true })
      expect(wiz.activeProperties.value.has('processor_name')).toBe(true)

      const after = wiz.activePropertiesFor({
        form: { has_processors: false },
        entity: {},
        current_user: {},
      })
      expect(after.has('processor_name')).toBe(false)
      expect(after.has('name')).toBe(true) // unconditional step survives
    })

    it('agrees with activeProperties when given the live bindings', () => {
      const { wiz } = setup(conditionalForm(), { inkooproute: 'aanbesteding' })
      const same = wiz.activePropertiesFor({
        form: { inkooproute: 'aanbesteding' },
        entity: {},
        current_user: {},
      })
      expect([...same].sort()).toEqual([...wiz.activeProperties.value].sort())
    })
  })
})
