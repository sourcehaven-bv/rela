<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import RlButton from 'rela-components/components/common/RlButton.vue'
import type { TableColumn } from 'rela-components/components/table/types'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigIndex from '@/components/configure/ConfigIndex.vue'
import NewScreenForm from '@/components/configure/NewScreenForm.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { boards, entityTypeLabel, propertiesOf } from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { ensureMap, newMap, set } from '@/configure/tree'

/** Every board: records of one entity type in columns by a choice. */
const draft = useConfigDraftStore()
const router = useRouter()
const adding = ref(false)

const rows = computed(() =>
  boards(draft.currentDataEntry).map((b) => ({
    id: b.name,
    title: b.title,
    entityType: b.entityType ? entityTypeLabel(draft.currentSchema, b.entityType) : '',
    columnsBy: b.columnProperty,
  }))
)

const columns: TableColumn[] = [
  { key: 'entityType', header: 'Entity type', field: 'entityType', width: 200 },
  { key: 'columnsBy', header: 'Columns by', field: 'columnsBy', width: 160 },
]

function add(id: string, title: string, entityType: string) {
  const choice = propertiesOf(draft.currentSchema, entityType).find(
    (p) => p.choiceList || p.inlineValues.length
  )
  draft.edit('screens', (tree) =>
    set(
      ensureMap(tree, 'kanbans'),
      id,
      newMap({
        title,
        entity_type: entityType,
        column_property: choice?.name ?? 'status',
        card: newMap({ fields: [] }),
      })
    )
  )
  adding.value = false
  void router.push(configureRoute.board(id))
}
</script>

<template>
  <ConfigurePage title="Boards">
    <template #actions>
      <RlButton variant="secondary" icon="plus" @click="adding = !adding">New board</RlButton>
    </template>
    <NewScreenForm v-if="adding" section="kanbans" what="board" @add="add" />
    <ConfigIndex
      :rows="rows"
      :columns="columns"
      name-label="Board"
      intro="A board shows records as cards in columns, one column per option of a choice, such as status."
      empty="No boards yet."
      :to="(row) => configureRoute.board(row.id)"
    />
  </ConfigurePage>
</template>
