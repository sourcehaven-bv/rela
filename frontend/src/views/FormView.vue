<script setup lang="ts">
import DynamicForm from '@/components/forms/DynamicForm.vue'

defineProps<{
  id: string
  entityId?: string
}>()
</script>

<template>
  <!-- One form per form and entity. The router reuses this view when only the
       params change; a reused form kept the previous entity's values and
       pending edits under the new id, and saved them there (BUG-NSGD6OE). -->
  <DynamicForm :key="`${id}/${entityId ?? ''}`" :form-id="id" :entity-id="entityId" />
</template>
