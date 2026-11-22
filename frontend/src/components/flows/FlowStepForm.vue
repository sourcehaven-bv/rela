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
 */
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import FieldRenderer from '@/components/forms/FieldRenderer.vue'
import { completeFlowStep, type FlowCompleted, type FlowRun } from '@/api/flows'
import { getEntity } from '@/api/entities'
import { ApiError, getErrorMessage } from '@/api/errors'
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
const errors = ref<Record<string, string>>({})
const saving = ref(false)
const loadError = ref('')
// The fields show once the subject's values are in, so an edit made while
// they load is not overwritten by them. loadSeq drops a superseded load.
const loading = ref(true)
let loadSeq = 0

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

/** Every property field the form shows, visible or not. */
const formFields = computed(() =>
  (formConfig.value?.fields ?? []).filter(
    (f): f is FormFieldOrRelation & { property: string } => !!f.property && !f.hidden
  )
)

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
      field: f,
      def: defs[f.property],
      readonly: !!f.readonly || !writable.value.has(f.property),
    })
  }
  return out
})

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
  const subject = props.run.subject
  if (!subject || formFields.value.length === 0) {
    values.value = {}
    loaded = {}
    loading.value = false
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
    if (seq === loadSeq) loading.value = false
  }
}

watch(
  () => [
    props.run.id,
    props.run.generation,
    props.run.subject,
    formFields.value.map((f) => f.property).join(','),
  ],
  () => void loadValues(),
  { immediate: true }
)

function setValue(name: string, value: unknown) {
  values.value = { ...values.value, [name]: value }
  if (errors.value[name]) {
    const next = { ...errors.value }
    delete next[name]
    errors.value = next
  }
}

/** The property a validation error names: its field, or the last pointer segment. */
function errorField(e: { field?: string; source?: { pointer: string } }): string {
  if (e.field) return e.field
  const pointer = e.source?.pointer ?? ''
  return pointer.slice(pointer.lastIndexOf('/') + 1)
}

function empty(v: unknown): boolean {
  return v === undefined || v === null || v === '' || (Array.isArray(v) && v.length === 0)
}

/** Marks the visible, editable fields that are required now and empty. */
function validate(): boolean {
  const next: Record<string, string> = {}
  for (const f of fields.value) {
    if (f.readonly) continue
    const required = !!f.def?.required || wizard.isFieldRequired(f.field)
    if (required && empty(values.value[f.name])) next[f.name] = t('form.validation.required')
  }
  errors.value = next
  return Object.keys(next).length === 0
}

async function submit() {
  if (saving.value || loading.value || formUnusable.value) return
  if (!validate()) return
  saving.value = true
  try {
    // A field `visible_when` hides is not sent: its value is not what the
    // user sees. The server would refuse a name outside `waiting.fields`.
    const properties: Record<string, unknown> = {}
    for (const f of fields.value) {
      if (f.readonly) continue
      const v = values.value[f.name]
      if (v !== undefined && !deepEqual(v, loaded[f.name])) properties[f.name] = v
    }
    emit('completed', await completeFlowStep(props.run.id, props.run.generation, properties))
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      ui.info(t('flows.step.moved'))
      emit('stale')
      return
    }
    const invalid = err instanceof ApiError ? err.validationErrors : []
    if (invalid.length > 0) {
      const next: Record<string, string> = {}
      for (const e of invalid) {
        const name = errorField(e)
        if (name) next[name] = e.message || e.detail || t('form.validation.invalid')
      }
      errors.value = next
    }
    ui.error(getErrorMessage(err, t('flows.step.failed')))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <form class="flow-step-form" data-testid="flow-step-form" @submit.prevent="submit">
    <p
      v-if="formUnusable"
      class="flow-step-form__error"
      role="alert"
      data-testid="flow-step-form-unusable"
    >
      {{ formUnusable }}
    </p>
    <p v-if="loadError" class="flow-step-form__error" role="alert">{{ loadError }}</p>
    <div v-if="fields.length > 0 && !loading" class="properties-list flow-step-form__fields">
      <FieldRenderer
        v-for="f in fields"
        :key="f.name"
        :field="f.field"
        :property-def="f.def"
        :value="values[f.name]"
        :error="errors[f.name]"
        :readonly="saving || f.readonly"
        :entity-type="subjectType"
        :entity-id="run.subject"
        @update="setValue(f.name, $event)"
      />
    </div>
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

.flow-step-form__error {
  margin: 0;
  color: var(--rl-color-danger);
}

.flow-step-form__actions {
  display: flex;
  gap: var(--space-sm);
}
</style>
