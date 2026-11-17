<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlTag from 'rela-components/components/common/RlTag.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import type { TableColumn } from 'rela-components/components/table/types'
import ConfigurePage from './ConfigurePage.vue'
import ConfigIndex from '@/components/configure/ConfigIndex.vue'
import { useConfigDraftStore } from '@/configure/configDraft'
import { automations, automationTitle, entityTypeLabel, entityTypeNames } from '@/configure/models'
import { configureRoute } from '@/configure/routes'
import { get, has, newMap } from '@/configure/tree'

/**
 * Every automation: something that happens by itself when a record changes.
 * They are the `triggers:` of flows.yaml and fire in the order listed.
 */
const draft = useConfigDraftStore()
const router = useRouter()
const { t } = useI18n()

const rows = computed(() =>
  automations(draft.currentFlows).map((a) => ({
    id: a.name,
    name: a.name,
    title: automationTitle(a),
    when: `${a.entityTypes.map((type) => entityTypeLabel(draft.currentSchema, type)).join(', ') || 'Any record'}: ${a.trigger}`,
    then: a.outputs.map((x) => x.summary).join('; '),
    readOnly: a.readOnly,
  }))
)

const columns = computed<TableColumn[]>(() => [
  { key: 'when', header: t('configureAutomations.when'), field: 'when', width: 300 },
  { key: 'then', header: t('configureAutomations.then'), width: 380 },
])

function routeOf(row: { name: string }): string {
  return configureRoute.automation(row.name)
}

/** A trigger name not in use yet. */
function freshName(): string {
  const triggers = get(draft.currentFlows, 'triggers')
  let name = 'new-automation'
  for (let i = 2; has(triggers, name); i++) name = `new-automation-${i}`
  return name
}

function add() {
  const type = entityTypeNames(draft.currentSchema)[0]
  const name = freshName()
  draft.setAt(
    'flows',
    ['triggers'],
    name,
    newMap({ on: newMap({ entity: type ? [type] : [], created: true }), outputs: [] })
  )
  void router.push(configureRoute.automation(name))
}
</script>

<template>
  <ConfigurePage :title="t('configureAutomations.title')">
    <template #actions>
      <RlButton variant="secondary" icon="plus" @click="add">{{
        t('configureAutomations.new')
      }}</RlButton>
    </template>
    <ConfigIndex
      :rows="rows"
      :columns="columns"
      :name-label="t('configureAutomations.one')"
      :intro="t('configureAutomations.intro')"
      :empty="t('configureAutomations.empty')"
      :to="routeOf"
    >
      <template #cell-then="{ item }">
        <RlText size="sm" tone="muted">{{
          item.then || t('configureAutomations.nothingYet')
        }}</RlText>
        <RlTag v-if="item.readOnly" :label="t('configureAutomations.readOnly')" />
      </template>
    </ConfigIndex>
  </ConfigurePage>
</template>
