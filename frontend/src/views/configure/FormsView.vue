<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import RlButton from 'rela-components/components/common/RlButton.vue'
import type { TableColumn } from 'rela-components/components/table/types'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigIndex from '@/components/configure/ConfigIndex.vue'
import NewScreenForm from '@/components/configure/NewScreenForm.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { entityTypeLabel, forms } from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { ensureMap, newMap, set } from '@/configure/tree'

/** Every form, and what it creates or edits. */
const draft = useConfigDraftStore()
const router = useRouter()
const adding = ref(false)

const rows = computed(() =>
  forms(draft.currentDataEntry).map((f) => ({
    id: f.name,
    title: f.title,
    entityType: f.entityType ? entityTypeLabel(draft.currentSchema, f.entityType) : '',
    mode: f.mode === 'edit' ? 'Edits' : 'Creates',
    fields: f.fields.length,
  }))
)

const columns: TableColumn[] = [
  { key: 'mode', header: 'Does', field: 'mode', width: 100 },
  { key: 'entityType', header: 'Entity type', field: 'entityType', width: 180 },
  { key: 'fields', header: 'Fields', field: 'fields', width: 90, align: 'end' },
]

function add(id: string, title: string, entityType: string) {
  draft.edit('screens', (tree) =>
    set(
      ensureMap(tree, 'forms'),
      id,
      newMap({ title, entity_type: entityType, mode: 'create', fields: [] })
    )
  )
  adding.value = false
  void router.push(configureRoute.form(id))
}
</script>

<template>
  <ConfigurePage title="Forms">
    <template #actions>
      <RlButton variant="secondary" icon="plus" @click="adding = !adding">New form</RlButton>
    </template>
    <NewScreenForm v-if="adding" section="forms" what="form" @add="add" />
    <ConfigIndex
      :rows="rows"
      :columns="columns"
      name-label="Form"
      intro="A form is how people create or edit a record. The fields and their order are set per form."
      empty="No forms yet."
      :to="(row) => configureRoute.form(row.id)"
    />
  </ConfigurePage>
</template>
