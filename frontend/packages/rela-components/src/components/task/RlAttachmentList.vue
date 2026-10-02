<script setup lang="ts">
/** Attachment cards side by side, wrapping onto new rows as needed. */
</script>

<template>
  <div class="rl-attachment-list"><slot /></div>
</template>

<style scoped>
.rl-attachment-list {
  display: flex;
  flex-wrap: wrap;
  gap: var(--rl-space-3);
}
</style>
