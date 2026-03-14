<script setup lang="ts">
import { computed } from 'vue'
import { useSchemaStore } from '@/stores'

const props = defineProps<{
  id: string
  entityId: string
}>()

const schemaStore = useSchemaStore()
const viewConfig = computed(() => schemaStore.getView(props.id))
</script>

<template>
  <div class="custom-view">
    <h1>{{ viewConfig?.title || props.id }}: {{ props.entityId }}</h1>
    <p class="placeholder">Custom view component coming in Phase 8</p>
  </div>
</template>

<style scoped>
.custom-view {
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
