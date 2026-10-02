<script setup lang="ts">
/** Muted "+ Add ..." affordance at the end of a list, column or section. */
import RlIcon from './RlIcon.vue'

withDefaults(defineProps<{ label: string; block?: boolean }>(), { block: false })

defineEmits<{ click: [event: MouseEvent] }>()
</script>

<template>
  <button
    type="button"
    class="rl-add-button"
    :class="{ 'rl-add-button--block': block }"
    @click="$emit('click', $event)"
  >
    <RlIcon name="plus" :size="15" />
    {{ label }}
  </button>
</template>

<style scoped>
.rl-add-button {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-2) var(--rl-space-3);
  border: none;
  border-radius: var(--rl-radius-md);
  background: transparent;
  font-family: inherit;
  font-size: var(--rl-font-size-md);
  color: var(--rl-color-text-subtle);
  cursor: pointer;
}

.rl-add-button:hover {
  background: var(--rl-color-bg-hover);
  color: var(--rl-color-text-muted);
}

.rl-add-button--block { width: 100%; }

.rl-add-button:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}
</style>
