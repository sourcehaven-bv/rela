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
 */
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import FieldRenderer from '@/components/forms/FieldRenderer.vue'
import { completeFlowStep, type FlowCompleted, type FlowRun } from '@/api/flows'
import { getEntity } from '@/api/entities'
import { ApiError, getErrorMessage } from '@/api/errors'
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
const errors = ref<Record<string, string>>({})
const saving = ref(false)
const loadError = ref('')
// The fields show once the subject's values are in, so an edit made while
// they load is not overwritten by them. loadSeq drops a superseded load.
const loading = ref(true)
let loadSeq = 0

interface StepField {
  name: string
  field: FormFieldOrRelation
  def?: PropertyDef
}

const fields = computed<StepField[]>(() => {
  const props_ = schema.getEntityType(props.subjectType)?.properties ?? {}
  return writes.value.map((name) => ({
    name,
    def: props_[name],
    field: { property: name, label: props_[name]?.label || name },
  }))
})

/** Reads the subject's current values of the written properties. */
async function loadValues() {
  const seq = ++loadSeq
  loadError.value = ''
  errors.value = {}
  const subject = props.run.subject
  if (!subject || writes.value.length === 0) {
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
    for (const name of writes.value) next[name] = e.properties?.[name]
    values.value = next
    loaded = { ...next }
  } catch (err) {
    if (seq !== loadSeq) return
    loadError.value = getErrorMessage(err, t('flows.step.loadFailed'))
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

watch(
  () => [props.run.id, props.run.generation, props.run.subject, writes.value.join(',')],
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

async function submit() {
  if (saving.value || loading.value) return
  saving.value = true
  errors.value = {}
  try {
    const properties: Record<string, unknown> = {}
    for (const name of writes.value) {
      const v = values.value[name]
      if (v !== undefined && JSON.stringify(v) !== JSON.stringify(loaded[name])) properties[name] = v
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
    <p v-if="loadError" class="flow-step-form__error" role="alert">{{ loadError }}</p>
    <div v-if="fields.length > 0 && !loading" class="properties-list flow-step-form__fields">
      <FieldRenderer
        v-for="f in fields"
        :key="f.name"
        :field="f.field"
        :property-def="f.def"
        :value="values[f.name]"
        :error="errors[f.name]"
        :readonly="saving"
        :entity-type="subjectType"
        :entity-id="run.subject"
        @update="setValue(f.name, $event)"
      />
    </div>
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

.flow-step-form__error {
  margin: 0;
  color: var(--rl-color-danger);
}

.flow-step-form__actions {
  display: flex;
  gap: var(--space-sm);
}
</style>
