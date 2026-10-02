<script setup lang="ts">
import RlIcon from './RlIcon.vue'
import type { IconName } from './icons'

/**
 * Small count + icon pair. The icon is decorative, so `countLabel` supplies
 * the meaning ("3 comments") that the glyph alone would only imply.
 */
defineProps<{ icon?: IconName; label: string | number; countLabel?: string }>()
</script>

<template>
  <span class="rl-meta-item">
    <span class="rl-meta-item__label">{{ label }}</span>
    <span v-if="countLabel" class="rl-visually-hidden">&nbsp;{{ countLabel }}</span>
    <RlIcon v-if="icon" :name="icon" :size="13" />
  </span>
</template>

<style scoped>
.rl-meta-item {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-subtle);
}
</style>
