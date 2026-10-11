<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import RlTable from 'rela-components/components/table/RlTable.vue'
import RlSearchBox from 'rela-components/components/data/RlSearchBox.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlTag from 'rela-components/components/common/RlTag.vue'
import type { Section } from 'rela-components/types'
import type { TableColumn } from 'rela-components/components/table/types'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import NewEntityTypeDrawer from '@/components/configure/NewEntityTypeDrawer.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { entityTypes } from '@/configure/models'
import { configureRoute } from '@/configure/routes'

/** Every entity type, the place an operator starts. */
const draft = useConfigDraftStore()
const router = useRouter()
const query = ref('')
const creating = ref(false)

interface Row {
  id: string
  title: string
  ids: string
  properties: number
  relations: number
  added: boolean
}

const rows = computed<Row[]>(() =>
  entityTypes(draft.currentSchema).map((t) => ({
    id: t.name,
    title: t.label,
    ids:
      t.idType === 'manual'
        ? 'Chosen by hand'
        : t.idType === 'sequential'
          ? `${t.idPrefix}1, ${t.idPrefix}2`
          : `${t.idPrefix}X8K2PQ`,
    properties: t.properties.length,
    relations: t.relationCount,
    added: !isInBase(t.name),
  }))
)

function isInBase(name: string): boolean {
  return entityTypes(draft.base?.schema).some((t) => t.name === name)
}

const sections = computed<Section<Row>[]>(() => {
  const q = query.value.trim().toLowerCase()
  const items = rows.value.filter(
    (r) => !q || r.title.toLowerCase().includes(q) || r.id.includes(q)
  )
  return [{ id: 'all', title: 'Entity types', items }]
})

const columns: TableColumn[] = [
  { key: 'ids', header: 'IDs look like', width: 180 },
  { key: 'properties', header: 'Properties', width: 110, field: 'properties', align: 'end' },
  { key: 'relations', header: 'Relations', width: 110, field: 'relations', align: 'end' },
]

function open(row: Row) {
  void router.push(configureRoute.entityType(row.id))
}
</script>

<template>
  <ConfigurePage title="Entity types">
    <template #actions>
      <RlButton variant="secondary" icon="plus" @click="creating = true">New entity type</RlButton>
    </template>
    <div class="entity-types">
      <RlText tone="muted" size="sm" as="p" class="entity-types__intro">
        An entity type is a kind of record, such as a ticket or a decision. It decides which
        properties a record has and how its ID is made.
      </RlText>
      <RlSearchBox
        v-model="query"
        placeholder="Find an entity type"
        size="sm"
        class="entity-types__search"
      />
      <RlTable
        :sections="sections"
        :columns="columns"
        name-label="Entity type"
        :show-section-header="false"
        :show-add="false"
        data-testid="config-entity-types"
        @select="open"
      >
        <template #cell-ids="{ item }">
          <span class="entity-types__ids">
            <RlText size="sm" tone="muted">{{ item.ids }}</RlText>
            <RlTag v-if="item.added" label="New" color="green" />
          </span>
        </template>
      </RlTable>
    </div>
    <NewEntityTypeDrawer :open="creating" @close="creating = false" />
  </ConfigurePage>
</template>

<style scoped>
.entity-types {
  max-width: 900px;
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
}

.entity-types__intro {
  margin: 0;
}

.entity-types__search {
  max-width: 320px;
}

.entity-types__ids {
  display: inline-flex;
  gap: var(--rl-space-2);
}
</style>
