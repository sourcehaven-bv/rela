<script setup lang="ts">
/**
 * Page controls for a list.
 *
 * A thin adapter over RlPagination: rela speaks `meta` + `page-change`, the
 * library speaks `page`/`pageCount` + `update:page`. Keeping the adapter means
 * the call sites and their tests are unchanged by the swap.
 *
 * The H/L shortcut hints the hand-rolled version printed on prev/next are gone
 * — RlPagination has no slot for them. The shortcuts themselves still work;
 * only the printed hint is missing (reported to the library).
 */
import { computed } from 'vue'
import RlPagination from 'rela-components/components/data/RlPagination.vue'
import type { ListMeta } from '@/types'

const props = defineProps<{
  meta: ListMeta
}>()

const emit = defineEmits<{
  'page-change': [page: number]
}>()

const pageCount = computed(() => Math.ceil(props.meta.total / props.meta.per_page))

const summary = computed(() => {
  const first = (props.meta.page - 1) * props.meta.per_page + 1
  const last = Math.min(props.meta.page * props.meta.per_page, props.meta.total)
  return `Showing ${first} - ${last} of ${props.meta.total}`
})

function onPage(page: number) {
  if (page < 1 || page > pageCount.value || page === props.meta.page) return
  emit('page-change', page)
}
</script>

<template>
  <RlPagination
    :page="meta.page"
    :page-count="pageCount"
    :summary="summary"
    @update:page="onPage"
  />
</template>
