<script setup lang="ts">
// FieldShell owns the chrome around a property widget: label (with
// required asterisk), help text, error text, and the .form-field layout.
// Widgets render only their input control. labelPosition handles the
// checkbox case where the label follows the control.
defineProps<{
  fieldId?: string
  label?: string
  required?: boolean
  help?: string
  error?: string
  labelPosition?: 'before' | 'after'
}>()
</script>

<template>
  <div class="form-field" :class="{ 'has-error': error }">
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

.checkbox-wrapper :deep(input) {
  width: 18px;
  height: 18px;
  cursor: pointer;
}

.checkbox-wrapper label {
  cursor: pointer;
}

.form-field :deep(input[type='text']),
.form-field :deep(input[type='number']),
.form-field :deep(input[type='date']),
.form-field :deep(textarea),
.form-field :deep(select) {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-color);
  transition: all 0.15s;
}

.form-field :deep(input:focus),
.form-field :deep(textarea:focus),
.form-field :deep(select:focus) {
  outline: none;
  border-color: var(--accent-color, #6366f1);
  box-shadow: 0 0 0 2px rgba(99, 102, 241, 0.1);
}

.form-field :deep(input:disabled),
.form-field :deep(textarea:disabled),
.form-field :deep(select:disabled) {
  background: var(--hover-bg);
  cursor: not-allowed;
}

.has-error :deep(input),
.has-error :deep(textarea),
.has-error :deep(select) {
  border-color: var(--error-color, #ef4444);
}

.has-error :deep(input:focus),
.has-error :deep(textarea:focus),
.has-error :deep(select:focus) {
  box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.1);
}

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
