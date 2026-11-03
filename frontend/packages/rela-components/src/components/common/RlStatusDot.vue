<script setup lang="ts">
import { computed } from 'vue'
import type { DotColor } from '../../types'

const props = withDefaults(defineProps<{ color?: DotColor; size?: number }>(), {
  color: 'grey',
  size: 8,
})

// A palette property paints inline; a named colour uses its class. A
// property may be a light shade close to the page, so it gets a hairline.
const custom = computed(() => props.color.startsWith('--rl-'))
</script>

<template>
  <span
    class="rl-status-dot"
    :class="custom ? undefined : `rl-status-dot--${color}`"
    :style="{ width: `${size}px`, height: `${size}px`, ...(custom ? { background: `var(${color})`, boxShadow: 'inset 0 0 0 1px var(--rl-color-border)' } : {}) }"
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
/* The accent, as the app's purple badge uses. */
.rl-status-dot--purple { background: var(--rl-color-accent); }
</style>
