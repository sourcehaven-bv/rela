<script setup lang="ts">
/** Caret toggle for collapsible sections and nav branches. */
import RlIcon from './RlIcon.vue'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

withDefaults(defineProps<{ open: boolean; label: string; size?: number }>(), { size: 12 })

defineEmits<{ toggle: [] }>()
</script>

<template>
  <button
    type="button"
    class="rl-disclosure"
    :aria-expanded="open"
    :aria-label="messages.toggleDisclosure({ label, open })"
    @click="$emit('toggle')"
  >
    <RlIcon :name="open ? 'chevron-down' : 'chevron-right'" :size="size" />
  </button>
</template>

<style scoped>
.rl-disclosure {
  display: flex;
  padding: var(--rl-space-1);
  border: none;
  border-radius: var(--rl-radius-sm);
  background: transparent;
  color: var(--rl-color-text-subtle);
  cursor: pointer;
}

.rl-disclosure:hover { background: var(--rl-color-bg-hover); }

.rl-disclosure:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}

/*
 * Touch devices need a comfortable hit area (WCAG 2.5.8 sets 24px as the
 * floor). The pseudo-element grows the target without changing the visual
 * size of the control.
 */
@media (pointer: coarse) {
  .rl-disclosure {
    position: relative;
  }

  .rl-disclosure::before {
    content: '';
    position: absolute;
    top: 50%;
    left: 50%;
    width: var(--rl-tap-target);
    height: var(--rl-tap-target);
    transform: translate(-50%, -50%);
  }
}
</style>
