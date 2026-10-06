<script setup lang="ts">
import { computed } from 'vue'
import RlTag from 'rela-components/components/common/RlTag.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import type { TableColumn } from 'rela-components/components/table/types'
import ConfigurePage from '@/components/configure/ConfigurePage.vue'
import ConfigIndex from '@/components/configure/ConfigIndex.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { relations, relationSentence } from '@/configure/models'
import { configureRoute } from '@/configure/routes'

/** Every relation, as the sentence it reads as. */
const draft = useConfigDraftStore()

const rows = computed(() =>
  relations(draft.currentSchema).map((r) => ({
    id: r.name,
    title: r.label,
    reads: relationSentence(draft.currentSchema, r),
    atLeast: r.minOutgoing ?? 0,
  }))
)

const columns: TableColumn[] = [
  { key: 'reads', header: 'Reads as', field: 'reads', width: 420 },
  { key: 'atLeast', header: 'Required', width: 120 },
]
</script>

<template>
  <ConfigurePage title="Relations">
    <ConfigIndex
      :rows="rows"
      :columns="columns"
      name-label="Relation"
      intro="A relation links two records, such as a ticket that implements a feature."
      empty="No relations yet."
      :to="(row) => configureRoute.relation(row.id)"
    >
      <template #cell-atLeast="{ item }">
        <RlTag v-if="item.atLeast" :label="`at least ${item.atLeast}`" color="blue" />
        <RlText v-else size="sm" tone="subtle">Optional</RlText>
      </template>
    </ConfigIndex>
  </ConfigurePage>
</template>
