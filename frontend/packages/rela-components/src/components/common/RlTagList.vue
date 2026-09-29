<script setup lang="ts">
/**
 * A row of tags that states what it is hiding.
 *
 * Tags are set by the user, so a task can carry more of them than any cell is
 * wide. Letting them overflow paints one column over the next; clipping them
 * is worse, because the row then looks like it has fewer tags than it does.
 * This shows the first few and counts the rest, so the row stays inside its
 * column and still tells the truth about what is on the task.
 */
import type { Tag } from '../../types'
import RlTag from './RlTag.vue'

const props = withDefaults(defineProps<{ tags: Tag[]; max?: number }>(), { max: 2 })

const shown = () => props.tags.slice(0, props.max)
const hidden = () => props.tags.length - props.max
/** Named in full, so the count is not the only way to know what is hidden. */
const hiddenLabel = () =>
  props.tags
    .slice(props.max)
    .map((tag) => tag.label)
    .join(', ')
</script>

<template>
  <span class="rl-tag-list">
    <RlTag v-for="tag in shown()" :key="tag.id" :label="tag.label" :color="tag.color" />
    <span v-if="hidden() > 0" class="rl-tag-list__more" :title="hiddenLabel()">
      +{{ hidden() }}
      <span class="rl-visually-hidden">more: {{ hiddenLabel() }}</span>
    </span>
  </span>
</template>

<style scoped>
.rl-tag-list {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  min-width: 0;
}

/*
 * The counter must survive the squeeze that hid the tags in the first place,
 * so it never shrinks; the tags before it give way instead.
 */
.rl-tag-list__more {
  flex: none;
  font-size: var(--rl-font-size-xs);
  font-weight: var(--rl-font-weight-medium);
  color: var(--rl-color-text-subtle);
}
</style>
