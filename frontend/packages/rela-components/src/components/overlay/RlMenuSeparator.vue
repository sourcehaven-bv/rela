<script setup lang="ts">
/**
 * Divider between groups of menu items. `role="separator"` so it is announced
 * as a group boundary rather than skipped silently.
 */
</script>

<template>
  <hr class="rl-menu-separator" role="separator" />
</template>

<style scoped>
.rl-menu-separator {
  margin: var(--rl-space-1) 0;
  border: none;
  border-top: 1px solid var(--rl-color-border);
}
</style>
