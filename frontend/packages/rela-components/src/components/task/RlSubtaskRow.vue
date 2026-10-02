<script setup lang="ts">
import type { Task } from '../../types'

withDefaults(defineProps<{ task: Task; highlight?: boolean }>(), { highlight: false })
const emit = defineEmits<{ click: [task: Task] }>()
</script>

<template>
  <div class="rl-subtask-row">
    <button type="button" class="rl-subtask-row__button" @click="emit('click', task)">
      {{ task.title }}
    </button>
    <span class="rl-subtask-row__meta" :class="{ 'rl-subtask-row__meta--highlight': highlight }">
      <span v-if="task.dueDate">
        <span class="rl-visually-hidden">Due </span>{{ task.dueDate }}
      </span>
      <span v-if="task.assignee">
        <span class="rl-visually-hidden">Assigned to </span>{{ task.assignee }}
      </span>
    </span>
  </div>
</template>

<style scoped>
.rl-subtask-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--rl-space-4);
  padding: var(--rl-space-3) var(--rl-space-2);
  border-bottom: 1px solid var(--rl-color-border);
}
.rl-subtask-row:hover { background: var(--rl-color-bg-sunken); }

.rl-subtask-row:has(.rl-subtask-row__button:focus-visible) {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

.rl-subtask-row__button {
  flex: 1;
  min-width: 0;
  padding: 0;
  border: none;
  background: none;
  font: inherit;
  color: inherit;
  text-align: left;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-subtask-row__button::after {
  content: '';
  position: absolute;
  inset: 0;
}

.rl-subtask-row__button:focus-visible { outline: none; }

.rl-subtask-row__meta {
  display: flex;
  align-items: center;
  gap: var(--rl-space-5);
  font-size: var(--rl-font-size-md);
  color: var(--rl-color-text-subtle);
}

.rl-subtask-row__meta--highlight { color: var(--rl-color-text-muted); }
</style>
