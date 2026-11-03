<script setup lang="ts">
/**
 * The cells of one `display: table` section row, editable or not
 * (TKT-4Q2EIVZ). A fragment of `<td>`s, so the host keeps its own `<tr>`
 * (plain or reorderable). Editable rows go through {@link EditableRowCells};
 * both kinds render a read-only cell with {@link SectionTableCellContent}.
 */
import { useI18n } from 'vue-i18n'
import { Pencil } from 'lucide-vue-next'
import type { ViewColumn, ViewRow } from '@/api/views'
import type { AutoSaveErrorInfo } from '@/composables/useAutoSave'
import type { AutoSaveOwner } from '@/composables/useSectionAutoSave'
import AutoSaveIndicator from '@/components/forms/AutoSaveIndicator.vue'
import EditableRowCells from './EditableRowCells.vue'
import SectionTableCellContent from './SectionTableCellContent.vue'

defineProps<{
  row: ViewRow
  columns: ViewColumn[]
  editable: boolean
  onPropertyApplied: (prop: string, value: unknown, owner: AutoSaveOwner) => void
  onError: (msg: string, info?: AutoSaveErrorInfo) => void
  onVerdictFlip?: (prop: string, label: string) => void
  runWrite?: <T>(send: () => Promise<T>) => Promise<T>
}>()

const emit = defineEmits<{
  'link-click': [event: MouseEvent, target: { id: string; type: string }, link: string]
  edit: [formId: string, row: { id: string; _self?: string }]
}>()

const { t } = useI18n()
</script>

<template>
  <EditableRowCells
    v-if="editable"
    :row="row"
    :columns="columns"
    :on-property-applied="onPropertyApplied"
    :on-error="onError"
    :on-verdict-flip="onVerdictFlip"
    :run-write="runWrite"
  >
    <template #cell="{ cell, index }">
      <SectionTableCellContent
        :cell="cell"
        :property="columns[index]?.property"
        :row="row"
        @link-click="(e, target, link) => emit('link-click', e, target, link)"
      />
    </template>
    <template #actions="{ status, error }">
      <td class="actions-cell">
        <AutoSaveIndicator :status="status" :error="error" />
        <button
          v-if="row.editFormId"
          class="icon-btn"
          :title="t('common.edit')"
          :aria-label="t('common.edit')"
          @click="emit('edit', row.editFormId, { id: row.entityId, _self: row._self })"
        >
          <Pencil :size="14" aria-hidden="true" />
        </button>
      </td>
    </template>
  </EditableRowCells>
  <template v-else>
    <td v-for="(cell, idx) in row.cells" :key="idx">
      <SectionTableCellContent
        :cell="cell"
        :property="columns[idx]?.property"
        :row="row"
        @link-click="(e, target, link) => emit('link-click', e, target, link)"
      />
    </td>
    <td class="actions-cell">
      <button
        v-if="row.editFormId"
        class="icon-btn"
        :title="t('common.edit')"
        :aria-label="t('common.edit')"
        @click="emit('edit', row.editFormId, { id: row.entityId, _self: row._self })"
      >
        <Pencil :size="14" aria-hidden="true" />
      </button>
    </td>
  </template>
</template>

<style scoped>
.actions-cell {
  text-align: center;
  white-space: nowrap;
}

.icon-btn {
  background: none;
  border: none;
  color: var(--rl-color-text-muted);
  cursor: pointer;
  padding: 4px 8px;
  font-size: var(--font-size-base);
  border-radius: var(--radius-sm);
}

.icon-btn:hover {
  background: var(--rl-color-bg-hover);
  color: var(--rl-color-text);
}
</style>
