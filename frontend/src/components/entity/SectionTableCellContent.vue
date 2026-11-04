<script setup lang="ts">
/**
 * The read-only content of one `display: table` section cell: the value, inside
 * a link when the column links. Shared by the plain and the editable rows, so
 * the two render a read-only cell the same way.
 */
import type { ViewCell, ViewRow } from '@/api/views'
import { entityRef } from '@/utils/entityRef'
import ViewTableCell from './ViewTableCell.vue'

defineProps<{
  cell: ViewCell
  property?: string
  row: ViewRow
}>()

const emit = defineEmits<{
  'link-click': [event: MouseEvent, target: { id: string; type: string }, link: string]
}>()
</script>

<template>
  <a
    v-if="cell.link"
    :href="cell.link"
    @click="
      emit(
        'link-click',
        $event,
        { id: cell.entityId || row.entityId, type: cell.entityType || row.entityType },
        cell.link
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
