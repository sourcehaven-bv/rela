<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import RlButton from 'rela-components/components/common/RlButton.vue'
import type { TableColumn } from 'rela-components/components/table/types'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigIndex from '@/components/configure/ConfigIndex.vue'
import NewScreenForm from '@/components/configure/NewScreenForm.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { entityTypeLabel, lists } from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { ensureMap, newMap, set } from '@/configure/tree'

/** Every list: a table of records of one entity type. */
const draft = useConfigDraftStore()
const router = useRouter()
const adding = ref(false)

const rows = computed(() =>
  lists(draft.currentDataEntry).map((l) => ({
    id: l.name,
    title: l.title,
    entityType: l.entityType ? entityTypeLabel(draft.currentSchema, l.entityType) : '',
    columns: l.columns.length,
  }))
)

const columns: TableColumn[] = [
  { key: 'entityType', header: 'Entity type', field: 'entityType', width: 200 },
  { key: 'columns', header: 'Columns', field: 'columns', width: 100, align: 'end' },
]

function add(id: string, title: string, entityType: string) {
  draft.edit('screens', (tree) =>
    set(
      ensureMap(tree, 'lists'),
      id,
      newMap({ title, entity_type: entityType, columns: [newMap({ property: 'title' })] })
    )
  )
  adding.value = false
  void router.push(configureRoute.list(id))
}
</script>

<template>
  <ConfigurePage title="Lists">
    <template #actions>
      <RlButton variant="secondary" icon="plus" @click="adding = !adding">New list</RlButton>
    </template>
    <NewScreenForm v-if="adding" section="lists" what="list" @add="add" />
    <ConfigIndex
      :rows="rows"
      :columns="columns"
      name-label="List"
      intro="A list shows records of one entity type as a table. Its columns and their order are set per list."
      empty="No lists yet."
      :to="(row) => configureRoute.list(row.id)"
    />
  </ConfigurePage>
</template>
