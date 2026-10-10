<script setup lang="ts" generic="T extends { id: string; title: string }">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import RlTable from 'rela-components/components/table/RlTable.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import type { Section } from 'rela-components/types'
import type { TableColumn } from 'rela-components/components/table/types'

/**
 * The index of one kind of thing (forms, lists, rules): a table whose rows
 * open the thing's own screen.
 */
const props = defineProps<{
  rows: T[]
  columns: TableColumn[]
  nameLabel: string
  intro?: string
  empty: string
  to: (row: T) => string
}>()

const router = useRouter()
const sections = computed<Section<T>[]>(() => [
  { id: 'all', title: props.nameLabel, items: props.rows },
])

function cell(item: unknown, column: TableColumn): unknown {
  return (item as Record<string, unknown>)[column.field ?? column.key]
}

function open(row: T) {
  void router.push(props.to(row))
}
</script>

<template>
  <div class="index">
    <RlText v-if="intro" tone="muted" size="sm" as="p" class="index__intro">{{ intro }}</RlText>
    <RlEmptyState v-if="!rows.length" :title="empty" />
    <RlTable
      v-else
      :sections="sections"
      :columns="columns"
      :name-label="nameLabel"
      :show-section-header="false"
      :show-add="false"
      @select="open"
    >
      <template v-for="column in columns" :key="column.key" #[`cell-${column.key}`]="{ item }">
        <slot :name="`cell-${column.key}`" :item="item">
          <RlText size="sm" tone="muted">{{ cell(item, column) }}</RlText>
        </slot>
      </template>
    </RlTable>
  </div>
</template>

<style scoped>
.index {
  max-width: 1000px;
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
}

.index__intro {
  margin: 0;
}
</style>
