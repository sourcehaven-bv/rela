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
   It is now custom-drawn from the scales.css tokens.

   THIS IS THE ONLY PLACE A CHECKBOX IS STYLED. RelationCards.vue used to
   hand-roll an identical control (`.inline-edit-checkbox`) for its inline
   boolean edit; the two copies drifted (off-centre tick, hardcoded focus-ring
   colour) and a fix applied to one silently missed the other. Both of its
   boolean inputs now render this widget and its local rules are deleted. If a
   new surface needs a checkbox, render this widget — do not restyle a raw
   `<input type="checkbox">`.

   `appearance: none` is what makes the box drawable, and it also removes the
   greyed rendering the browser gave the disabled DISPLAY arm for free. That
   affordance was load-bearing (RR-UD2H reads the display arm as visibly
   read-only), so the muted state below is now drawn explicitly rather than
   inherited. */
input[type='checkbox'] {
  /* Unprefixed only: -webkit-appearance is unnecessary from Safari 15.4, and
     nothing in this build regenerates prefixes (no autoprefixer, no
     browserslist). */
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

/* The checkmark: the right and bottom borders of a box, rotated 45°, so the
   two painted edges read as the short and long arms of a tick.

   The ARM GEOMETRY (5×10, 2px stroke) is RelationCards', unchanged — it
   matches the native glyph's proportions well. The POSITION is not:
   RelationCards' `left: 5px; top: 1px` leaves the tick low and right, with
   dead space along the top and the vertex nearly touching the bottom edge.

   `top` is NEGATIVE and that is correct, not a typo. Rotating about the box
   centre turns the 5×10 rect into a 10.61×8.49 ink bounding box, so the
   painted glyph sits well inside — and below — the element's own edges.
   `left`/`top` position the UNROTATED rect, so they are NOT the visible
   margins and cannot be reasoned about as if they were.

   These are tuned by eye at 13×, and deliberately NOT the arithmetic centre.
   Centring the ink bounding box gives 4.5px/0.94px (1.7px clearance on all
   four sides) — that was tried, and it reads as sitting low, because a tick's
   mass is in its long upper-right arm while the eye tracks the lower-left
   vertex. Optical centring wins over the bounding box here. If the 18px box or
   2px stroke changes, re-tune the same way; do not scale these arithmetically
   and do not "fix" them back to the computed centre. */
input[type='checkbox']:checked::after {
  content: '';
  position: absolute;
  left: 4px;
  top: -1px;
  width: 5px;
  height: 10px;
  border: solid #fff;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}

input[type='checkbox']:hover:not(:disabled) {
  border-color: var(--accent-color, #6366f1);
}

/* The shared ring token (TKT-FRING7). This widget originally carried its own
   local `color-mix` at 30% — the first fix for the hardcoded indigo — which is
   now hoisted into `--focus-ring` and used by every focusable control. The
   token is fully opaque because no translucent ring reaches WCAG 2.2's 3:1
   non-text minimum; 30% scored 1.46:1.

   TWO shadows, not one, via the shared --focus-ring-gap band. This widget is
   the extreme case that motivated the token: a CHECKED checkbox is FILLED with
   --accent-color and the ring is that same colour, so a single ring abuts its
   own fill at 1:1 contrast and disappears entirely — focused+checked would
   look identical to unfocused. Every other control has the milder version of
   the same problem (accent border against accent ring). */
input[type='checkbox']:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 2px var(--focus-ring-gap),
    0 0 0 4px var(--focus-ring);
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
