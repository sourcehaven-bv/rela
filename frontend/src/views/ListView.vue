<script setup lang="ts">
import EntityList from '@/components/lists/EntityList.vue'
import type { PageScope } from '@/types'

defineProps<{
  id: string
  pageScope?: PageScope
}>()
</script>

<template>
  <EntityList :list-id="id" :page-scope="pageScope" />
</template>
