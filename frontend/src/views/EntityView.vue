<script setup lang="ts">
const props = defineProps<{
  type: string
  id: string
}>()
</script>

<template>
  <div class="entity-view">
    <h1>{{ props.type }}: {{ props.id }}</h1>
    <p class="placeholder">Entity detail component coming in Phase 5</p>
  </div>
</template>

<style scoped>
.entity-view {
  max-width: 800px;
}

h1 {
  margin-bottom: 16px;
}

.placeholder {
  color: #666;
  font-style: italic;
  margin-top: 24px;
}
</style>
