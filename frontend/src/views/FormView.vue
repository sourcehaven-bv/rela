<script setup lang="ts">
import DynamicForm from '@/components/forms/DynamicForm.vue'

defineProps<{
  id: string
  entityId?: string
}>()
</script>

<template>
  <DynamicForm :form-id="id" :entity-id="entityId" />
</template>
