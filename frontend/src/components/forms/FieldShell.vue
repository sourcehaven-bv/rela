<script setup lang="ts">
// FieldShell owns the chrome around a property widget: label (with
// required asterisk), help text, error text, and the .form-field layout.
// Widgets render only their input control. labelPosition handles the
// checkbox case where the label follows the control.
import { computed } from 'vue'
import { fieldSpanStyle } from '@/utils/fieldSpan'

const props = defineProps<{
  fieldId?: string
  label?: string
  required?: boolean
  help?: string
  error?: string
  labelPosition?: 'before' | 'after'
  // Authored width on the 12-column form grid (TKT-5V8704); undefined = full
  // width. FieldShell owns the .form-field element, so it is the only place
  // that can set the grid column.
  span?: number
}>()

const spanStyle = computed(() => fieldSpanStyle(props.span))
</script>

<template>
  <div class="form-field" :class="{ 'has-error': error }" :style="spanStyle">
    <template v-if="labelPosition === 'after'">
      <div class="checkbox-wrapper">
        <slot />
        <label v-if="label" :for="fieldId">
          {{ label }}
          <span v-if="required" class="required">*</span>
        </label>
      </div>
    </template>

    <template v-else>
      <label v-if="label" :for="fieldId">
        {{ label }}
        <span v-if="required" class="required">*</span>
      </label>
      <slot />
    </template>

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
  color: var(--text-color);
}

.required {
  color: var(--error-color, #ef4444);
}

.checkbox-wrapper {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* Scoped to the direct checkbox input only — a future widget rendered
   with labelPosition='after' should NOT have its inputs miniaturised. */
.checkbox-wrapper :deep(input[type='checkbox']) {
  width: 18px;
  height: 18px;
  cursor: pointer;
}

.checkbox-wrapper label {
  cursor: pointer;
}

/* Input/textarea/select typography and focus/disabled/error visuals live
   on each widget so they don't cross component boundaries via :deep().
   FieldShell owns only label/help/error chrome and layout. */

.field-help {
  font-size: 13px;
  color: var(--muted-text);
  margin: 0;
}

.field-error {
  font-size: 13px;
  color: var(--error-color, #ef4444);
  margin: 0;
}
</style>
