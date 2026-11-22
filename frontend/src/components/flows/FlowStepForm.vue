<script setup lang="ts">
/**
 * The step form of a waiting run (TKT-ZAXRXND, TKT-9AYQAYK): the
 * data-entry.yaml form the state names in `form:`, rendered with the entity
 * form's own field widgets, and the Complete button.
 *
 * The form's labels, help, widgets, `visible_when`, `required` and
 * `required_when` apply as they do on an entity form. The server bounds what
 * a step may send to `waiting.fields`, the form's fields that are not hidden.
 *
 * The form reads the subject through the gated entity GET, so the values it
 * starts from are the ones the reader may see. It sends only the visible,
 * editable fields the user changed, so a value edited elsewhere meanwhile is
 * not overwritten with the one the form loaded. An editable field the subject
 * holds no value for starts at the form field's `default:`, else the schema
 * property's, as on a new entity; Complete then sends it. It sends the run generation
 * it was rendered at: a run that moved on meanwhile answers 409, and the card
 * reloads it (`stale`).
 *
 * A step whose form cannot be used, because the server reports
 * `form_problem` or the schema store does not know the form, shows why and
 * offers no Complete: completing it would write nothing.
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
import { useFormWizard } from '@/composables/useFormWizard'
import { useSchemaStore, useUIStore } from '@/stores'
import { useUserToken } from '@/composables/useUserToken'
import { deepEqual } from '@/utils/deepEqual'
import { resolveUserToken } from '@/utils/userToken'
import type { FormConfig, FormFieldOrRelation, PropertyDef } from '@/types'

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

/** The properties the server lets this step send. */
const writable = computed(() => new Set(props.run.waiting?.fields ?? []))
const formConfig = computed<FormConfig | undefined>(() => {
  const name = props.run.waiting?.form
  return name ? schema.getForm(name) : undefined
})
/** Why the step's form cannot be used, or '' when it can. */
const formUnusable = computed(() => {
  const name = props.run.waiting?.form
  if (!name) return ''
  if (props.run.waiting?.form_problem || !formConfig.value)
    return t('flows.step.formUnusable', { form: name })
  return ''
})
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

const wizard = useFormWizard(
  formConfig,
  () => ({ form: values.value, entity: {}, current_user: {} }),
  {
    syncUrl: false,
  }
)

interface StepField {
  name: string
  field: FormFieldOrRelation
  def?: PropertyDef
  /** Shown, not sent: read-only in the form, or outside `waiting.fields`. */
  readonly: boolean
}

/** The form fields every change branch's condition reads. */
const required = computed(() => {
  const bs = (props.run.waiting?.branches ?? []).filter((b) => !b.fork?.length)
  if (bs.length === 0) return new Set<string>()
  const each = bs.map((b) => new Set(b.fields ?? []))
  return new Set([...each[0]].filter((name) => each.every((set) => set.has(name))))
})

/** Every property field the form shows, visible or not. */
const formFields = computed(() =>
  (formConfig.value?.fields ?? []).filter(
    (f): f is FormFieldOrRelation & { property: string } => !!f.property && !f.hidden
  )
)

/** def, marked required when every change branch's condition reads name. */
function requiredDef(def: PropertyDef | undefined, name: string): PropertyDef | undefined {
  return def && required.value.has(name) ? { ...def, required: true } : def
}

/** The fields whose `visible_when` holds now, in form order. */
const fields = computed<StepField[]>(() => {
  const defs = schema.getEntityType(props.subjectType)?.properties ?? {}
  const step = wizard.currentStepDef.value
  const visible = step ? wizard.visibleFieldsOf(step) : []
  const seen = new Set<string>()
  const out: StepField[] = []
  for (const f of visible) {
    if (!f.property || f.hidden || seen.has(f.property)) continue
    seen.add(f.property)
    out.push({
      name: f.property,
      // A form field without a label takes the schema's, as a form does.
      field: f.label ? f : { ...f, label: defs[f.property]?.label || f.property },
      def: requiredDef(defs[f.property], f.property),
      readonly: !!f.readonly || !writable.value.has(f.property),
    })
  }
  return out
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

const signedInUser = useUserToken(() => formFields.value.map((f) => f.default))

/**
 * The value an empty editable field starts at: the form field's `default:`,
 * else the schema property's, or undefined.
 */
function defaultOf(f: FormFieldOrRelation & { property: string }): unknown {
  if (f.readonly || !writable.value.has(f.property)) return undefined
  if (f.default !== undefined) return resolveUserToken(f.default, signedInUser.value)
  return schema.getEntityType(props.subjectType)?.properties?.[f.property]?.default
}

/** Reads the subject's current values of the form's properties. */
async function loadValues() {
  const seq = ++loadSeq
  loadError.value = ''
  errors.value = {}
  warnings.value = {}
  formErrors.value = []
  const subject = props.run.subject
  if (!subject || formFields.value.length === 0) {
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
    for (const f of formFields.value) next[f.property] = e.properties?.[f.property]
    loaded = { ...next }
    for (const f of formFields.value) {
      if (empty(next[f.property])) {
        const d = defaultOf(f)
        if (d !== undefined) next[f.property] = d
      }
    }
    values.value = next
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
    () => formFields.value.map((f) => f.property).join(','),
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
  const names = fields.value.filter((f) => !f.readonly).map((f) => f.name)
  if (named && names.includes(named)) return named
  return names.find((w) => e.fields?.includes(w)) ?? ''
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

function empty(v: unknown): boolean {
  return v === undefined || v === null || v === '' || (Array.isArray(v) && v.length === 0)
}

/** Marks the visible, editable fields that are required now and empty. */
function validate(): boolean {
  const next: Record<string, string> = {}
  for (const f of fields.value) {
    if (f.readonly) continue
    // A guard's mark on f.def is a hint; only the schema and the form hold
    // a field back.
    const schemaDef = schema.getEntityType(props.subjectType)?.properties?.[f.name]
    const required = !!schemaDef?.required || wizard.isFieldRequired(f.field)
    if (required && empty(values.value[f.name])) next[f.name] = t('form.validation.required')
  }
  errors.value = next
  return Object.keys(next).length === 0
}

async function submit() {
  if (saving.value || loading.value || formUnusable.value) return
  if (!validate()) return
  saving.value = true
  errors.value = {}
  warnings.value = {}
  formErrors.value = []
  blocked.value = false
  try {
    // A field `visible_when` hides is not sent: its value is not what the
    // user sees. The server would refuse a name outside `waiting.fields`.
    const properties: Record<string, unknown> = {}
    for (const f of fields.value) {
      if (f.readonly) continue
      const v = values.value[f.name]
      if (v !== undefined && !deepEqual(v, loaded[f.name])) properties[f.name] = v
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
    <p
      v-if="formUnusable"
      class="flow-step-form__error"
      role="alert"
      data-testid="flow-step-form-unusable"
    >
      {{ formUnusable }}
    </p>
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
          :readonly="saving || loading || f.readonly"
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
    <div v-if="!formUnusable" class="flow-step-form__actions">
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
