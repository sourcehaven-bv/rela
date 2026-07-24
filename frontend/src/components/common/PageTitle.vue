<script setup lang="ts">
/**
 * PageTitle — h1 + optional subtitle.
 *
 * Subtitle is hidden on mobile (<=768px) so the topbar stays compact.
 * Default slot overrides the title prop for non-string composite titles.
 */
defineProps<{
  title?: string
  subtitle?: string
}>()
</script>

<template>
  <div class="page-title">
    <h1 class="page-title__heading">
      <slot>{{ title }}</slot>
    </h1>
    <p v-if="subtitle" class="page-title__subtitle">{{ subtitle }}</p>
  </div>
</template>

<style scoped>
.page-title {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.page-title__heading {
  margin: 0 0 4px;
}

.page-title__subtitle {
  margin: 0;
  font-size: 14px;
  color: var(--muted-text);
}

@media (max-width: 768px) {
  .page-title__heading {
    font-size: 18px;
    margin: 0;
  }

  .page-title__subtitle {
    display: none;
  }
}
</style>
