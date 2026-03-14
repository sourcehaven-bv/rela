<script setup lang="ts">
import EntityDetail from '@/components/entity/EntityDetail.vue'

defineProps<{
  type: string
  id: string
}>()
</script>

<template>
  <EntityDetail :entity-type="type" :entity-id="id" />
</template>
