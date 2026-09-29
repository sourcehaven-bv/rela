<script setup lang="ts">
/**
 * One transient notification. Rendered by RlToastHost, not placed directly.
 *
 * The timer is paused while the pointer is over it or focus is inside it, so
 * a message never disappears while it is being read or acted on
 * (WCAG 2.2.1 needs a way to extend a time limit).
 *
 * ## Selecting a toast from a test
 *
 * `rl-toast` and `rl-toast--<tone>` are a supported contract, not incidental
 * class names: an end-to-end test may select on them and this library will
 * not rename them without a note in the changelog. The same holds for
 * `rl-toast__action`, which is the control a test reaches for to press Undo.
 *
 * No `data-testid`, because these classes already name the same things and a
 * second set of hooks would be a second thing to keep in step with the first.
 *
 * Prefer a role-based query where one fits — the dismiss control has an
 * accessible name, and the action button is a button with its own label —
 * and fall back to these classes for the toast element itself, which is a
 * plain container with no role of its own.
 */
import { onBeforeUnmount, onMounted, ref } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import RlButton from '../common/RlButton.vue'
import RlIconButton from '../common/RlIconButton.vue'
import RlText from '../common/RlText.vue'
import type { ToastMessage } from './types'

const props = withDefaults(defineProps<{ toast: ToastMessage }>(), {})

const emit = defineEmits<{ dismiss: [id: string] }>()

/*
 * The toast goes once the action has run: it described a state the action
 * has just changed, so leaving it up would leave a message that is no longer
 * true, with a control that would run twice.
 */
function onAction() {
  props.toast.action?.onAction()
  emit('dismiss', props.toast.id)
}

const icons = {
  info: 'info',
  success: 'check',
  warning: 'warning',
  danger: 'alert',
} as const

const remaining = ref(props.toast.duration ?? 5000)
let timer: ReturnType<typeof setTimeout> | undefined
let startedAt = 0

function start() {
  if (remaining.value <= 0) return
  startedAt = Date.now()
  timer = setTimeout(() => emit('dismiss', props.toast.id), remaining.value)
}

function pause() {
  if (!timer) return
  clearTimeout(timer)
  timer = undefined
  remaining.value -= Date.now() - startedAt
}

onMounted(start)
onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <div
    class="rl-toast"
    :class="`rl-toast--${toast.tone ?? 'info'}`"
    @mouseenter="pause"
    @mouseleave="start"
    @focusin="pause"
    @focusout="start"
  >
    <RlIcon :name="icons[toast.tone ?? 'info']" :size="18" class="rl-toast__icon" aria-hidden="true" />

    <div class="rl-toast__content">
      <RlText size="md" weight="medium">{{ toast.title }}</RlText>
      <RlText v-if="toast.description" as="p" size="sm" tone="muted" class="rl-toast__description">
        {{ toast.description }}
      </RlText>
    </div>

    <!--
      Before the dismiss, so the act the message invites comes first in the
      tab order: a reader tabbing into the toast reaches Undo before the
      control that throws the toast away.
    -->
    <RlButton
      v-if="toast.action"
      variant="subtle"
      size="sm"
      class="rl-toast__action"
      @click="onAction"
    >
      {{ toast.action.label }}
    </RlButton>

    <RlIconButton
      icon="x"
      label="Dismiss notification"
      :size="14"
      @click="emit('dismiss', toast.id)"
    />
  </div>
</template>

<style scoped>
.rl-toast__action { flex: none; }

.rl-toast {
  display: flex;
  align-items: flex-start;
  gap: var(--rl-space-3);
  width: 100%;
  padding: var(--rl-space-3) var(--rl-space-4);
  border: 1px solid var(--rl-color-border-raised);
  border-radius: var(--rl-radius-md);
  background: var(--rl-color-bg-raised);
  box-shadow: var(--rl-shadow-md);
}

.rl-toast__icon { flex: none; margin-top: 1px; }

/*
 * Line breaks in a message are kept: a message may list one item per line,
 * such as one error per line when a configuration fails to reload.
 */
.rl-toast__content { flex: 1; min-width: 0; white-space: pre-line; }
.rl-toast__description { margin: 2px 0 0; }

.rl-toast--info .rl-toast__icon { color: var(--rl-color-info-fg); }
.rl-toast--success .rl-toast__icon { color: var(--rl-color-success-fg); }
.rl-toast--warning .rl-toast__icon { color: var(--rl-color-warning-fg); }
.rl-toast--danger .rl-toast__icon { color: var(--rl-color-danger); }
</style>
