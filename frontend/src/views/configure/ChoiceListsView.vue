<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import type { TableColumn } from 'rela-components/components/table/types'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigIndex from '@/components/configure/ConfigIndex.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { choiceListNames, choiceLists, entityTypeLabel } from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { machineName } from '@/configure/names'
import { ensureMap, newMap, set } from '@/configure/tree'

/** Every choice list: a named set of options a property can take. */
const draft = useConfigDraftStore()
const router = useRouter()
const adding = ref(false)
const label = ref('')

const rows = computed(() =>
  choiceLists(draft.currentSchema, draft.currentDataEntry).map((c) => ({
    id: c.name,
    title: c.label,
    options: c.options.length,
    usedBy: c.usedBy
      .map((u) => `${entityTypeLabel(draft.currentSchema, u.entityType)} › ${u.property}`)
      .join(', '),
  }))
)

const columns: TableColumn[] = [
  { key: 'options', header: 'Options', field: 'options', width: 100, align: 'end' },
  { key: 'usedBy', header: 'Used by', field: 'usedBy', width: 320 },
]

const name = computed(() => machineName(label.value, '_'))
const error = computed(() =>
  label.value.trim() && choiceListNames(draft.currentSchema).includes(name.value)
    ? 'A choice list with this name exists.'
    : undefined
)

function add() {
  if (!label.value.trim() || error.value) return
  const id = name.value
  draft.edit('schema', (tree) => set(ensureMap(tree, 'types'), id, newMap({ values: [] })))
  adding.value = false
  label.value = ''
  void router.push(configureRoute.choiceList(id))
}
</script>

<template>
  <ConfigurePage title="Choice lists">
    <template #actions>
      <RlButton variant="secondary" icon="plus" @click="adding = !adding">New choice list</RlButton>
    </template>
    <form v-if="adding" class="add-row" @submit.prevent="add">
      <RlTextField v-model="label" label="Name of the new choice list" :error="error" />
      <RlButton variant="primary" type="submit" :disabled="!label.trim() || !!error"
        >Add to draft</RlButton
      >
    </form>
    <ConfigIndex
      :rows="rows"
      :columns="columns"
      name-label="Choice list"
      intro="A choice list is a set of options, such as the statuses a ticket can have. Properties use it as their type."
      empty="No choice lists yet."
      :to="(row) => configureRoute.choiceList(row.id)"
    >
      <template #cell-usedBy="{ item }">
        <RlText size="sm" tone="muted">{{ item.usedBy || 'Not used' }}</RlText>
      </template>
    </ConfigIndex>
  </ConfigurePage>
</template>

<style scoped>
.add-row {
  display: flex;
  align-items: flex-end;
  gap: var(--rl-space-3);
  max-width: 600px;
}
</style>
