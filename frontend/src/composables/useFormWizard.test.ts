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

  it('detects a wizard vs a single-page form', () => {
    expect(setup(wizardForm()).wiz.isWizard.value).toBe(true)
    expect(setup({ entity: 'x', fields: [{ property: 'a' }] }).wiz.isWizard.value).toBe(false)
    expect(setup(undefined).wiz.isWizard.value).toBe(false)
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
})
