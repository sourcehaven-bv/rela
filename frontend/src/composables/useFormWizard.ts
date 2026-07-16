import { computed, ref, watch, type Ref, type ComputedRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { parse, type Program, type Bindings } from '@/utils/conditions'
import type { FormConfig, FormFieldOrRelation, FormStep } from '@/types'

/**
 * useFormWizard — the multi-step (wizard) layer for DynamicForm.
 *
 * A form config is a wizard when it declares `steps`. This composable owns the
 * wizard's derived state so DynamicForm stays a single-page renderer with a
 * thin wizard branch:
 *
 * - which steps are visible (each step's `visible_when` evaluated against the
 *   live form values),
 * - which fields inside the current step are visible (`visible_when`),
 * - the current step index, kept in the `?step=N` URL query (refresh/deep-link
 *   safe, `router.replace` so next/back don't spam history),
 * - `isFieldRequired` (authored `required` OR `required_when` true),
 * - `activeProperties` — the property keys under currently-visible steps, so
 *   the caller can drop hidden-branch values from the submitted payload.
 *
 * Conditions are evaluated with the shared engine (`utils/conditions`). A
 * malformed condition is a config bug: `compileCond` catches the parse throw,
 * warns once, and treats it as "always false" so a broken `visible_when` hides
 * its target rather than crashing the form. Runtime eval never throws.
 */

/** Compile a condition once, tolerating a malformed expression (config bug). */
function compileCond(expr: string | undefined): Program | null {
  if (!expr || expr.trim() === '') return null
  try {
    return parse(expr)
  } catch (err) {
    console.warn(`[wizard] ignoring invalid condition ${JSON.stringify(expr)}:`, err)
    return null
  }
}

function stepFields(step: FormStep): FormFieldOrRelation[] {
  const fields = (step.fields || []) as FormFieldOrRelation[]
  const relations = (step.relations || []) as FormFieldOrRelation[]
  return [...fields, ...relations]
}

export interface FormWizard {
  isWizard: ComputedRef<boolean>
  /** Steps whose `visible_when` currently holds, in authored order. */
  visibleSteps: ComputedRef<FormStep[]>
  /** Clamped index into visibleSteps. */
  currentStep: Ref<number>
  currentStepDef: ComputedRef<FormStep | undefined>
  isFirstStep: ComputedRef<boolean>
  isLastStep: ComputedRef<boolean>
  /** Visible fields of a given step (honors per-field `visible_when`). */
  visibleFieldsOf: (step: FormStep) => FormFieldOrRelation[]
  /** True if the field is required now (authored `required` OR `required_when`). */
  isFieldRequired: (field: FormFieldOrRelation) => boolean
  /** Property keys under all currently-visible steps (for payload pruning). */
  activeProperties: ComputedRef<Set<string>>
  next: () => void
  back: () => void
  goTo: (index: number) => void
}

export function useFormWizard(
  formConfig: Ref<FormConfig | undefined> | ComputedRef<FormConfig | undefined>,
  // A getter, not a computed: it is invoked inside each derived computed so the
  // reactive reads of the underlying form values happen in that computed's
  // tracking scope. A cached computed would return a stable object whose
  // property reads are not re-tracked, so visibility would never update as the
  // user types (the `form.done == true` reveal would go stale).
  getBindings: () => Bindings
): FormWizard {
  const route = useRoute()
  const router = useRouter()

  const isWizard = computed(() => (formConfig.value?.steps?.length ?? 0) > 0)

  // Evaluate a condition against the live bindings. The engine memoizes the
  // compiled program by source, so re-evaluating each render is cheap. An
  // absent/empty condition means "no condition" → true; a present-but-malformed
  // one was already warned about in compileCond and reads as false.
  const evalCond = (expr: string | undefined): boolean => {
    const prog = compileCond(expr)
    if (!prog) return !expr || expr.trim() === ''
    return prog.eval(getBindings())
  }

  const visibleSteps = computed<FormStep[]>(() => {
    const steps = formConfig.value?.steps
    if (!steps?.length) return []
    return steps.filter((s) => evalCond(s.visible_when))
  })

  const currentStep = ref(0)

  const currentStepDef = computed<FormStep | undefined>(() => visibleSteps.value[currentStep.value])
  const isFirstStep = computed(() => currentStep.value <= 0)
  const isLastStep = computed(() => currentStep.value >= visibleSteps.value.length - 1)

  function visibleFieldsOf(step: FormStep): FormFieldOrRelation[] {
    return stepFields(step).filter((f) => evalCond(f.visible_when))
  }

  function isFieldRequired(field: FormFieldOrRelation): boolean {
    if (field.required) return true
    if (field.required_when) return evalCond(field.required_when)
    return false
  }

  const activeProperties = computed<Set<string>>(() => {
    const keys = new Set<string>()
    for (const step of visibleSteps.value) {
      for (const f of visibleFieldsOf(step)) {
        if (f.property) keys.add(f.property)
      }
    }
    return keys
  })

  // --- URL sync (`?step=N`), mirroring useUrlFilterSync's seed/replace/echo. ---

  function clamp(i: number): number {
    const max = visibleSteps.value.length - 1
    if (!Number.isFinite(i) || i < 0) return 0
    return Math.min(i, Math.max(0, max))
  }

  // Seed synchronously from the URL so a refresh/deep-link lands on the step.
  if (isWizard.value) {
    const raw = route.query.step
    const parsed = typeof raw === 'string' ? parseInt(raw, 10) : NaN
    currentStep.value = clamp(parsed)
  }

  let lastWritten = ''
  function writeStep(i: number): void {
    const value = String(i)
    lastWritten = value
    const query = { ...route.query, step: value }
    router.replace({ query })
  }

  // External navigation (back/forward, deep link) restores the step; our own
  // writes are ignored via the echo guard.
  watch(
    () => route.query.step,
    (raw) => {
      const incoming = typeof raw === 'string' ? raw : ''
      if (incoming === lastWritten) return
      const parsed = incoming ? parseInt(incoming, 10) : 0
      currentStep.value = clamp(parsed)
    }
  )

  // If the visible-step set shrinks under the current index (an earlier answer
  // hid a later step), clamp back into range.
  watch(visibleSteps, () => {
    const clamped = clamp(currentStep.value)
    if (clamped !== currentStep.value) currentStep.value = clamped
  })

  function goTo(index: number): void {
    const i = clamp(index)
    currentStep.value = i
    writeStep(i)
  }

  function next(): void {
    if (!isLastStep.value) goTo(currentStep.value + 1)
  }

  function back(): void {
    if (!isFirstStep.value) goTo(currentStep.value - 1)
  }

  return {
    isWizard,
    visibleSteps,
    currentStep,
    currentStepDef,
    isFirstStep,
    isLastStep,
    visibleFieldsOf,
    isFieldRequired,
    activeProperties,
    next,
    back,
    goTo,
  }
}
