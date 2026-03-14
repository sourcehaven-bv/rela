<script setup lang="ts">
import { computed } from 'vue'
import type { FormField, PropertyDef } from '@/types'

const props = defineProps<{
  field: FormField
  propertyDef?: PropertyDef
  value: unknown
  error?: string
  readonly?: boolean
}>()

const emit = defineEmits<{
  update: [value: unknown]
}>()

const label = computed(() => props.field.label || props.field.property || '')
const placeholder = computed(() => props.field.placeholder || '')
const help = computed(() => props.field.help || props.propertyDef?.description || '')

const inputType = computed(() => {
  const propType = props.propertyDef?.type || 'string'
  switch (propType) {
    case 'date':
      return 'date'
    case 'integer':
      return 'number'
    case 'boolean':
      return 'checkbox'
    default:
      return 'text'
  }
})

const isTextarea = computed(() => {
  return props.field.widget === 'textarea'
})

const isSelect = computed(() => {
  return (props.propertyDef?.values?.length ?? 0) > 0 || props.field.widget === 'select'
})

const isMultiSelect = computed(() => {
  return props.propertyDef?.list === true || props.field.widget === 'multiselect'
})

const isCheckbox = computed(() => {
  return props.propertyDef?.type === 'boolean' || props.field.widget === 'checkbox'
})

const options = computed(() => props.propertyDef?.values || [])

const stringValue = computed(() => {
  if (props.value === null || props.value === undefined) return ''
  return String(props.value)
})

const boolValue = computed(() => {
  return props.value === true || props.value === 'true'
})

const arrayValue = computed(() => {
  if (Array.isArray(props.value)) return props.value
  if (props.value) return [props.value]
  return []
})

function handleInput(event: Event) {
  const target = event.target as HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement

  if (isCheckbox.value) {
    emit('update', (target as HTMLInputElement).checked)
  } else if (inputType.value === 'number') {
    const num = parseInt(target.value, 10)
    emit('update', isNaN(num) ? target.value : num)
  } else {
    emit('update', target.value)
  }
}

function handleMultiSelect(event: Event) {
  const select = event.target as HTMLSelectElement
  const selected = Array.from(select.selectedOptions).map((opt) => opt.value)
  emit('update', selected)
}
</script>

<template>
  <div class="form-field" :class="{ 'has-error': error }">
    <label v-if="!isCheckbox" :for="`field-${field.property}`">
      {{ label }}
      <span v-if="propertyDef?.required" class="required">*</span>
    </label>

    <!-- Checkbox -->
    <div v-if="isCheckbox" class="checkbox-wrapper">
      <input
        :id="`field-${field.property}`"
        type="checkbox"
        :checked="boolValue"
        :disabled="readonly"
        @change="handleInput"
      />
      <label :for="`field-${field.property}`">
        {{ label }}
        <span v-if="propertyDef?.required" class="required">*</span>
      </label>
    </div>

    <!-- Textarea -->
    <textarea
      v-else-if="isTextarea"
      :id="`field-${field.property}`"
      :value="stringValue"
      :placeholder="placeholder"
      :disabled="readonly"
      rows="4"
      @input="handleInput"
    ></textarea>

    <!-- Multi-select -->
    <select
      v-else-if="isMultiSelect"
      :id="`field-${field.property}`"
      :disabled="readonly"
      multiple
      @change="handleMultiSelect"
    >
      <option
        v-for="opt in options"
        :key="opt"
        :value="opt"
        :selected="arrayValue.includes(opt)"
      >
        {{ opt }}
      </option>
    </select>

    <!-- Select -->
    <select
      v-else-if="isSelect"
      :id="`field-${field.property}`"
      :value="stringValue"
      :disabled="readonly"
      @change="handleInput"
    >
      <option value="">Select...</option>
      <option v-for="opt in options" :key="opt" :value="opt">
        {{ opt }}
      </option>
    </select>

    <!-- Standard input -->
    <input
      v-else
      :id="`field-${field.property}`"
      :type="inputType"
      :value="stringValue"
      :placeholder="placeholder"
      :disabled="readonly"
      @input="handleInput"
    />

    <p v-if="help" class="field-help">{{ help }}</p>
    <p v-if="error" class="field-error">{{ error }}</p>
  </div>
</template>

<style scoped>
.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-field label {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
}

.required {
  color: var(--error-color, #ef4444);
}

.checkbox-wrapper {
  display: flex;
  align-items: center;
  gap: 8px;
}

.checkbox-wrapper input {
  width: 18px;
  height: 18px;
  cursor: pointer;
}

.checkbox-wrapper label {
  cursor: pointer;
}

input[type="text"],
input[type="number"],
input[type="date"],
textarea,
select {
  padding: 10px 12px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  font-size: 14px;
  transition: all 0.15s;
}

input:focus,
textarea:focus,
select:focus {
  outline: none;
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

input:disabled,
textarea:disabled,
select:disabled {
  background: #f8fafc;
  cursor: not-allowed;
}

select[multiple] {
  min-height: 120px;
}

.has-error input,
.has-error textarea,
.has-error select {
  border-color: var(--error-color, #ef4444);
}

.has-error input:focus,
.has-error textarea:focus,
.has-error select:focus {
  box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.1);
}

.field-help {
  font-size: 13px;
  color: #64748b;
  margin: 0;
}

.field-error {
  font-size: 13px;
  color: var(--error-color, #ef4444);
  margin: 0;
}
</style>
