<script setup lang="ts">
/**
 * An on/off control for a setting that takes effect immediately.
 *
 * Not a restyled checkbox. A checkbox means "include this when I submit" and
 * belongs in a form with a button under it; a switch means "this is on now",
 * and flipping one is the act itself. Using a checkbox on a page that saves
 * as you go tells a small lie about when the change lands, and using a switch
 * inside a form that needs submitting tells the opposite one.
 *
 * `role="switch"` rather than `checkbox`, so a screen reader says "on" and
 * "off" rather than "ticked": the words match what the control does.
 *
 * Takes an optional `pending` state, because a switch that applies at once is
 * usually waiting on a server. The control stays in its old position while
 * the request is out rather than flipping optimistically, so it never shows a
 * state the server has not agreed to.
 */
import { computed } from 'vue'
import RlFieldShell from './RlFieldShell.vue'

const props = withDefaults(
  defineProps<{
    modelValue?: boolean
    label: string
    hint?: string
    error?: string
    disabled?: boolean
    labelHidden?: boolean
    /**
     * Whether the change is in flight. Holds the current position and blocks
     * further presses, so a slow save cannot be queued up twice.
     */
    pending?: boolean
    size?: 'sm' | 'md'
  }>(),
  {
    modelValue: false,
    disabled: false,
    labelHidden: false,
    pending: false,
    size: 'md',
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()

const locked = computed(() => props.disabled || props.pending)

function toggle() {
  if (locked.value) return
  emit('update:modelValue', !props.modelValue)
}

/*
 * Space and Enter both flip it. A native button takes Space only; a switch is
 * commonly reached for with Enter, and the ARIA pattern asks for both.
 */
function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'Enter') return
  event.preventDefault()
  toggle()
}
</script>

<template>
  <RlFieldShell
    :label="label"
    :hint="hint"
    :error="error"
    :label-hidden="labelHidden"
    label-mode="wrap"
  >
    <template #default="{ describedBy, invalid }">
      <button
        type="button"
        class="rl-switch"
        :class="[
          `rl-switch--${size}`,
          {
            'rl-switch--on': modelValue,
            'rl-switch--locked': locked,
            'rl-switch--pending': pending,
          },
        ]"
        role="switch"
        :aria-checked="modelValue"
        :aria-describedby="describedBy"
        :aria-invalid="invalid || undefined"
        :disabled="locked"
        @click="toggle"
        @keydown="onKeydown"
      >
        <span class="rl-switch__track">
          <span class="rl-switch__thumb" />
        </span>
      </button>
    </template>
  </RlFieldShell>
</template>

<style scoped>
/*
 * The shell's `wrap` mode already lays the control before the label in a
 * row, which is how a switch reads: "[state] [what it controls]".
 */
.rl-switch {
  flex: none;
  padding: 0;
  border: none;
  background: none;
  cursor: pointer;
  line-height: 0;
}

.rl-switch__track {
  display: block;
  position: relative;
  border-radius: var(--rl-radius-pill);
  background: var(--rl-color-border-strong);
  transition: background var(--rl-duration-base) var(--rl-ease);
}

.rl-switch__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  border-radius: var(--rl-radius-pill);
  /* Contrasts with the track it rides on, not with the page behind it. */
  background: var(--rl-color-knob);
  box-shadow: var(--rl-shadow-sm);
  transition: transform var(--rl-duration-base) var(--rl-ease);
}

.rl-switch--md .rl-switch__track { width: 36px; height: 20px; }
.rl-switch--md .rl-switch__thumb { width: 16px; height: 16px; }
.rl-switch--md.rl-switch--on .rl-switch__thumb { transform: translateX(16px); }

.rl-switch--sm .rl-switch__track { width: 28px; height: 16px; }
.rl-switch--sm .rl-switch__thumb { width: 12px; height: 12px; }
.rl-switch--sm.rl-switch--on .rl-switch__thumb { transform: translateX(12px); }

.rl-switch--on .rl-switch__track { background: var(--rl-color-accent); }

.rl-switch:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 2px;
  border-radius: var(--rl-radius-pill);
}

.rl-switch--locked { cursor: default; }
.rl-switch--locked .rl-switch__track { opacity: 0.5; }

/*
 * Pending reads as busy rather than as broken: the track keeps its colour and
 * the thumb pulses, so the position still says what is true while the request
 * is out.
 */
.rl-switch--pending .rl-switch__thumb { animation: rl-switch-pulse 1s ease-in-out infinite; }

@keyframes rl-switch-pulse {
  50% { opacity: 0.45; }
}

@media (prefers-reduced-motion: reduce) {
  .rl-switch__track,
  .rl-switch__thumb { transition: none; }

  .rl-switch--pending .rl-switch__thumb { animation: none; opacity: 0.6; }
}

/* The track is small; the press target around it is not. */
@media (pointer: coarse) {
  .rl-switch {
    padding: var(--rl-space-2);
    margin: calc(-1 * var(--rl-space-2));
  }
}
</style>
