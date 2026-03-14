<script setup lang="ts">
import { computed } from 'vue'
import { useSchemaStore } from '@/stores'

const props = defineProps<{
  id: string
}>()

const schemaStore = useSchemaStore()
const listConfig = computed(() => schemaStore.getList(props.id))
</script>

<template>
  <div class="list-view">
    <h1>{{ listConfig?.title || props.id }}</h1>
    <p class="placeholder">List component coming in Phase 3</p>
    <pre v-if="listConfig">{{ JSON.stringify(listConfig, null, 2) }}</pre>
  </div>
</template>

<style scoped>
.list-view {
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

pre {
  background: #f1f5f9;
  padding: 16px;
  border-radius: 8px;
  overflow: auto;
  font-size: 12px;
  margin-top: 16px;
}
</style>
