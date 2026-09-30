<script setup lang="ts">
/** Status dot + title + count, shared by board columns and table sections. */
import type { Component } from 'vue'
import type { StatusColor } from '../../types'
import RlStatusDot from '../common/RlStatusDot.vue'
import RlCount from '../common/RlCount.vue'
import RlHeading from '../common/RlHeading.vue'

withDefaults(
  defineProps<{
    title: string
    color?: StatusColor
    /**
     * A glyph before the title, taken as a component so an app's own icon
     * registry can supply it. See `Section.icon`.
     */
    icon?: Component
    count?: number
    size?: 'sm' | 'md'
    level?: 1 | 2 | 3 | 4 | 5 | 6
  }>(),
  { size: 'md', level: 2 },
)
</script>

<template>
  <div class="rl-section-heading">
    <component :is="icon" v-if="icon" class="rl-section-heading__icon" :size="16" />
    <RlStatusDot v-else :color="color" />
    <RlHeading :level="level" :size="size === 'md' ? 'lg' : 'md'" line-height="normal">{{ title }}</RlHeading>
    <RlCount v-if="count !== undefined" :value="count" />
  </div>
</template>

<style scoped>
.rl-section-heading {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}

/*
 * The icon replaces the status dot rather than joining it: both sit in the
 * same slot before the title, and a heading carrying a dot and a glyph reads
 * as two separate claims about the column.
 */
.rl-section-heading__icon {
  flex: none;
  color: var(--rl-color-text-subtle);
}
</style>
