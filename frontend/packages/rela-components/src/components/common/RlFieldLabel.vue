<script setup lang="ts">
/**
 * Label in a label/value pair. Renders as `dt` inside a definition list and as
 * a plain span elsewhere, so the markup stays valid in both places.
 */
withDefaults(
  defineProps<{ as?: 'dt' | 'span'; width?: string }>(),
  { as: 'dt' },
)
</script>

<template>
  <component :is="as" class="rl-field-label" :style="width ? { width } : undefined">
    <slot />
  </component>
</template>

<style scoped>
.rl-field-label {
  flex: none;
  font-size: var(--rl-font-size-md);
  color: var(--rl-color-text-muted);
}

@media (max-width: 767px) {
  .rl-field-label { font-size: var(--rl-font-size-sm); }
}
</style>
