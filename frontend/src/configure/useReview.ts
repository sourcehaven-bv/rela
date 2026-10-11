import { computed, ref } from 'vue'
import {
  finishConfigureMigration,
  previewConfigure,
  saveConfigure,
  type ConfigMigrationStep,
  type ConfigProblem,
} from '@/api/configure'
import { ApiError, getErrorMessage } from '@/api/errors'
import { useConfigDraftStore } from '@/stores/configDraft'
import {
  beginConfigSave,
  endConfigSave,
  isConfigIncomplete,
  markConfigSaved,
  setConfigIncomplete,
} from './saveGate'
import { choiceListModel, entityTypeLabel, propertiesOf, typeLabel } from './models'
import type { ChangeKind } from './changes'

/**
 * Whether a refusal means the configuration changed under the draft. Other
 * 409s (`busy`, `migration_not_started`) and a 503 are passing states: the
 * server says why, and the same draft can be sent again.
 */
function isVersionConflict(err: unknown): boolean {
  if (!(err instanceof ApiError) || err.status !== 409) return false
  const type = err.problem?.type
  return !type || type.endsWith('/conflict')
}

/**
 * The review step's behaviour: check the draft, ask what it cannot decide,
 * save it. The drawer only draws this.
 */
export function useReview() {
  const draft = useConfigDraftStore()
  const checking = ref(false)
  const saving = ref(false)
  /** A failure that is not a list of problems: a conflict, a refused request. */
  const failure = ref('')
  /** The save went through but its migration did not finish. Survives a reload. */
  const incomplete = ref(isConfigIncomplete())
  /** The configuration changed under the draft (409 conflict). */
  const conflict = ref(false)
  let checkRun = 0

  async function check(): Promise<void> {
    if (!draft.hasDraft) return
    const run = ++checkRun
    // An edit while the check is out makes its answer one about an older draft.
    const revision = draft.revision
    checking.value = true
    failure.value = ''
    try {
      const result = await previewConfigure(draft.body())
      if (run !== checkRun || revision !== draft.revision) return
      conflict.value = false
      draft.setResult(result)
    } catch (err) {
      if (run !== checkRun) return
      conflict.value = isVersionConflict(err)
      failure.value = getErrorMessage(err, 'The draft could not be checked.')
    } finally {
      if (run === checkRun) checking.value = false
    }
  }

  function reload(): void {
    window.location.reload()
  }

  async function save(): Promise<void> {
    saving.value = true
    failure.value = ''
    beginConfigSave()
    let reloading = false
    const revision = draft.revision
    try {
      const result = await saveConfigure(draft.body())
      if (!result.saved) {
        if (revision === draft.revision) draft.setResult(result)
        return
      }
      draft.discardAll()
      if (result.incomplete) {
        setConfigIncomplete(true)
        draft.reviewOpen = true
        incomplete.value = true
        return
      }
      markConfigSaved()
      reloading = true
      reload()
    } catch (err) {
      conflict.value = isVersionConflict(err)
      failure.value = getErrorMessage(err, 'The draft could not be saved.')
    } finally {
      saving.value = false
      if (!reloading) endConfigSave()
    }
  }

  async function finishMigration(): Promise<void> {
    saving.value = true
    failure.value = ''
    beginConfigSave()
    let reloading = false
    try {
      await finishConfigureMigration()
      setConfigIncomplete(false)
      incomplete.value = false
      markConfigSaved()
      reloading = true
      reload()
    } catch (err) {
      failure.value = getErrorMessage(err, 'The migration could not be finished.')
    } finally {
      saving.value = false
      if (!reloading) endConfigSave()
    }
  }

  const problems = computed(() => draft.problems)
  const valueProblems = computed(() => problems.value.filter((p) => p.code === 'value_in_use'))
  const otherProblems = computed(() => problems.value.filter((p) => p.code !== 'value_in_use'))
  const migration = computed(() => draft.result?.migration)
  const records = computed(() =>
    (migration.value?.steps ?? []).reduce((n, s) => Math.max(n, s.count), 0)
  )
  const checked = computed(() => draft.result !== null)
  const canSave = computed(
    () =>
      draft.hasDraft &&
      checked.value &&
      !checking.value &&
      problems.value.length === 0 &&
      !draft.stale &&
      !conflict.value
  )

  return {
    checking,
    saving,
    failure,
    incomplete,
    conflict,
    check,
    save,
    finishMigration,
    problems,
    valueProblems,
    otherProblems,
    migration,
    records,
    checked,
    canSave,
  }
}

/** The options records holding a removed value can move to. */
export function mappingOptions(
  schema: Parameters<typeof propertiesOf>[0],
  problem: ConfigProblem
): { value: string; label: string }[] {
  const property = propertiesOf(schema, problem.entity_type ?? '').find(
    (p) => p.name === problem.property
  )
  if (!property) return []
  const values = property.choiceList
    ? (choiceListModel(schema, undefined, property.choiceList)?.options ?? []).map((o) => o.value)
    : property.inlineValues
  return values.filter((v) => v !== problem.value).map((v) => ({ value: v, label: v }))
}

/** A value-in-use problem as the question the review asks. */
export function mappingQuestion(
  schema: Parameters<typeof propertiesOf>[0],
  p: ConfigProblem
): string {
  const type = entityTypeLabel(schema, p.entity_type ?? '')
  const many = (p.count ?? 0) === 1 ? type : `${type} records`
  return `${p.count ?? 'Some'} ${many} with ${p.property ?? ''} ${p.value ?? ''} move to`
}

export interface StepView {
  kind: ChangeKind
  label: string
  detail?: string
  before?: string
  after?: string
}

/** A migration step in the review's words. */
export function describeStep(
  schema: Parameters<typeof propertiesOf>[0],
  s: ConfigMigrationStep
): StepView {
  const type = entityTypeLabel(schema, s.entity_type)
  const records = `${s.count.toLocaleString('en')} ${s.count === 1 ? type : `${type} records`}`
  switch (s.kind) {
    case 'rename_property':
      return {
        kind: 'changed',
        label: `Move values to the new name on ${records}`,
        before: s.from ?? '',
        after: s.to ?? s.property,
      }
    case 'map_values':
      return {
        kind: 'changed',
        label: `${s.property} of ${records}`,
        before: s.from ?? '',
        after: s.to ?? '',
      }
    case 'convert':
      return {
        kind: 'changed',
        label: `Convert ${s.property} on ${records}`,
        after: s.to ? typeLabel(schema, s.to) : undefined,
      }
    default:
      return { kind: 'changed', label: `${s.kind} on ${records}` }
  }
}
