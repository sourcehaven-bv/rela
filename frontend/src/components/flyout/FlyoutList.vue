<script setup lang="ts">
/**
 * A configured list as the rows of a sidebar flyout: titles only, with the
 * list's first enum column as trailing meta.
 *
 * Deliberately not EntityList. The flyout is a quick look over the page, in
 * a panel a third the width of the page, so it carries no filters, search,
 * sort, bulk actions or columns; the expand control leads to the full list
 * for any of those. What it does share is the READ: the same base params
 * (static filters, `condition:`, scope, default sort) through the same query
 * key, so the flyout holds exactly the rows the page would, and live updates
 * reach both.
 *
 * A list with `group_by:` shows its rows under section headings, in the
 * page's order. The flyout still loads one page of FLYOUT_ROWS rather than the
 * page's whole grouped set, so its sections are those of the first rows and
 * the "shown of total" note says when rows were left out. Sections no loaded
 * row falls in are left out: an empty heading in a quick look is noise. In a
 * date-bucketed list the trailing meta says when within the bucket (see
 * `bucketMeta`) instead of repeating the enum column.
 */
import { computed } from 'vue'
import { useQuery } from '@pinia/colada'
import { useSchemaStore, useUIStore } from '@/stores'
import { listEntities, getErrorMessage } from '@/api'
import { entityKeys } from '@/queries/entities'
import { useWorld } from '@/composables/useWorld'
import { useFlyout } from '@/composables/useFlyout'
import { useListGrouping, type ListSection } from '@/composables/useListGrouping'
import { bucketMeta } from '@/utils/dateBuckets'
import { listBaseParams } from '@/utils/listParams'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { formatCellValue, getCellValue } from '@/utils/format'
import { densePropertyRoutingHint } from '@/widgets/viewRouting'
import type { ListColumn, ListParams } from '@/types'
import RlPanelListRow from 'rela-components/components/layout/RlPanelListRow.vue'
import RlStatusRegion from 'rela-components/components/feedback/RlStatusRegion.vue'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlSectionHeading from 'rela-components/components/layout/RlSectionHeading.vue'

const props = defineProps<{ listId: string }>()

/** Enough to scan; the expand control is the way to the rest. */
const FLYOUT_ROWS = 50

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const flyout = useFlyout()
const { worldParam } = useWorld()

const listConfig = computed(() => schemaStore.getList(props.listId))
const entityType = computed(() =>
  listConfig.value ? schemaStore.getEntityType(listConfig.value.entity) : undefined,
)

const params = computed((): ListParams | null => {
  const config = listConfig.value
  if (!config) return null
  const result: ListParams = { ...listBaseParams(props.listId, config), page: 1, per_page: FLYOUT_ROWS }
  if (worldParam.value) result.world = worldParam.value
  return result
})

const query = useQuery({
  key: () => entityKeys.listParams(listConfig.value?.entity ?? '', params.value ?? {}),
  query: () => {
    const config = listConfig.value
    if (!config || !params.value) throw new Error(`unknown list: ${props.listId}`)
    return listEntities(config.entity, params.value)
  },
  enabled: () => params.value !== null,
})

const rows = computed(() => query.data.value?.data ?? [])

const grouping = useListGrouping({
  listId: () => props.listId,
  groupBy: () => listConfig.value?.group_by,
  entityType: () => entityType.value,
  response: () => query.data.value,
  includeEmpty: false,
})
const total = computed(() => query.data.value?.meta.total ?? 0)
const loadError = computed(() =>
  query.error.value ? getErrorMessage(query.error.value, 'Failed to load the list') : null,
)

/*
 * The state a reader scans a list for, which is what an enum column holds:
 * status, priority. Picked the way the page's compressed layout picks the
 * columns it keeps beside a panel, so the two views agree on what matters.
 */
const metaColumn = computed<ListColumn | undefined>(() =>
  listConfig.value?.columns.find((column) => {
    if (!column.property) return false
    const def = entityType.value?.properties[column.property]
    const kind = def ? densePropertyRoutingHint(def, column.property).kind : undefined
    return kind === 'enum' || kind === 'enum-list'
  }),
)

function metaOf(entity: (typeof rows.value)[number], section?: ListSection): string | undefined {
  const property = listConfig.value?.group_by?.property
  if (section?.bucket && property) {
    const isDate = entityType.value?.properties[property]?.type === 'date'
    return bucketMeta(
      entity.properties[property],
      section.bucket,
      uiStore.effectiveTimezone,
      undefined,
      isDate,
    )
  }
  const column = metaColumn.value
  if (!column) return undefined
  return formatCellValue(getCellValue(entity, column), column.property, entityType.value) || undefined
}
</script>

<template>
  <RlStatusRegion v-if="loadError" tone="error">{{ loadError }}</RlStatusRegion>
  <RlStatusRegion v-else-if="query.isPending.value">Loading...</RlStatusRegion>
  <RlEmptyState
    v-else-if="rows.length === 0"
    size="sm"
    :title="`No ${listConfig?.title ?? 'entries'}`"
  />
  <div v-else class="flyout-list" :data-list-id="listId">
    <!--
      Headings sit directly in the list, not in a wrapper per section: the
      panel aligns a heading with its rows only when it is at most one level
      below the panel body.
    -->
    <template v-for="section in grouping.sections.value" :key="section.id">
      <RlSectionHeading
        :title="section.title"
        :color="section.color"
        :count="section.items.length"
        size="sm"
        :level="3"
        :data-section-id="section.id"
      />
      <RlPanelListRow
        v-for="entity in section.items"
        :key="entity.id"
        :label="entityDisplayTitle(entity)"
        :meta="metaOf(entity, section)"
        :meta-highlight="section.bucket === 'overdue'"
        :selected="flyout.entity.value?.id === entity.id"
        :data-entity-id="entity.id"
        @select="flyout.openEntity(entity)"
      />
    </template>
    <RlPanelListRow
      v-for="entity in grouping.grouped.value ? [] : rows"
      :key="entity.id"
      :label="entityDisplayTitle(entity)"
      :meta="metaOf(entity)"
      :selected="flyout.entity.value?.id === entity.id"
      :data-entity-id="entity.id"
      @select="flyout.openEntity(entity)"
    />
    <RlText v-if="total > rows.length" as="p" tone="subtle" class="flyout-list__more">
      {{ rows.length }} of {{ total }} shown
    </RlText>
  </div>
</template>

<style scoped>
.flyout-list__more {
  padding: var(--rl-space-3) var(--rl-space-5);
}
</style>
