<script setup lang="ts">
/**
 * The step form of a waiting run (TKT-ZAXRXND): the subject properties the
 * state `writes`, rendered by the entity form's own field widgets, and the
 * Complete button.
 *
 * The form reads the subject through the gated entity GET, so the values it
 * starts from are the ones the reader may see. It sends only the names in
 * `writes` the user changed, so a value edited elsewhere meanwhile is not
 * overwritten with the one the form loaded. It sends the run generation it
 * was rendered at: a run that moved on meanwhile answers 409, and the card
 * reloads it (`stale`).
 *
 * A field is marked required when every change branch's condition reads
 * it. The mark is a hint: the form has `novalidate`, so the browser does
 * not hold the step back, and the server's conditions decide.
 *
 * Feedback stays at the fields and in the form, not in a toast
 * (BUG-53GP5F5). A refused write shows each failure at its field: a
 * schema validation rule at the first field of the form it reads. A
 * soft finding on a written value, such as a failed pattern, shows as a
 * warning at its field. A step taken while no condition of the next step
 * holds (`blocked`) says so above the button. The first flagged field gets
 * focus, and the fields stay on screen while the values reload, so the
 * form does not move away from the reader.
 */
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import FieldRenderer from '@/components/forms/FieldRenderer.vue'
import { completeFlowStep, type FlowCompleted, type FlowRun } from '@/api/flows'
import { getEntity } from '@/api/entities'
import { ApiError, getErrorMessage, type ValidationError } from '@/api/errors'
import { useSchemaStore, useUIStore } from '@/stores'
import type { FormFieldOrRelation, PropertyDef } from '@/types'

const props = defineProps<{
  run: FlowRun
  /** The flow's subject type. */
  subjectType: string
}>()

const emit = defineEmits<{
  completed: [result: FlowCompleted]
  /** The run moved on before the step was sent. */
  stale: []
}>()

const { t } = useI18n()
const schema = useSchemaStore()
const ui = useUIStore()

const writes = computed(() => props.run.waiting?.writes ?? [])
const values = ref<Record<string, unknown>>({})
// The values as loaded, to send only what the user changed.
let loaded: Record<string, unknown> = {}
// Per field: the failures of a refused write, and the warnings of a
// written one. Editing a field clears its entries.
const errors = ref<Record<string, string>>({})
const warnings = ref<Record<string, string>>({})
// The failures of a refused write that name no field of the form.
const formErrors = ref<string[]>([])
// The last step was written, but no condition of the next step held.
const blocked = ref(false)
const saving = ref(false)
const loadError = ref('')
// The fields show once the subject's values are in, so an edit made while
// they load is not overwritten by them. A later reload keeps them on screen
// but read-only. loadSeq drops a superseded load.
const loading = ref(true)
const shownOnce = ref(false)
let loadSeq = 0
const formEl = ref<HTMLFormElement | null>(null)

interface StepField {
  name: string
  field: FormFieldOrRelation
  def?: PropertyDef
}

/** The written properties every change branch's condition reads. */
const required = computed(() => {
  const bs = (props.run.waiting?.branches ?? []).filter((b) => !b.fork?.length)
  if (bs.length === 0) return new Set<string>()
  const each = bs.map((b) => new Set(b.fields ?? []))
  return new Set([...each[0]].filter((name) => each.every((set) => set.has(name))))
})

const fields = computed<StepField[]>(() => {
  const props_ = schema.getEntityType(props.subjectType)?.properties ?? {}
  return writes.value.map((name) => {
    const def = props_[name]
    return {
      name,
      def: def && required.value.has(name) ? { ...def, required: true } : def,
      field: { property: name, label: def?.label || name },
    }
  })
})

/** Moves focus to the first field with feedback, and keeps the form in view. */
async function focusFeedback(extra: string[] = []) {
  await nextTick()
  const name = fields.value.find(
    (f) => errors.value[f.name] || warnings.value[f.name] || extra.includes(f.name)
  )?.name
  const box = name ? formEl.value?.querySelector(`[data-step-field="${name}"]`) : null
  const input = box?.querySelector<HTMLElement>('input, select, textarea, [tabindex]')
  input?.focus({ preventScroll: true })
  ;(box ?? formEl.value)?.scrollIntoView?.({ block: 'nearest' })
}

/** Reads the subject's current values of the written properties. */
async function loadValues() {
  const seq = ++loadSeq
  loadError.value = ''
  errors.value = {}
  warnings.value = {}
  formErrors.value = []
  const subject = props.run.subject
  if (!subject || writes.value.length === 0) {
    values.value = {}
    loaded = {}
    loading.value = false
    shownOnce.value = true
    return
  }
  loading.value = true
  try {
    const e = await getEntity(props.subjectType, subject)
    if (seq !== loadSeq) return
    const next: Record<string, unknown> = {}
    for (const name of writes.value) next[name] = e.properties?.[name]
    values.value = next
    loaded = { ...next }
  } catch (err) {
    if (seq !== loadSeq) return
    loadError.value = getErrorMessage(err, t('flows.step.loadFailed'))
  } finally {
    if (seq === loadSeq) {
      loading.value = false
      shownOnce.value = true
    }
  }
}

// Sources as a list of getters, compared one by one: one getter returning
// a fresh array would fire on every new run object, so the reload of the
// run after a blocked step would reload the values and drop the feedback.
watch(
  [
    () => props.run.id,
    () => props.run.generation,
    () => props.run.subject,
    () => writes.value.join(','),
  ],
  () => {
    blocked.value = false
    void loadValues()
  },
  { immediate: true }
)

function drop(map: Record<string, string>, name: string): Record<string, string> {
  if (!(name in map)) return map
  const next = { ...map }
  delete next[name]
  return next
}

function setValue(name: string, value: unknown) {
  values.value = { ...values.value, [name]: value }
  errors.value = drop(errors.value, name)
  warnings.value = drop(warnings.value, name)
}

/**
 * The field of the form a validation failure belongs at: the property it
 * names, or for a schema rule the first field of the form the rule reads.
 */
function errorField(e: ValidationError): string {
  const pointer = e.source?.pointer ?? ''
  const named = e.field || pointer.slice(pointer.lastIndexOf('/') + 1)
  if (named && writes.value.includes(named)) return named
  return writes.value.find((w) => e.fields?.includes(w)) ?? ''
}

/** Shows the failures of a refused write at their fields. */
function showRefusal(invalid: ValidationError[]) {
  const next: Record<string, string> = {}
  const rest: string[] = []
  for (const e of invalid) {
    const text = e.message || e.detail || t('form.validation.invalid')
    const name = errorField(e)
    if (!name) rest.push(text)
    else next[name] = next[name] ? `${next[name]} ${text}` : text
  }
  errors.value = next
  formErrors.value = rest
}

async function submit() {
  if (saving.value || loading.value) return
  saving.value = true
  errors.value = {}
  warnings.value = {}
  formErrors.value = []
  blocked.value = false
  try {
    const properties: Record<string, unknown> = {}
    for (const name of writes.value) {
      const v = values.value[name]
      if (v !== undefined && JSON.stringify(v) !== JSON.stringify(loaded[name]))
        properties[name] = v
    }
    const result = await completeFlowStep(props.run.id, props.run.generation, properties)
    if (!result.moved) {
      // The values are written: the next submit compares against them.
      loaded = { ...values.value }
      const notes: Record<string, string> = {}
      for (const w of result.warnings ?? []) {
        notes[w.field] = notes[w.field] ? `${notes[w.field]} ${w.message}` : w.message
      }
      warnings.value = notes
      blocked.value = result.problem === 'blocked'
      void focusFeedback(blocked.value ? [...required.value] : [])
    }
    emit('completed', result)
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      ui.info(t('flows.step.moved'))
      emit('stale')
      return
    }
    const invalid = err instanceof ApiError ? err.validationErrors : []
    if (invalid.length > 0) {
      showRefusal(invalid)
      void focusFeedback()
    } else {
      ui.error(getErrorMessage(err, t('flows.step.failed')))
    }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <form
    ref="formEl"
    class="flow-step-form"
    novalidate
    data-testid="flow-step-form"
    @submit.prevent="submit"
  >
    <p v-if="loadError" class="flow-step-form__error" role="alert">{{ loadError }}</p>
    <div v-if="fields.length > 0 && shownOnce" class="flow-step-form__fields">
      <div
        v-for="f in fields"
        :key="f.name"
        :data-step-field="f.name"
        data-testid="flow-step-field"
      >
        <FieldRenderer
          :field="f.field"
          :property-def="f.def"
          :value="values[f.name]"
          :error="errors[f.name]"
          :warning="warnings[f.name]"
          :readonly="saving || loading"
          :entity-type="subjectType"
          :entity-id="run.subject"
          @update="setValue(f.name, $event)"
        />
      </div>
    </div>
    <p
      v-for="(msg, i) in formErrors"
      :key="i"
      class="flow-step-form__error"
      role="alert"
      data-testid="flow-step-refused"
    >
      {{ msg }}
    </p>
    <p
      v-if="blocked"
      class="flow-step-form__notice"
      role="alert"
      data-testid="flow-step-blocked"
    >
      {{ required.size > 0 ? t('flows.step.blockedRequired') : t('flows.step.problem.blocked') }}
    </p>
    <div class="flow-step-form__actions">
      <RlButton
        type="submit"
        variant="primary"
        :loading="saving"
        :disabled="loading"
        :pending-label="t('flows.step.completing')"
        data-testid="flow-complete"
      >
        {{ t('flows.step.complete') }}
      </RlButton>
    </div>
  </form>
</template>

<style scoped>
.flow-step-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

/* One labelled field per row, as on the entity form. */
.flow-step-form__fields {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
}

.flow-step-form__fields > * {
  min-width: 0;
}

.flow-step-form__error {
  margin: 0;
  color: var(--rl-color-danger);
}

.flow-step-form__notice {
  margin: 0;
  color: var(--rl-color-warning-fg);
}

.flow-step-form__actions {
  display: flex;
  gap: var(--space-sm);
}
</style>
