<script setup lang="ts">
/**
 * Row of related actions, such as the Cancel / Create pair under a form.
 *
 * The DOM order is the reading order, so put the confirming action last and
 * let `align` place the row. Keyboard and screen-reader users meet the
 * actions in the order they are written regardless of how they are aligned.
 */
withDefaults(
  defineProps<{
    align?: 'start' | 'end' | 'between'
    /** Stacks the buttons full-width; useful in a narrow drawer. */
    stacked?: boolean
  }>(),
  { align: 'end', stacked: false },
)
</script>

<template>
  <div
    class="rl-button-group"
    :class="[`rl-button-group--${align}`, { 'rl-button-group--stacked': stacked }]"
  >
    <slot />
  </div>
</template>

<style scoped>
.rl-button-group {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
}

.rl-button-group--start { justify-content: flex-start; }
.rl-button-group--end { justify-content: flex-end; }
.rl-button-group--between { justify-content: space-between; }

.rl-button-group--stacked {
  flex-direction: column;
  align-items: stretch;
}
.rl-button-group--stacked :deep(.rl-button) { width: 100%; }
</style>
