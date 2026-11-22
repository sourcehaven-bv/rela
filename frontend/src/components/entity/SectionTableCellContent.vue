<script setup lang="ts">
/**
 * The read-only content of one `display: table` section cell: the value, inside
 * a link when the column links. Shared by the plain and the editable rows, so
 * the two render a read-only cell the same way.
 */
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import type { ViewCell, ViewRow } from '@/api/views'
import { entityRef } from '@/utils/entityRef'
import { safeInternalHref } from '@/utils/openIntent'
import { isSectionCellEmpty } from '@/utils/sectionTable'
import ViewTableCell from './ViewTableCell.vue'

const props = defineProps<{
  cell: ViewCell
  property?: string
  row: ViewRow
}>()

// The server sends the link as a bare path. The router resolves it into the
// current space, so a new tab or a copied address opens the same page. An
// empty cell gets no link: an empty anchor is an invisible tab stop
// (BUG-6WQ1U8S).
const router = useRouter()
const href = computed(() => {
  const link = props.cell.link
  if (!link || !safeInternalHref(link) || isSectionCellEmpty(props.cell)) return undefined
  return router.resolve(link).href
})

const emit = defineEmits<{
  'link-click': [event: MouseEvent, target: { id: string; type: string }, link: string]
}>()
</script>

<template>
  <a
    v-if="href"
    :href="href"
    @click="
      emit(
        'link-click',
        $event,
        { id: cell.entityId || row.entityId, type: cell.entityType || row.entityType },
        cell.link!
      )
    "
  >
    <ViewTableCell :values="cell.values" :property="property" :entity-type="row.entityType" />
  </a>
  <ViewTableCell
    v-else
    :values="cell.values"
    :property="property"
    :entity-type="row.entityType"
    :entity-id="entityRef({ id: row.entityId, _self: row._self })"
  />
</template>
