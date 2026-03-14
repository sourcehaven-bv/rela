<script setup lang="ts">
import { computed } from 'vue'
import { useSchemaStore } from '@/stores'

const props = defineProps<{
  id: string
  entityId?: string
}>()

const schemaStore = useSchemaStore()
const formConfig = computed(() => schemaStore.getForm(props.id))
const isEdit = computed(() => !!props.entityId)
</script>

<template>
  <div class="form-view">
    <h1>{{ isEdit ? 'Edit' : 'Create' }} {{ formConfig?.title || props.id }}</h1>
    <p class="placeholder">Form component coming in Phase 4</p>
    <p v-if="isEdit">Entity ID: {{ props.entityId }}</p>
    <pre v-if="formConfig">{{ JSON.stringify(formConfig, null, 2) }}</pre>
  </div>
</template>

<style scoped>
.form-view {
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

pre {
  background: #f1f5f9;
  padding: 16px;
  border-radius: 8px;
  overflow: auto;
  font-size: 12px;
  margin-top: 16px;
}
</style>
