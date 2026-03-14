<script setup lang="ts">
import { computed } from 'vue'
import { useSchemaStore } from '@/stores'

const props = defineProps<{
  id: string
}>()

const schemaStore = useSchemaStore()
const kanbanConfig = computed(() => schemaStore.getKanban(props.id))
</script>

<template>
  <div class="kanban-view">
    <h1>{{ kanbanConfig?.title || props.id }}</h1>
    <p class="placeholder">Kanban component coming in Phase 9</p>
  </div>
</template>

<style scoped>
.kanban-view {
  max-width: 1400px;
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
