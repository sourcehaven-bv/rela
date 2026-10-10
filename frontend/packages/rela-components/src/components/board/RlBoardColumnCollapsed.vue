<script setup lang="ts" generic="T extends CollectionItem">
import type { CollectionItem, Section } from '../../types'
import RlStatusDot from '../common/RlStatusDot.vue'
import RlCount from '../common/RlCount.vue'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

defineProps<{ section: Section<T> }>()
const emit = defineEmits<{ expand: [section: Section<T>] }>()
</script>

<template>
  <button
    type="button"
    class="rl-board-column-collapsed"
    :aria-label="messages.expandSection({ title: section.title })"
    :data-section-id="section.id"
    @click="emit('expand', section)"
  >
    <component :is="section.icon" v-if="section.icon" class="rl-board-column-collapsed__icon" :size="16" />
    <RlStatusDot v-else :color="section.color" />
    <span class="rl-board-column-collapsed__title">{{ section.title }}</span>
    <RlCount :value="section.count ?? section.items.length" />
  </button>
</template>

<style scoped>
.rl-board-column-collapsed {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--rl-space-3);
  width: 48px;
  flex: none;
  align-self: stretch;
  padding: var(--rl-space-3) 0;
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg-sunken);
  font-family: inherit;
  cursor: pointer;
}
.rl-board-column-collapsed:hover { background: var(--rl-color-bg-hover); }

.rl-board-column-collapsed__icon {
  flex: none;
  color: var(--rl-color-text-subtle);
}

.rl-board-column-collapsed__title {
  font-size: var(--rl-font-size-md);
  font-weight: var(--rl-font-weight-medium);
  color: var(--rl-color-text);
  writing-mode: vertical-rl;
}

.rl-board-column-collapsed:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}
</style>
