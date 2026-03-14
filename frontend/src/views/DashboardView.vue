<script setup lang="ts">
import { useSchemaStore } from '@/stores'

const schemaStore = useSchemaStore()
</script>

<template>
  <div class="dashboard-view">
    <h1>Dashboard</h1>
    <p>Welcome to {{ schemaStore.app.name }}</p>
    <p class="placeholder">Dashboard components coming in Phase 7</p>
  </div>
</template>

<style scoped>
.dashboard-view {
  max-width: 1200px;
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
