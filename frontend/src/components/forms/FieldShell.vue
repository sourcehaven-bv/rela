<script setup lang="ts">
// FieldShell owns the chrome around a property widget: label (with
// required asterisk), help text, error text, and the .form-field layout.
// Widgets render only their input control. labelPosition handles the
// checkbox case where the label follows the control.
//
// # Why the shell is separate from the control
//
// rela-components bundles the two: `RlTextField` renders its own
// `RlFieldShell`. rela cannot, because the shell has to wrap widgets the
// library has no equivalent for -- RruleWidget, StatusControl, and the file
// widget's staged-upload create mode. If ordinary fields were bundles and
// those were shell-plus-control, the label, help and error text would be
// rendered by two different components and would drift. The dispatch in
// FieldRenderer is what keeps one shell around all of them.
//
// What IS taken from RlFieldShell is its accessibility wiring, below.
import { computed, useId } from 'vue'
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

// The help and error text used to render with no association to the input, so
// a screen-reader user heard the label and then nothing: the guidance and the
// validation message were visible but unannounced. These ids fix that.
//
// `fieldId` still comes from above (callers depend on `field-<property>`), so
// only the describing elements need ids of their own. `useId` rather than
// deriving them from `fieldId`, which is absent whenever a field config
// arrives without a property name.
const uid = useId()
const helpId = computed(() => `${uid}-help`)
const errorId = computed(() => `${uid}-error`)

// Error first: a user who has just been told what went wrong should hear that
// before the original guidance.
const describedBy = computed(() => {
  const ids = [props.error ? errorId.value : null, props.help ? helpId.value : null]
  return ids.filter(Boolean).join(' ') || undefined
})

// Handed to the control through the slot so the widget can spread it onto the
// element it actually renders; the shell cannot reach across the boundary.
const control = computed(() => ({
  describedBy: describedBy.value,
  invalid: Boolean(props.error),
}))
</script>

<template>
  <div class="form-field" :class="{ 'has-error': error }" :style="spanStyle">
    <template v-if="labelPosition === 'after'">
      <div class="checkbox-wrapper">
        <slot v-bind="control" />
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
      <slot v-bind="control" />
    </template>

    <p v-if="help" :id="helpId" class="field-help">{{ help }}</p>
    <!--
      role="alert" so a message that appears after the user leaves the field is
      announced without them having to go back and look for it.
    -->
    <p v-if="error" :id="errorId" class="field-error" role="alert">{{ error }}</p>
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
  color: var(--rl-color-text);
}

.required {
  color: var(--rl-color-danger, #ef4444);
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

/* Input/textarea/select visuals now come from the library's global
   `.rl-control` class (rl/styles/control.css, imported by styles/rl.css), so
   a rela field draws the same box as an Rl one. FieldShell still owns only
   label/help/error chrome and layout. */

.field-help {
  font-size: 13px;
  color: var(--rl-color-text-muted);
  margin: 0;
}

.field-error {
  font-size: 13px;
  color: var(--rl-color-danger, #ef4444);
  margin: 0;
}
</style>
