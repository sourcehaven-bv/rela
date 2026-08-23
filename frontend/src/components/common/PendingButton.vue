<script setup lang="ts">
import { computed } from 'vue'
import { useDelayedPending } from '@/composables/useDelayedPending'
import { PENDING_TIMINGS } from '@/composables/pendingTimings'

// Explicit-action pending state (TKT-TFSNBY): "Save" -> "Saving…".
//
// A label swap rather than a spinner, deliberately:
//   - it says WHAT is happening; a spinner says only that something is;
//   - it is the only option that survives prefers-reduced-motion, since
//     text has no motion to suppress;
//   - the swapped label IS the screen-reader announcement, so a spinner
//     would need a parallel visually-hidden live region to convey less.
// Turbo ships the same idea as `data-turbo-submits-with`.
//
// `pendingLabel` is REQUIRED and never derived from `label`. Munging
// "Save" into "Saving" breaks on the first irregular verb or translated
// string, and a wrong pending verb is worse than an explicit one.

const props = withDefaults(
  defineProps<{
    /** True while the action is in flight. Gated before it is shown. */
    pending: boolean
    /** Resting label, e.g. "Save". */
    label: string
    /** In-flight label, e.g. "Saving…". Use U+2026, not "...". */
    pendingLabel: string
    /** Native button type. Defaults to "button" to avoid stray submits. */
    type?: 'button' | 'submit'
    /** Disabled for reasons other than pending (e.g. an invalid form). */
    disabled?: boolean
  }>(),
  { type: 'button', disabled: false }
)

const emit = defineEmits<{ click: [MouseEvent] }>()

const showPending = useDelayedPending(
  () => props.pending,
  {
    delay: PENDING_TIMINGS.actionDelayMs,
    minDuration: PENDING_TIMINGS.actionMinDurationMs,
  }
)

// aria-disabled, not native `disabled`, while pending — native disabled
// drops focus to <body> mid-interaction, stranding a keyboard user who was
// on this control. Per RR-R5VL59 this applies to the PRIMARY action only,
// which is all this component is; secondary/Cancel buttons keep native
// disabled because "cancel an in-flight action" has no defined meaning.
//
// A genuinely disabled button (invalid form) still uses native disabled:
// there the control is not interactive at all, and there is no in-flight
// operation whose focus we are trying to preserve.
const nativelyDisabled = computed(() => props.disabled)
// Never emit BOTH. If a caller sets `disabled` while also pending, native
// disabled has already dropped focus and made the control inert, so adding
// aria-disabled would only assert a focus-preserving contract the element
// no longer honours. Native disabled wins; aria-disabled is for the
// pending-but-otherwise-enabled case it exists to serve.
const ariaDisabled = computed(() =>
  props.pending && !props.disabled ? 'true' : undefined
)

// aria-disabled does NOT prevent activation, so suppression is ours to do.
// Gated on `pending` (the raw source), not `showPending` (the gated
// display state) — a second click during the pre-delay window must not
// fire a second request just because nothing is on screen yet. On a
// destructive action that would be a second DELETE.
function onClick(event: MouseEvent) {
  if (props.pending || props.disabled) {
    event.preventDefault()
    return
  }
  emit('click', event)
}

// Keyboard activation reaches an aria-disabled button too — that is the
// entire point of using it over native disabled — so Enter/Space need the
// same guard. A native <button> synthesises a click from both, so this
// only has to stop the default before that happens.
function onKeydown(event: KeyboardEvent) {
  if (!props.pending && !props.disabled) return
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
  }
}

// Announced via a polite live region rather than by the visible label, so
// assistive tech gets one clear message. Empty when idle — copying
// AutoSaveIndicator's discipline — so nothing is announced on mount or
// when settling back.
const announcement = computed(() => (showPending.value ? props.pendingLabel : ''))
</script>

<template>
  <button
    class="pending-button"
    :type="type"
    :disabled="nativelyDisabled"
    :aria-disabled="ariaDisabled"
    :data-pending="showPending ? 'true' : undefined"
    @click="onClick"
    @keydown="onKeydown"
  >
    <!--
      Both labels are ALWAYS rendered, stacked in one grid cell. The
      inactive one is `visibility: hidden`, which keeps its box, so the
      cell sizes to max(width(label), width(pendingLabel)) and the swap
      causes zero reflow — the button cannot resize under the cursor.

      Browser-computed, so it stays correct after webfont load and in every
      language. v-if / display:none would collapse the hidden state's box
      and reintroduce the resize this component exists to prevent.
    -->
    <span class="pending-button__labels">
      <span
        class="pending-button__label"
        :class="{ 'pending-button__label--hidden': showPending }"
        >{{ label }}</span
      >
      <span
        class="pending-button__label"
        :class="{ 'pending-button__label--hidden': !showPending }"
        >{{ pendingLabel }}</span
      >
    </span>
    <!--
      Adjacent content that is NOT part of the swap — a keyboard-shortcut
      hint, say. Deliberately outside the reserved cell: it is identical in
      both states, so including it would inflate the reservation for no
      reason, and it must not disappear while pending.
    -->
    <slot name="adornment" />
    <span class="pending-button__sr" role="status" aria-live="polite">{{ announcement }}</span>
  </button>
</template>

<style scoped>
/* The button itself lays out the label group and any adornment inline. */
.pending-button {
  display: inline-flex;
  align-items: center;
  gap: var(--space-xs, 4px);
}

.pending-button__labels {
  display: grid;
  /* Both children occupy the same cell; the grid takes the wider. */
  align-items: center;
  justify-items: center;
}

.pending-button__label {
  grid-area: 1 / 1;
  white-space: nowrap;
}

/* visibility, not display — the hidden label must keep contributing its
   box or the width reservation collapses. */
.pending-button__label--hidden {
  visibility: hidden;
}

/* Standard sr-only clip, matching AutoSaveIndicator's. */
.pending-button__sr {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
</style>
