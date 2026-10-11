<script setup lang="ts">
/**
 * The two-column layout every Configure editor uses: the editor on the left,
 * and beside it either a narrow column of settings or a wider preview. Below
 * 900px the columns stack.
 */
withDefaults(defineProps<{ aside?: 'settings' | 'preview' }>(), { aside: 'settings' })
</script>

<template>
  <div class="split" :class="`split--${aside}`">
    <div class="split__main"><slot /></div>
    <div class="split__aside"><slot name="aside" /></div>
  </div>
</template>

<style scoped>
.split {
  display: grid;
  gap: 40px;
  align-items: start;
}

.split--settings {
  grid-template-columns: minmax(0, 1fr) 320px;
  max-width: 1100px;
}

.split--preview {
  grid-template-columns: minmax(360px, 1fr) minmax(0, 1.2fr);
}

@media (max-width: 900px) {
  .split--settings,
  .split--preview {
    grid-template-columns: minmax(0, 1fr);
  }
}

.split__main,
.split__aside {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-8);
  min-width: 0;
}
</style>
