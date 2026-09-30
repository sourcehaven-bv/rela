<script setup lang="ts">
/**
 * The chrome around one form control: label, optional hint, error message and
 * the required marker.
 *
 * Every input in the library renders through this, so a field looks and
 * announces the same wherever it appears. It owns the ids and hands them to
 * the control through a scoped slot, which is what wires `aria-describedby`
 * to the hint and the error without each input repeating the plumbing.
 *
 * Rendering your own control into the default slot is supported, and is the
 * way to give the shell's treatment to a control this library does not cover.
 * A host that owns its own field layer can use the shell for the labelling
 * and announce behaviour alone.
 *
 * The slot props are the contract. Spread them onto a native control, or
 * forward them through your own wrapper:
 *
 *   <RlFieldShell label="Name" :error="error" v-slot="{ id, describedBy, invalid, required }">
 *     <input :id="id" class="rl-control" :aria-describedby="describedBy"
 *            :aria-invalid="invalid || undefined" :required="required" />
 *   </RlFieldShell>
 *
 * - `id` belongs on the control, because the label points at it.
 * - `describedBy` is the error id first, then the hint id, space-joined, and
 *   `undefined` when there is neither. The order is deliberate, not an
 *   accident of the array literal below.
 * - `invalid` is true whenever `error` is set. Map it to `aria-invalid`.
 * - `required` mirrors the prop, so the asterisk stays decorative.
 *
 * Treat a rename of any of these as a breaking change: a host reads them by
 * name, often in one shared wrapper that every one of its controls goes
 * through, so a rename breaks every field at once.
 */
import { computed, useId } from 'vue'
import RlText from '../common/RlText.vue'

const props = withDefaults(
  defineProps<{
    label: string
    /** Guidance shown under the label, before the user has done anything. */
    hint?: string
    /** A validation message. Its presence is what puts the field in error. */
    error?: string
    required?: boolean
    /**
     * Hides the label visually but keeps it for screen readers. For a field
     * whose purpose is already obvious from context, such as a search box
     * under a heading that says Search.
     */
    labelHidden?: boolean
    /**
     * `wrap` renders a `label` element around the control, for checkboxes
     * where the whole row should be clickable. `for` renders a separate label
     * pointing at the control, right for anything with its own focusable box.
     */
    labelMode?: 'for' | 'wrap'
  }>(),
  { required: false, labelHidden: false, labelMode: 'for' },
)

const uid = useId()
const controlId = computed(() => `${uid}-control`)
const hintId = computed(() => `${uid}-hint`)
const errorId = computed(() => `${uid}-error`)

/**
 * Both the hint and the error are announced, error first: a user who has just
 * been told what went wrong should hear that before the original guidance.
 */
const describedBy = computed(() => {
  const ids = [props.error ? errorId.value : null, props.hint ? hintId.value : null]
  return ids.filter(Boolean).join(' ') || undefined
})

const control = computed(() => ({
  id: controlId.value,
  describedBy: describedBy.value,
  invalid: Boolean(props.error),
  required: props.required,
}))
</script>

<template>
  <component
    :is="labelMode === 'wrap' ? 'label' : 'div'"
    class="rl-field"
    :class="{ 'rl-field--invalid': error, 'rl-field--wrap': labelMode === 'wrap' }"
  >
    <label
      v-if="labelMode === 'for'"
      :for="controlId"
      class="rl-field__label"
      :class="{ 'rl-visually-hidden': labelHidden }"
    >
      {{ label }}
      <!--
        The asterisk is decorative: `required` on the control is what conveys
        this to a screen reader, so repeating it as text would say it twice.
      -->
      <span v-if="required" class="rl-field__required" aria-hidden="true">*</span>
    </label>

    <div class="rl-field__control">
      <slot v-bind="control" />

      <!--
        Hidden rather than dropped: a wrapped label is the control's only
        accessible name, so removing it would leave the control unnamed.
      -->
      <span
        v-if="labelMode === 'wrap'"
        class="rl-field__wrapped-label"
        :class="{ 'rl-visually-hidden': labelHidden }"
      >
        {{ label }}
        <span v-if="required" class="rl-field__required" aria-hidden="true">*</span>
      </span>
    </div>

    <RlText v-if="hint" :id="hintId" as="p" size="sm" tone="muted" class="rl-field__hint">
      {{ hint }}
    </RlText>

    <!--
      `role="alert"` so a message that appears after the user leaves the field
      is announced without them having to go back and look for it.
    -->
    <p v-if="error" :id="errorId" class="rl-field__error" role="alert">{{ error }}</p>
  </component>
</template>

<style scoped>
.rl-field {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-1);
}

.rl-field__label {
  font-size: var(--rl-font-size-sm);
  font-weight: var(--rl-font-weight-medium);
  color: var(--rl-color-text);
}

.rl-field__required {
  margin-left: 2px;
  color: var(--rl-color-danger);
}

.rl-field__control { display: flex; flex-direction: column; }

/* A wrapped label sits beside its control rather than above it. */
.rl-field--wrap { cursor: pointer; }
.rl-field--wrap .rl-field__control {
  flex-direction: row;
  align-items: center;
  gap: var(--rl-space-2);
}
.rl-field__wrapped-label {
  font-size: var(--rl-font-size-md);
  color: var(--rl-color-text);
}

.rl-field__hint { margin: 0; }

.rl-field__error {
  margin: 0;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-danger);
}
</style>
