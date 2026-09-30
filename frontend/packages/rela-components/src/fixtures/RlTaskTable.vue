<script setup lang="ts">
/**
 * `RlTable` wired up for this library's own `Task` row.
 *
 * The table is generic and renders only text for a field, so the tag and
 * priority cells come from slots. That is the consumer's job by design, and
 * this component is what the demo stories and pages use to do it; it is a
 * fixture, not part of the library's surface.
 */
import type { Section, Task } from '../types'
import type { TableColumn } from '../components/table/types'
import RlTable from '../components/table/RlTable.vue'
import RlTag from '../components/common/RlTag.vue'
import RlTagList from '../components/common/RlTagList.vue'
import RlMetaItem from '../components/common/RlMetaItem.vue'

withDefaults(
  defineProps<{
    sections: Section<Task>[]
    columns?: TableColumn[]
    showHeaderRow?: boolean
    selectedId?: string
    cursorId?: string
    pageSize?: number
    showAdd?: boolean
  }>(),
  { columns: () => [], showHeaderRow: true, pageSize: 0, showAdd: true },
)

const emit = defineEmits<{ add: [section: Section<Task>]; select: [item: Task] }>()
</script>

<template>
  <RlTable
    :sections="sections"
    :columns="columns"
    :show-header-row="showHeaderRow"
    :selected-id="selectedId"
    :cursor-id="cursorId"
    :page-size="pageSize"
    :show-add="showAdd"
    add-label="Add task"
    @add="emit('add', $event)"
    @select="emit('select', $event)"
  >
    <template #meta="{ item }">
      <RlMetaItem v-if="item.commentCount" icon="message-square" :label="item.commentCount" countLabel="comments" />
      <RlMetaItem v-if="item.subtaskCount" icon="git-branch" :label="item.subtaskCount" countLabel="subtasks" />
    </template>

    <template #cell-label="{ item }">
      <RlTagList :tags="item.tags ?? []" />
    </template>

    <template #cell-priority="{ item }">
      <RlTag v-if="item.priority" :label="item.priority.label" :color="item.priority.color" />
    </template>

    <!-- Anything the caller passes through, such as an inline status cell. -->
    <template v-for="(_, name) in $slots" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps ?? {}" />
    </template>
  </RlTable>
</template>
