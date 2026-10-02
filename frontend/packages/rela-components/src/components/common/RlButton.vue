<script setup lang="ts">
/**
 * The general-purpose button. `variant` carries the visual weight and
 * `tone` carries the meaning, so a destructive action can be either a solid
 * Delete button or a quiet text-only one without needing its own variant.
 */
import { computed, type Component } from 'vue'
import RlIcon from './RlIcon.vue'
import RlSpinner from './RlSpinner.vue'
import type { IconName } from './icons'
import { useDelayedPending } from '../../composables/useDelayedPending'

const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'ghost' | 'subtle'
    /** `danger` recolours any variant for destructive actions. */
    tone?: 'default' | 'danger'
    size?: 'sm' | 'md' | 'lg'
    /** Shorthand for a leading icon; the `icon` slot overrides it. */
    icon?: IconName
    /** Swaps the leading icon for a spinner and blocks interaction. */
    loading?: boolean
    loadingLabel?: string
    /**
     * Replaces the label while loading, such as `Save` becoming `Saving`.
     * This is the explicit-action pending indicator: the change is on the
     * control the user just pressed, so there is no doubt what is working.
     *
     * Write it out rather than expecting it to be derived from the label:
     * turning `Save` into `Saving` breaks on the first irregular verb or
     * translated string, and a wrong pending verb is worse than an explicit
     * one.
     */
    pendingLabel?: string
    /**
     * How long `loading` must hold before the pending label appears, and the
     * minimum it stays once it does. Both default to the explicit-action
     * timings, so a fast response shows nothing at all.
     */
    pendingDelay?: number
    pendingMinDuration?: number
    disabled?: boolean
    block?: boolean
    /** Set `submit` to use the button inside a form. */
    type?: 'button' | 'submit' | 'reset'
    /**
     * What to render. A navigation action that goes somewhere is a link, not
     * a button: only a real `a` or router link gives the user the middle
     * click, the modifier click and the status bar they expect from one.
     *
     * Pass the router link component itself rather than a name, so this
     * library does not depend on a router. The pending props do not apply,
     * because a link has no in-flight state to report.
     */
    as?: 'button' | 'a' | Component
  }>(),
  {
    variant: 'ghost',
    tone: 'default',
    size: 'md',
    loading: false,
    loadingLabel: 'Working',
    disabled: false,
    block: false,
    type: 'button',
    as: 'button',
  },
)

const emit = defineEmits<{ click: [event: MouseEvent] }>()

/*
 * Anything that is not the native button is treated as a link: it takes none
 * of the button-only attributes, and its activation is the browser's to
 * handle rather than ours.
 */
const isButton = computed(() => props.as === 'button')

const iconSize = { sm: 14, md: 15, lg: 16 } as const

const showPending = useDelayedPending(() => props.loading, {
  delay: props.pendingDelay,
  minDuration: props.pendingMinDuration,
})

const showPendingLabel = computed(() => showPending.value && !!props.pendingLabel)

/*
 * With a pending label, the spinner waits for the same gate as the label, so a
 * fast action passes in complete silence.
 *
 * Without a pending label the spinner IS the indicator, so it appears as soon
 * as `loading` is set: gating it would leave a press with no acknowledgement.
 */
const showSpinner = computed(() =>
  props.pendingLabel ? showPending.value : props.loading,
)

/*
 * A spinner appearing beside a label would widen the button mid-action, which
 * is the same reflow the stacked labels prevent. So a button that swaps its
 * label holds the spinner's slot open from the start, rather than adding a flex
 * child when the swap happens. Not needed when an icon is already present: the
 * spinner replaces it in a box that is already the right size.
 */
const reservesSpinner = computed(
  () => !!props.pendingLabel && !props.icon,
)

/*
 * `aria-disabled` while loading rather than native `disabled`, because native
 * disabled drops focus to `<body>` mid-interaction and strands a keyboard user
 * who was on this control. A button that is disabled for another reason, such
 * as an invalid form, keeps native disabled: it is not interactive at all, and
 * there is no in-flight operation whose focus needs preserving.
 *
 * Never both. If a caller sets `disabled` while loading, native disabled has
 * already made the control inert, so `aria-disabled` would assert a
 * focus-preserving contract the element no longer honours.
 */
const ariaDisabled = computed(() =>
  props.loading && !props.disabled ? 'true' : undefined,
)

/*
 * `aria-disabled` does not prevent activation, so suppression is ours. Gated on
 * `loading` rather than on `showPending`, because a second click during the
 * delay window must not fire a second request just because nothing is on
 * screen yet. On a destructive action that would be a second delete.
 */
function onClick(event: MouseEvent) {
  /*
   * A link navigates; suppressing that here would cancel the navigation the
   * user asked for, including the modifier and middle clicks that are the
   * reason to render a link at all.
   */
  if (isButton.value && (props.loading || props.disabled)) {
    event.preventDefault()
    return
  }
  emit('click', event)
}

/*
 * Keyboard activation reaches an `aria-disabled` button too, which is the point
 * of using it over native disabled, so Enter and Space need the same guard. A
 * native button synthesises a click from both, so stopping the default is
 * enough.
 */
function onKeydown(event: KeyboardEvent) {
  if (!props.loading && !props.disabled) return
  if (event.key === 'Enter' || event.key === ' ') event.preventDefault()
}
</script>

<template>
  <component
    :is="as"
    :type="isButton ? type : undefined"
    class="rl-button"
    :class="[
      `rl-button--${variant}`,
      `rl-button--${size}`,
      `rl-button--tone-${tone}`,
      { 'rl-button--block': block, 'rl-button--loading': loading },
    ]"
    :disabled="isButton ? disabled : undefined"
    :aria-disabled="isButton ? ariaDisabled : undefined"
    :aria-busy="isButton && loading ? true : undefined"
    @click="onClick"
    @keydown="isButton ? onKeydown($event) : undefined"
  >
    <!--
      The spinner's slot, held open for the whole action when a label swap is in
      play so the button does not widen when the spinner arrives.
    -->
    <span
      v-if="reservesSpinner"
      class="rl-button__spinner-slot"
      :style="{ width: `${iconSize[size]}px`, height: `${iconSize[size]}px` }"
    >
      <RlSpinner v-if="showSpinner" :size="iconSize[size]" :label="loadingLabel" />
    </span>
    <template v-else>
      <RlSpinner v-if="showSpinner" :size="iconSize[size]" :label="loadingLabel" />
      <slot v-else name="icon">
        <RlIcon v-if="icon" :name="icon" :size="iconSize[size]" />
      </slot>
    </template>

    <!--
      With a pending label, both labels are always rendered, stacked in one
      grid cell. The inactive one keeps its box through `visibility: hidden`,
      so the cell sizes to the wider of the two and the swap causes no reflow:
      the button cannot resize under the cursor. Measuring this in the browser
      keeps it correct after a webfont loads and in every language, which
      `v-if` cannot do, because collapsing the hidden box is what reintroduces
      the resize.
    -->
    <span v-if="pendingLabel && $slots.default" class="rl-button__labels">
      <span class="rl-button__label" :class="{ 'rl-button__label--hidden': showPendingLabel }">
        <slot />
      </span>
      <span class="rl-button__label" :class="{ 'rl-button__label--hidden': !showPendingLabel }">
        {{ pendingLabel }}
      </span>
    </span>
    <span v-else-if="$slots.default" class="rl-button__label"><slot /></span>
    <slot name="trailing" />

    <!--
      Announced through a live region rather than by the visible label, so
      assistive technology gets one clear message. Empty while idle, following
      RlAutoSaveIndicator, so nothing is announced on mount or when settling
      back.
    -->
    <span v-if="pendingLabel" class="rl-button__sr" role="status" aria-live="polite">
      {{ showPendingLabel ? pendingLabel : '' }}
    </span>
  </component>
</template>

<style scoped>
.rl-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--rl-space-2);
  border: none;
  border-radius: var(--rl-radius-md);
  font-family: inherit;
  font-weight: var(--rl-font-weight-medium);
  cursor: pointer;
  /* A link root brings an underline the button shape does not want. */
  text-decoration: none;
  transition: background-color var(--rl-duration-fast) var(--rl-ease), color var(--rl-duration-fast) var(--rl-ease), box-shadow var(--rl-duration-fast) var(--rl-ease);
}

.rl-button:disabled,
.rl-button[aria-disabled='true'] {
  opacity: 0.5;
  cursor: not-allowed;
}

/*
 * A loading button cannot be pressed twice, but it is working rather than
 * unavailable, so it keeps its full colour.
 */
.rl-button--loading[aria-disabled='true'],
.rl-button--loading:disabled { opacity: 1; cursor: progress; }

/* Keeps the spinner's box whether or not the spinner is in it. */
.rl-button__spinner-slot {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
}

/* Both labels occupy the same cell, so the group takes the wider of the two. */
.rl-button__labels {
  display: grid;
  align-items: center;
  justify-items: center;
}

.rl-button__labels .rl-button__label {
  grid-area: 1 / 1;
  white-space: nowrap;
}

/* `visibility`, not `display`: the hidden label must keep its box or the width
   reservation collapses and the button resizes on the swap. */
.rl-button__label--hidden { visibility: hidden; }

.rl-button__sr {
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

.rl-button--block { width: 100%; }

/*
 * The height comes from the control token, not from the content, so a button
 * with a key hint or a larger icon is as tall as one without, and as tall as
 * the fields beside it.
 */
.rl-button--sm { min-height: var(--rl-control-height-sm); padding: 0 var(--rl-space-2); font-size: var(--rl-font-size-sm); }
.rl-button--md { min-height: var(--rl-control-height); padding: 0 var(--rl-space-3); font-size: var(--rl-font-size-md); }
.rl-button--lg { min-height: var(--rl-control-height-lg); padding: 0 var(--rl-space-5); font-size: var(--rl-font-size-lg); }

/* Solid fill. */
.rl-button--primary {
  background: var(--rl-color-accent);
  color: var(--rl-color-text-inverse);
}
.rl-button--primary:hover:not(:disabled) { background: var(--rl-color-accent-hover); }

.rl-button--tone-danger.rl-button--primary { background: var(--rl-color-danger); }
.rl-button--tone-danger.rl-button--primary:hover:not(:disabled) {
  background: var(--rl-color-danger-hover);
}

/*
 * Outlined: the default weight for Cancel beside a primary action. The border
 * is drawn inside the box so an outlined button is exactly as tall as a solid
 * one and the two line up in a row.
 */
.rl-button--secondary {
  box-shadow: inset 0 0 0 1px var(--rl-color-border-strong);
  background: var(--rl-color-bg);
  color: var(--rl-color-text);
}
.rl-button--secondary:hover:not(:disabled) { background: var(--rl-color-bg-hover); }

.rl-button--tone-danger.rl-button--secondary {
  box-shadow: inset 0 0 0 1px var(--rl-color-danger);
  color: var(--rl-color-danger);
}
.rl-button--tone-danger.rl-button--secondary:hover:not(:disabled) {
  background: var(--rl-color-danger-bg);
}

/* Text-only. */
.rl-button--ghost {
  background: transparent;
  color: var(--rl-color-text-muted);
}
.rl-button--ghost:hover:not(:disabled) {
  background: var(--rl-color-bg-hover);
  color: var(--rl-color-text);
}

.rl-button--tone-danger.rl-button--ghost { color: var(--rl-color-danger); }
.rl-button--tone-danger.rl-button--ghost:hover:not(:disabled) {
  background: var(--rl-color-danger-bg);
  color: var(--rl-color-danger-hover);
}

/* Tinted. */
.rl-button--subtle {
  background: var(--rl-color-bg-hover);
  color: var(--rl-color-text);
}
.rl-button--subtle:hover:not(:disabled) { background: var(--rl-color-bg-active); }

.rl-button--tone-danger.rl-button--subtle {
  background: var(--rl-color-danger-bg);
  color: var(--rl-color-danger);
}
.rl-button--tone-danger.rl-button--subtle:hover:not(:disabled) {
  background: var(--rl-color-danger-bg-hover);
}

.rl-button:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 2px;
}

/* Destructive controls take a matching ring so the meaning survives focus. */
.rl-button--tone-danger:focus-visible { outline-color: var(--rl-color-danger); }

@media (pointer: coarse) {
  .rl-button--sm,
  .rl-button--md { min-height: var(--rl-tap-target); }
}
</style>
