<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlTag from 'rela-components/components/common/RlTag.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import type { TableColumn } from 'rela-components/components/table/types'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigIndex from '@/components/configure/ConfigIndex.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { automations, automationTitle, entityTypeLabel, entityTypeNames } from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { listAt, newMap } from '@/configure/tree'

/** Every automation: something that happens by itself when a record changes. */
const draft = useConfigDraftStore()
const router = useRouter()

const rows = computed(() =>
  automations(draft.currentSchema).map((a) => ({
    id: String(a.index),
    index: a.index,
    title: automationTitle(a),
    when: `${a.entityTypes.map((t) => entityTypeLabel(draft.currentSchema, t)).join(', ') || 'Any record'}: ${a.trigger}`,
    then: a.actions.map((x) => x.summary).join('; '),
    readOnly: a.readOnly,
  }))
)

const columns: TableColumn[] = [
  { key: 'when', header: 'When', field: 'when', width: 300 },
  { key: 'then', header: 'Then', width: 380 },
]

function add() {
  const type = entityTypeNames(draft.currentSchema)[0]
  draft.appendAt(
    'schema',
    [],
    'automations',
    newMap({ name: 'new_automation', on: newMap({ entity: type ? [type] : [], created: true }) })
  )
  const count = listAt(draft.currentSchema, ['automations'])?.length ?? 1
  void router.push(configureRoute.automation(count - 1))
}
</script>

<template>
  <ConfigurePage title="Automations">
    <template #actions>
      <RlButton variant="secondary" icon="plus" @click="add">New automation</RlButton>
    </template>
    <ConfigIndex
      :rows="rows"
      :columns="columns"
      name-label="Automation"
      intro="An automation does something by itself when a record is created or changes, such as creating a checklist."
      empty="No automations yet."
      :to="(row) => configureRoute.automation(row.index)"
    >
      <template #cell-then="{ item }">
        <RlText size="sm" tone="muted">{{ item.then || 'Nothing yet' }}</RlText>
        <RlTag v-if="item.readOnly" label="Read only" />
      </template>
    </ConfigIndex>
  </ConfigurePage>
</template>
