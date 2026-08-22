<script setup lang="ts">
import { computed } from 'vue'
import type { WidgetProps } from './types'

const props = defineProps<WidgetProps>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const boolValue = computed(() => props.modelValue === true || props.modelValue === 'true')

function onChange(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).checked)
}
</script>

<template>
  <!-- Display mode uses a real disabled checkbox so screen readers get
       native "checkbox, checked|unchecked, read-only" semantics, and
       rendering is consistent across system fonts (RR-UD2I). -->
  <input
    v-if="mode === 'display'"
    type="checkbox"
    :checked="boolValue"
    disabled
    aria-readonly="true"
    class="display-checkbox"
  />
  <input
    v-else
    :id="id"
    type="checkbox"
    :checked="boolValue"
    :disabled="disabled"
    @change="onChange"
  />
</template>

<style scoped>
/* TKT-CBSTYLE. The edit arm was a bare native checkbox — OS default size and
   colour — sitting in the same 12-column grid as widgets that all carry
   padding, a token radius and a focus ring, so it read as a foreign control.
   This is the custom-drawn checkbox RelationCards.vue already uses for its
   inline boolean edit (`.inline-edit-checkbox`), expressed in the scales.css
   tokens: same 18px box, same accent fill, same CSS checkmark, so the two
   boolean edit surfaces agree.

   `appearance: none` is what makes the box drawable, and it also removes the
   greyed rendering the browser gave the disabled DISPLAY arm for free. That
   affordance was load-bearing (RR-UD2H reads the display arm as visibly
   read-only), so the muted state below is now drawn explicitly rather than
   inherited. */
input[type='checkbox'] {
  /* Unprefixed only: -webkit-appearance is unnecessary from Safari 15.4, and
     nothing in this build regenerates prefixes (no autoprefixer, no
     browserslist). RelationCards still carries the prefix; it predates that. */
  appearance: none;
  width: 18px;
  height: 18px;
  border: 2px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: var(--input-bg);
  position: relative;
  cursor: pointer;

  /* Named, not `all`: `all` would also animate the focus ring, so the
     indicator would fade in rather than appear on tab. */
  transition:
    background-color 0.15s,
    border-color 0.15s;
  flex-shrink: 0;
}

input[type='checkbox']:checked {
  background: var(--accent-color, #6366f1);
  border-color: var(--accent-color, #6366f1);
}

/* The checkmark: two borders of a rotated box. Geometry is copied from
   RelationCards so the glyph is identical at the same 18px size. */
input[type='checkbox']:checked::after {
  content: '';
  position: absolute;
  left: 5px;
  top: 1px;
  width: 5px;
  height: 10px;
  border: solid #fff;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}

input[type='checkbox']:hover:not(:disabled) {
  border-color: var(--accent-color, #6366f1);
}

/* Derived from the accent rather than a hardcoded indigo. The sibling widgets
   write `rgba(99, 102, 241, 0.1)` here, but that literal is NOT the app's
   accent (#4772fb light / #6f93ff dark) — it renders as an off-hue ring that
   ignores the theme, and an operator's `custom.css` accent would not reach it.
   Copying that would propagate the one part of the prior art that is a bug. */
input[type='checkbox']:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent-color) 30%, transparent);
}

/* `:not(.display-checkbox)` is load-bearing, not defensive.
   `input[type='checkbox']:disabled` is (0,3,1) and `.display-checkbox:disabled`
   is (0,3,0) — Vue's scoped `[data-v-x]` lifts both equally — so the element
   rule WINS and a plain override below it never applies. The display arm was
   silently getting `not-allowed`, inverting the intent stated here. Excluding
   the arm rather than out-specifying it also can't be re-lost by a later edit. */
input[type='checkbox']:disabled:not(.display-checkbox) {
  cursor: not-allowed;
  opacity: 0.6;
}

/* Display mode is never interactive, so it takes the default cursor rather
   than not-allowed — it is a rendered value, not a control you were denied.
   Muted, not hidden: a checked read-only box must still read as checked.

   0.6, not the 0.85 this file used before. That value was a nudge ON TOP OF
   the browser's native disabled greying; with `appearance: none` it became the
   ENTIRE read-only signal, and a 15% reduction on an accent-filled box is
   near-imperceptible. The sibling widgets' disabled treatment
   (`background: var(--hover-bg)`) can't be borrowed here: on a checkbox the
   background IS the checked signal, so overwriting it would erase state to
   convey read-only. Dimming preserves both. */
.display-checkbox:disabled {
  cursor: default;
  opacity: 0.6;
}

/* In forced-colors (Windows High Contrast) the OS replaces our background and
   border, `box-shadow` is dropped entirely, and a `::after` checkmark is not
   guaranteed — so a custom-painted checkbox can lose BOTH its focus ring and
   its checked/unchecked distinction. Hand the control back to the OS, which
   draws it correctly for free, and restore a ring that survives. */
@media (forced-colors: active) {
  input[type='checkbox'] {
    appearance: auto;
  }

  input[type='checkbox']:focus-visible {
    outline: 2px solid Highlight;
    outline-offset: 2px;
  }
}
</style>
