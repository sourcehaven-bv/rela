import { computed, type Ref } from 'vue'
import { useQuery } from '@pinia/colada'
import { getEntityRelations, listAllEntities } from '@/api'
import { useSchemaStore } from '@/stores'
import { usePageStore } from '@/stores/pages'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { ORDER_PROPERTY_OUT } from '@/types/schema'
import type { Entity, KanbanColumnsFrom, PageScope } from '@/types'

/** The column for cards whose target is not among the board's columns. */
export const OTHER_COLUMN = '__other__'

export interface RelationColumn {
  value: string
  label: string
  /** The target entity; absent for the Other column. */
  entity?: Entity
}

function orderValue(v: unknown): number {
  const n = typeof v === 'number' ? v : Number(v)
  return Number.isFinite(n) ? n : Number.POSITIVE_INFINITY
}

/**
 * The columns of a relation-backed board (`columns_from`, TKT-KJ3Q07).
 *
 * On a page tab with `offered_by`, the columns are the targets the page's
 * entity offers, in that relation's `_order_out` order. Otherwise they are
 * every entity of the target type, ordered by `order_by` and then by title.
 */
export function useRelationColumns(
  columnsFrom: Ref<KanbanColumnsFrom | undefined>,
  pageScope: Ref<PageScope | undefined>,
  worldParam: Ref<string | undefined>,
) {
  const schemaStore = useSchemaStore()
  const pageStore = usePageStore()

  const targetType = computed(() => {
    const rel = columnsFrom.value?.relation
    return rel ? schemaStore.getRelationType(rel)?.to?.[0] : undefined
  })

  const anchor = computed(() => {
    const s = pageScope.value
    const offeredBy = columnsFrom.value?.offered_by
    if (!s || !offeredBy) return undefined
    const type = pageStore.pages[s.page]?.entity_type
    return type ? { type, id: s.ref ?? s.entity, relation: offeredBy } : undefined
  })

  const query = useQuery({
    key: () => [
      'entities',
      targetType.value ?? '',
      'relation-columns',
      anchor.value?.type ?? '',
      anchor.value?.id ?? '',
      anchor.value?.relation ?? '',
      worldParam.value ?? '',
    ],
    enabled: () => !!targetType.value,
    query: async ({ signal }) => {
      const type = targetType.value as string
      const params = worldParam.value ? { world: worldParam.value } : undefined
      const all = (await listAllEntities(type, params, signal)).data
      const a = anchor.value
      if (!a) return { targets: all, offered: undefined }
      const entries = await getEntityRelations(a.type, a.id, a.relation)
      return { targets: all, offered: entries }
    },
  })

  const columns = computed<RelationColumn[]>(() => {
    const data = query.data.value
    if (!data) return []
    const byId = new Map(data.targets.map((e) => [e.id, e]))
    const toColumn = (e: Entity): RelationColumn => ({
      value: e.id,
      label: entityDisplayTitle(e),
      entity: e,
    })

    if (data.offered) {
      return data.offered
        .filter((r) => byId.has(r.id))
        .map((r, i) => ({ r, i }))
        .sort(
          (x, y) =>
            orderValue(x.r.meta?.[ORDER_PROPERTY_OUT]) - orderValue(y.r.meta?.[ORDER_PROPERTY_OUT]) ||
            x.i - y.i,
        )
        .map(({ r }) => toColumn(byId.get(r.id) as Entity))
    }

    const orderBy = columnsFrom.value?.order_by
    return [...data.targets]
      .sort((x, y) => {
        if (orderBy) {
          const d = orderValue(x.properties[orderBy]) - orderValue(y.properties[orderBy])
          if (d) return d
        }
        return entityDisplayTitle(x).localeCompare(entityDisplayTitle(y))
      })
      .map(toColumn)
  })

  /** The column a card belongs to: its target when that is a column, else Other. */
  function columnOf(card: Entity): string {
    const rel = columnsFrom.value?.relation
    const id = rel ? card.relations?.[rel]?.[0] : undefined
    return id && columns.value.some((c) => c.value === id) ? id : OTHER_COLUMN
  }

  return { columns, columnOf, targetType, loading: computed(() => query.isPending.value) }
}
