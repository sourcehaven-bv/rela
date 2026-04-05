<script setup lang="ts">
import EntityList from '@/components/lists/EntityList.vue'

defineProps<{
  id: string
}>()
</script>

<template>
  <EntityList :list-id="id" />
</template>
