<script setup lang="ts">
import type { StatusColor } from '../../types'

withDefaults(defineProps<{ color?: StatusColor; size?: number }>(), {
  color: 'grey',
  size: 8,
})
</script>

<template>
  <span
    class="rl-status-dot"
    :class="`rl-status-dot--${color}`"
    :style="{ width: `${size}px`, height: `${size}px` }"
  />
</template>

<style scoped>
.rl-status-dot {
  display: inline-block;
  flex: none;
  border-radius: var(--rl-radius-pill);
}

.rl-status-dot--green { background: var(--rl-color-status-green); }
.rl-status-dot--amber { background: var(--rl-color-status-amber); }
.rl-status-dot--red   { background: var(--rl-color-status-red); }
.rl-status-dot--grey  { background: var(--rl-color-status-grey); }
.rl-status-dot--blue  { background: var(--rl-color-status-blue); }
</style>
