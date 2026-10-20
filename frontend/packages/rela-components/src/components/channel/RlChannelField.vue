<script setup lang="ts">
/**
 * One composer field, drawn with the library's form control for its widget.
 *
 * The value stays raw (`ChannelFieldValue`): input text, a checkbox state, or
 * a ref's entity id. Converting it to a request value is the app's, so every
 * conversion rule (dates, time zones, integer range) lives in one place. An
 * integer is a text box with a numeric keyboard, so what the user typed
 * reaches the app unchanged.
 *
 * An optional boolean has one unchecked state, and it means "not set":
 * unchecking emits undefined, never false, so the box cannot send two
 * different values while looking the same. A required boolean starts as
 * false and stays a plain yes or no.
 *
 * A ref has no text box: it shows the chosen entity as a chip and a button
 * that asks the app to pick one, through `pickRef`. Without `pickRef` it
 * falls back to a text box for the entity id. A `restricted` ref (an edit of
 * a message holding a ref the user may not read) shows the restricted
 * placeholder and a Replace button, and no Clear: the server keeps the stored
 * ref unless the user picks another.
 */
import { computed, ref } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlCheckbox from '../form/RlCheckbox.vue'
import RlDateField from '../form/RlDateField.vue'
import RlFieldShell from '../form/RlFieldShell.vue'
import RlSelect from '../form/RlSelect.vue'
import RlTextField from '../form/RlTextField.vue'
import RlTextarea from '../form/RlTextarea.vue'
import type { SelectOption } from '../form/types'
import RlRefChip from './RlRefChip.vue'
import type { ResolvedChannelField } from './channelFields'
import type { ChannelEnumOption, ChannelFieldValue, ChannelReadableRef } from './types'
import { useMessages, type ChannelTextKey } from '../../composables/useMessages'

const props = withDefaults(
  defineProps<{
    field: ResolvedChannelField
    modelValue: ChannelFieldValue
    error?: string
    enumOptions?: Record<string, ChannelEnumOption[]>
    /** The chosen ref, so the chip can show its title. */
    chosenRef?: ChannelReadableRef
    /**
     * Asks the app to pick an entity. Resolve null on cancel; never reject.
     * A rejection is treated as a cancel.
     */
    pickRef?: (to: string[]) => Promise<ChannelReadableRef | null>
    /** The stored ref is one the user may not read, and has not been replaced. */
    restricted?: boolean
    timeZoneLabel?: string
  }>(),
  {
    error: undefined,
    enumOptions: undefined,
    chosenRef: undefined,
    pickRef: undefined,
    restricted: false,
    timeZoneLabel: undefined,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: ChannelFieldValue]
  pick: [ref: ChannelReadableRef | null]
}>()

const messages = useMessages()
const t = (key: ChannelTextKey) => messages.channelText({ key })

const p = computed(() => props.field.property)
const text = computed(() => (typeof props.modelValue === 'string' ? props.modelValue : ''))

const selectOptions = computed<SelectOption[]>(() => {
  const known = p.value.enum ? props.enumOptions?.[p.value.enum] : undefined
  const opts = (p.value.values ?? []).map((v) => ({
    value: v,
    label: known?.find((o) => o.value === v)?.label ?? v,
  }))
  // An optional enum needs a way back to "not set", and that choice is its
  // empty state too, so it carries the placeholder.
  return p.value.required ? opts : [{ value: '', label: props.field.placeholder ?? '—' }, ...opts]
})

function setText(v: string) {
  emit('update:modelValue', v === '' ? undefined : v)
}

function setChecked(v: boolean) {
  emit('update:modelValue', v || p.value.required ? v : undefined)
}

const picking = ref(false)

async function choose() {
  if (!props.pickRef || picking.value) return
  picking.value = true
  let picked: ChannelReadableRef | null = null
  try {
    picked = await props.pickRef(p.value.to ?? [])
  } catch {
    // The contract is to resolve null on cancel; a rejection is read as one.
    picked = null
  } finally {
    picking.value = false
  }
  if (!picked) return
  emit('pick', picked)
  emit('update:modelValue', picked.id)
}

function clearRef() {
  emit('pick', null)
  emit('update:modelValue', undefined)
}

const chooseLabel = computed(() => {
  if (props.restricted) return t('replace')
  return text.value ? t('change') : (props.field.placeholder ?? t('choose'))
})
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
  <RlCheckbox
    v-else-if="field.widget === 'checkbox'"
    :model-value="modelValue === true"
    :label="field.label"
    :hint="field.help"
    :error="error"
    :required="p.required"
    @update:model-value="setChecked"
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
    :placeholder="p.required ? (field.placeholder ?? t('choose')) : undefined"
    @update:model-value="setText"
  />
  <RlFieldShell
    v-else-if="field.widget === 'ref' && (pickRef || restricted)"
    v-slot="control"
    :label="field.label"
    :hint="field.help"
    :error="error"
    :required="p.required"
  >
    <div
      :id="control.id"
      class="rl-channel-field__ref"
      role="group"
      :aria-label="field.label"
      :aria-describedby="control.describedBy"
    >
      <RlRefChip v-if="restricted" :value="{ restricted: true }" />
      <RlRefChip v-else-if="chosenRef" :value="chosenRef" plain />
      <span v-else-if="text" class="rl-channel-field__ref-id">{{ text }}</span>
      <RlButton
        v-if="pickRef"
        variant="secondary"
        size="sm"
        :loading="picking"
        :pending-label="t('choosing')"
        @click="choose"
      >
        {{ chooseLabel }}
      </RlButton>
      <RlButton v-if="text && !p.required && !restricted" variant="ghost" size="sm" @click="clearRef">
        {{ t('clear') }}
      </RlButton>
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
    :inputmode="field.widget === 'number' ? 'numeric' : undefined"
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
