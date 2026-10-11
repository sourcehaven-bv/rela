<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlTag from 'rela-components/components/common/RlTag.vue'
import type { TableColumn } from 'rela-components/components/table/types'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigIndex from '@/components/configure/ConfigIndex.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { entityTypeLabel, entityTypeNames, rules, ruleTitle } from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { listAt, newMap } from '@/configure/tree'

/** Every rule: a check records must pass, or a warning they show. */
const draft = useConfigDraftStore()
const router = useRouter()

const rows = computed(() =>
  rules(draft.currentSchema).map((r) => ({
    id: String(r.index),
    index: r.index,
    title: ruleTitle(r),
    appliesTo: r.entityType ? entityTypeLabel(draft.currentSchema, r.entityType) : 'Every record',
    severity: r.severity,
    script: r.usesScript,
  }))
)

const columns: TableColumn[] = [
  { key: 'appliesTo', header: 'Applies to', field: 'appliesTo', width: 160 },
  { key: 'severity', header: 'When broken', width: 180 },
]

function add() {
  const type = entityTypeNames(draft.currentSchema)[0]
  draft.appendAt(
    'schema',
    [],
    'validations',
    newMap({ name: 'new_rule', entity_type: type, severity: 'warning' })
  )
  const count = listAt(draft.currentSchema, ['validations'])?.length ?? 1
  void router.push(configureRoute.rule(count - 1))
}
</script>

<template>
  <ConfigurePage title="Rules">
    <template #actions>
      <RlButton variant="secondary" icon="plus" @click="add">New rule</RlButton>
    </template>
    <ConfigIndex
      :rows="rows"
      :columns="columns"
      name-label="Rule"
      intro="A rule is a check on records. A broken rule either blocks saving or shows a warning on the record."
      empty="No rules yet."
      :to="(row) => configureRoute.rule(row.index)"
    >
      <template #cell-severity="{ item }">
        <RlTag
          :label="item.severity === 'error' ? 'Blocks saving' : 'Shows a warning'"
          :color="item.severity === 'error' ? 'red' : 'amber'"
        />
        <RlTag v-if="item.script" label="Runs a script" />
      </template>
    </ConfigIndex>
  </ConfigurePage>
</template>
