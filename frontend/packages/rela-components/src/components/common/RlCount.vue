<script setup lang="ts">
/** Subdued count shown beside a section or column title. */
defineProps<{ value: number }>()
</script>

<template>
  <span class="rl-count">{{ value }}</span>
</template>

<style scoped>
.rl-count {
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-subtle);
}
</style>
