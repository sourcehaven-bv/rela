<script setup lang="ts">
/** Single-letter square badge marking an initiative's type (R, J, T, D). */
defineProps<{ initial: string }>()
</script>

<template>
  <span class="rl-sidebar-badge">{{ initial }}</span>
</template>

<style scoped>
.rl-sidebar-badge {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  flex: none;
  border: 1px solid var(--rl-color-border-strong);
  border-radius: var(--rl-radius-sm);
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-muted);
}
</style>
