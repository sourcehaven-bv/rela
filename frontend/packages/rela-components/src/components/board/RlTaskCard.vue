<script setup lang="ts">
import type { Task } from '../../types'
import RlTag from '../common/RlTag.vue'
import RlMetaItem from '../common/RlMetaItem.vue'
import RlHeading from '../common/RlHeading.vue'

withDefaults(defineProps<{ task: Task; selected?: boolean }>(), { selected: false })
const emit = defineEmits<{ click: [task: Task] }>()
</script>

<template>
  <article class="rl-task-card" :class="{ 'rl-task-card--selected': selected }">
    <RlHeading :level="3" size="md" weight="normal" line-height="normal" class="rl-task-card__title">
      <!--
        The button carries the accessible name and all interaction; the
        stretched pseudo-element makes the whole card clickable without
        putting a click handler on a non-interactive element.
      -->
      <button
        type="button"
        class="rl-task-card__button"
        :aria-current="selected ? 'true' : undefined"
        @click="emit('click', task)"
      >
        {{ task.title }}
      </button>
    </RlHeading>

    <div v-if="task.tags?.length" class="rl-task-card__tags">
      <RlTag v-for="tag in task.tags" :key="tag.id" :label="tag.label" :color="tag.color" />
    </div>

    <footer
      v-if="task.assignee || task.dueDate || task.commentCount || task.subtaskCount"
      class="rl-task-card__footer"
    >
      <span v-if="task.assignee" class="rl-task-card__assignee">{{ task.assignee }}</span>

      <div class="rl-task-card__meta">
        <RlMetaItem v-if="task.commentCount" icon="message-square" :label="task.commentCount" countLabel="comments" />
        <RlMetaItem v-if="task.subtaskCount" icon="git-branch" :label="task.subtaskCount" countLabel="subtasks" />
        <span v-if="task.dueDate" class="rl-task-card__due">
          <span class="rl-visually-hidden">Due </span>{{ task.dueDate }}
        </span>
      </div>
    </footer>
  </article>
</template>

<style scoped>
.rl-task-card {
  position: relative;
  padding: var(--rl-space-3) var(--rl-space-4);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg);
  transition: box-shadow var(--rl-duration-fast) var(--rl-ease), border-color var(--rl-duration-fast) var(--rl-ease);
}

.rl-task-card:hover {
  border-color: var(--rl-color-border-strong);
  box-shadow: var(--rl-shadow-sm);
}

/* Matches the table's selected row, so selection survives a change of view. */
.rl-task-card--selected,
.rl-task-card--selected:hover {
  border-color: var(--rl-color-accent);
  background: var(--rl-color-bg-selected);
}

/* Focus lands on the inner button; show the ring around the whole card. */
.rl-task-card:has(.rl-task-card__button:focus-visible) {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}


.rl-task-card__button {
  padding: 0;
  border: none;
  background: none;
  font: inherit;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.rl-task-card__button::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
}

.rl-task-card__button:focus-visible { outline: none; }

.rl-task-card__tags {
  display: flex;
  flex-wrap: wrap;
  gap: var(--rl-space-2);
  margin-top: var(--rl-space-2);
}

.rl-task-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--rl-space-3);
  margin-top: var(--rl-space-3);
}

.rl-task-card__assignee {
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-muted);
}

.rl-task-card__meta {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  margin-left: auto;
}

.rl-task-card__due {
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-subtle);
}
</style>
