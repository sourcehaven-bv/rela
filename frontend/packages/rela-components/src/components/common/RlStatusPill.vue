<script setup lang="ts">
import type { StatusColor } from '../../types'
import RlStatusDot from './RlStatusDot.vue'

withDefaults(defineProps<{ label: string; color?: StatusColor }>(), { color: 'green' })
</script>

<template>
  <span class="rl-status-pill">
    <RlStatusDot :color="color" :size="7" />
    {{ label }}
  </span>
</template>

<style scoped>
.rl-status-pill {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: 2px var(--rl-space-3) 2px var(--rl-space-2);
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg-hover);
  font-size: var(--rl-font-size-xs);
  font-weight: var(--rl-font-weight-medium);
  color: var(--rl-color-text-muted);
  line-height: 18px;
  white-space: nowrap;
}
</style>
