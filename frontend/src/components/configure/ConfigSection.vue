<script setup lang="ts">
import RlSectionHeading from 'rela-components/components/layout/RlSectionHeading.vue'
import RlText from 'rela-components/components/common/RlText.vue'

/** A titled block of a Configure editor, with an optional line saying what it is for. */
defineProps<{ title: string; count?: number; description?: string }>()
</script>

<template>
  <section class="section">
    <RlSectionHeading :title="title" :count="count" :level="2" />
    <RlText v-if="description" size="sm" tone="muted" as="p" class="section__para">{{
      description
    }}</RlText>
    <slot />
  </section>
</template>

<style scoped>
.section {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
}

.section__para {
  margin: 0;
}
</style>
