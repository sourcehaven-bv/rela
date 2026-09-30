<script setup lang="ts">
/**
 * Placeholder shown while content loads.
 *
 * Preferred over a spinner where the shape of what is coming is known, since
 * it reserves the space and stops the page jumping when the data lands. It is
 * hidden from screen readers: the surrounding region should be marked busy,
 * which is more useful than announcing several grey boxes.
 */
withDefaults(
  defineProps<{
    variant?: 'text' | 'block' | 'circle'
    width?: string
    height?: string
    /** Repeats the shape, for a list of rows. */
    lines?: number
  }>(),
  { variant: 'text', lines: 1 },
)
</script>

<template>
  <div class="rl-skeleton-group" aria-hidden="true">
    <span
      v-for="line in lines"
      :key="line"
      class="rl-skeleton"
      :class="`rl-skeleton--${variant}`"
      :style="{
        width,
        height,
        /* The last line of a paragraph is short, as real text would be. */
        ...(variant === 'text' && lines > 1 && line === lines ? { width: '60%' } : {}),
      }"
    />
  </div>
</template>

<style scoped>
.rl-skeleton-group {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-2);
}

.rl-skeleton {
  display: block;
  border-radius: var(--rl-radius-sm);
  background: linear-gradient(
    90deg,
    var(--rl-color-bg-hover) 25%,
    var(--rl-color-bg-active) 37%,
    var(--rl-color-bg-hover) 63%
  );
  background-size: 400% 100%;
  animation: rl-skeleton-shimmer 1.4s ease infinite;
}

.rl-skeleton--text { height: 12px; }
.rl-skeleton--block { height: 80px; border-radius: var(--rl-radius-md); }
.rl-skeleton--circle { width: 32px; height: 32px; border-radius: 50%; }

@keyframes rl-skeleton-shimmer {
  0% { background-position: 100% 50%; }
  100% { background-position: 0 50%; }
}

@media (prefers-reduced-motion: reduce) {
  /* A moving gradient across a whole page is exactly what this setting is for. */
  .rl-skeleton { animation: none; background: var(--rl-color-bg-hover); }
}
</style>
