import { toApiOperator } from '@/utils/filters'
import type { ListConfig, ListGroupBy, ListParams, PageScope, SortSpec } from '@/types'

/** A sort as the list endpoint takes it: `a,-b` for a ascending, b descending. */
export function sortParam(specs: SortSpec[]): string {
  return specs.map((s) => (s.direction === 'desc' ? `-${s.property}` : s.property)).join(',')
}

/**
 * The sort a grouped list is read in: the group property first, ascending,
 * then the given sort within each group.
 *
 * Ascending on the group property is what makes each section's rows arrive
 * together. The server ranks an enum by its declared order and puts unset
 * values last, which is the section order; for dates it is the bucket order.
 * A later key on the same property is dropped, since it could only reorder
 * rows the first key has already tied.
 */
export function groupedSort(groupBy: ListGroupBy | undefined, specs: SortSpec[]): SortSpec[] {
  if (!groupBy) return specs
  return [
    { property: groupBy.property, direction: 'asc' },
    ...specs.filter((s) => s.property !== groupBy.property),
  ]
}

/**
 * The part of a list read that the list's CONFIG decides, before the reader
 * filters, sorts or pages it.
 *
 * Shared by the list page and the sidebar flyout, so the two cannot disagree
 * about which rows a list holds. Static `filters:` are applied client-side of
 * the wire (the server applies only `condition:`, through `list_id`), so a
 * second reader that forgot them would silently show a wider list.
 */
export function listBaseParams(listId: string, config: ListConfig): ListParams {
  const params: ListParams = {
    per_page: config.page_size || 25,
    // Names the configured list so the server can apply its `condition:`.
    // Sent unconditionally rather than only when a condition exists: the SPA
    // would otherwise have to know which lists carry one, duplicating a
    // server-side fact that changes on config reload. An id for a list with
    // no condition simply resolves to no constraint.
    list_id: listId,
  }

  const record = params as Record<string, string | number | undefined>
  for (const filter of config.filters ?? []) {
    if (!filter.operator || !filter.value) continue
    const key = `filter[${filter.property}][${toApiOperator(filter.operator)}]`
    const existing = record[key]
    record[key] = existing ? `${existing},${filter.value}` : (filter.value as string)
  }

  const sort = groupedSort(config.group_by, config.default_sort ?? [])
  if (sort.length) params.sort = sortParam(sort)

  // The list's configured scope. Sent because the endpoint is keyed by entity
  // TYPE, so the server cannot tell which list is on screen — without this the
  // view falls back to the type's `default` and a list declaring
  // `query_scope: archief` renders unscoped.
  if (config.query_scope) params.query_scope = config.query_scope

  return params
}

/**
 * The params that narrow a read to an entity page tab, or none outside one.
 * Spread into a view's list params, and into its export URL, so every read
 * of the tab shows the same rows.
 */
export function pageScopeParams(scope: PageScope | undefined): Pick<ListParams, 'scope_page' | 'scope_tab' | 'anchor'> {
  if (!scope) return {}
  return { scope_page: scope.page, scope_tab: scope.tab, anchor: scope.entity }
}
