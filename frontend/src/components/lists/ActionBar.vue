<script setup lang="ts">
import type { ActionConfig } from '@/types'

defineProps<{
  selectedCount: number
  actions: { id: string; config: ActionConfig }[]
  processing: boolean
}>()

const emit = defineEmits<{
  trigger: [actionId: string, action: ActionConfig]
}>()
</script>

<template>
  <div v-if="selectedCount > 0" class="action-bar">
    <span class="action-bar-count">{{ selectedCount }} selected</span>
    <div class="action-bar-actions">
      <button
        v-for="{ id, config } in actions"
        :key="id"
        class="action-bar-btn"
        :disabled="processing"
        @click="emit('trigger', id, config)"
      >
        <kbd>{{ config.key }}</kbd>
        {{ config.label }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.action-bar {
  position: sticky;
  bottom: 0;
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.5rem 1rem;
  background: var(--color-bg-elevated, #f0f0f0);
  border-top: 1px solid var(--color-border, #ddd);
  z-index: 10;
}

:root.dark .action-bar {
  background: var(--color-bg-elevated, #2a2a2a);
  border-top-color: var(--color-border, #444);
}

.action-bar-count {
  font-weight: 600;
  font-size: 0.875rem;
  white-space: nowrap;
}

.action-bar-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.action-bar-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.25rem 0.75rem;
  border: 1px solid var(--color-border, #ccc);
  border-radius: 4px;
  background: var(--color-bg, #fff);
  color: var(--color-text, #333);
  font-size: 0.8125rem;
  cursor: pointer;
  transition: background 0.15s;
}

.action-bar-btn:hover:not(:disabled) {
  background: var(--color-bg-hover, #e8e8e8);
}

.action-bar-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.action-bar-btn kbd {
  display: inline-block;
  padding: 0.1rem 0.35rem;
  border: 1px solid var(--color-border, #bbb);
  border-radius: 3px;
  background: var(--color-bg-muted, #eee);
  font-family: monospace;
  font-size: 0.75rem;
  line-height: 1;
}
</style>
