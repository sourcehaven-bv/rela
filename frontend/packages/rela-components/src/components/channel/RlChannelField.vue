<script setup lang="ts">
/**
 * One composer field, drawn with the library's form control for its widget.
 *
 * The value stays raw (`ChannelFieldValue`): input text, a checkbox state, or
 * a ref's entity id. Converting it to a request value is the app's, so every
 * conversion rule (dates, time zones, integer range) lives in one place.
 *
 * A ref has no text box: it shows the chosen entity as a chip and a button
 * that asks the app to pick one, through `pickRef`. Without `pickRef` it
 * falls back to a text box for the entity id.
 */
import { computed } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlCheckbox from '../form/RlCheckbox.vue'
import RlDateField from '../form/RlDateField.vue'
import RlFieldShell from '../form/RlFieldShell.vue'
import RlNumberField from '../form/RlNumberField.vue'
import RlSelect from '../form/RlSelect.vue'
import RlTextField from '../form/RlTextField.vue'
import RlTextarea from '../form/RlTextarea.vue'
import type { SelectOption } from '../form/types'
import RlRefChip from './RlRefChip.vue'
import type { ResolvedChannelField } from './channelFields'
import type { ChannelEnumOption, ChannelFieldValue, ChannelReadableRef } from './types'

const props = defineProps<{
  field: ResolvedChannelField
  modelValue: ChannelFieldValue
  error?: string
  enumOptions?: Record<string, ChannelEnumOption[]>
  /** The chosen ref, so the chip can show its title. */
  chosenRef?: ChannelReadableRef
  pickRef?: (to: string[]) => Promise<ChannelReadableRef | null>
  timeZoneLabel?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: ChannelFieldValue]
  pick: [ref: ChannelReadableRef | null]
}>()

const p = computed(() => props.field.property)
const text = computed(() => (typeof props.modelValue === 'string' ? props.modelValue : ''))

const selectOptions = computed<SelectOption[]>(() => {
  const known = p.value.enum ? props.enumOptions?.[p.value.enum] : undefined
  const opts = (p.value.values ?? []).map((v) => ({
    value: v,
    label: known?.find((o) => o.value === v)?.label ?? v,
  }))
  // An optional enum needs a way back to "not set".
  return p.value.required ? opts : [{ value: '', label: '—' }, ...opts]
})

const numberValue = computed(() => {
  if (!text.value.trim()) return undefined
  const n = Number(text.value)
  return Number.isFinite(n) ? n : undefined
})

function setText(v: string) {
  emit('update:modelValue', v === '' ? undefined : v)
}

function setNumber(v: number | undefined) {
  emit('update:modelValue', v === undefined || Number.isNaN(v) ? undefined : String(v))
}

async function choose() {
  if (!props.pickRef) return
  const picked = await props.pickRef(p.value.to ?? [])
  if (!picked) return
  emit('pick', picked)
  emit('update:modelValue', picked.id)
}

function clearRef() {
  emit('pick', null)
  emit('update:modelValue', undefined)
}
</script>

<template>
  <RlTextarea
    v-if="field.widget === 'textarea'"
    :model-value="text"
    :label="field.label"
    :hint="field.help"
    :error="error"
    :required="p.required"
    :placeholder="field.placeholder"
    :rows="3"
    auto-grow
    @update:model-value="setText"
  />
  <RlNumberField
    v-else-if="field.widget === 'number'"
    :model-value="numberValue"
    :label="field.label"
    :hint="field.help"
    :error="error"
    :required="p.required"
    :placeholder="field.placeholder"
    :step="1"
    @update:model-value="setNumber"
  />
  <RlCheckbox
    v-else-if="field.widget === 'checkbox'"
    :model-value="modelValue === true"
    :label="field.label"
    :hint="field.help"
    :error="error"
    :required="p.required"
    @update:model-value="(v: boolean) => emit('update:modelValue', v)"
  />
  <RlDateField
    v-else-if="field.widget === 'date' || field.widget === 'datetime'"
    :model-value="text"
    :label="field.label"
    :hint="field.help"
    :error="error"
    :required="p.required"
    :with-time="field.widget === 'datetime'"
    :time-zone-label="timeZoneLabel"
    @update:model-value="setText"
  />
  <RlSelect
    v-else-if="field.widget === 'select'"
    :model-value="text"
    :label="field.label"
    :hint="field.help"
    :error="error"
    :required="p.required"
    :options="selectOptions"
    :placeholder="field.placeholder ?? 'Choose…'"
    @update:model-value="setText"
  />
  <RlFieldShell
    v-else-if="field.widget === 'ref' && pickRef"
    v-slot="control"
    :label="field.label"
    :hint="field.help"
    :error="error"
    :required="p.required"
  >
    <div :id="control.id" class="rl-channel-field__ref" role="group" :aria-label="field.label" :aria-describedby="control.describedBy">
      <RlRefChip v-if="chosenRef" :value="chosenRef" />
      <span v-else-if="text" class="rl-channel-field__ref-id">{{ text }}</span>
      <RlButton variant="secondary" size="sm" @click="choose">
        {{ text ? 'Change…' : (field.placeholder ?? 'Choose…') }}
      </RlButton>
      <RlButton v-if="text && !p.required" variant="ghost" size="sm" @click="clearRef">Clear</RlButton>
    </div>
  </RlFieldShell>
  <RlTextField
    v-else
    :model-value="text"
    :label="field.label"
    :hint="field.help"
    :error="error"
    :required="p.required"
    :placeholder="field.placeholder"
    @update:model-value="setText"
  />
</template>

<style scoped>
.rl-channel-field__ref {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-2);
}

.rl-channel-field__ref-id {
  font-family: var(--rl-font-family-mono);
  font-size: var(--rl-font-size-sm);
}
</style>
