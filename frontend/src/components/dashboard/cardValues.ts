/**
 * How breakdown and table cards render values (TKT-69PZ09D): the way the rest
 * of the app does. An enum value is a badge with its authored label and
 * `styles:` colour, a `display: {currency}` integer an amount, a date
 * formatted. Pure derivations over rows the dashboard already holds; the
 * view calls them once per card (TKT-ERHWL0).
 */
import type { DashboardCardEntry, DashboardCardValue, Entity, EntityType } from '@/types'
import { isPropertyRedacted } from '@/utils/affordances'
import { createDenseCellResolver, type DenseCell } from '@/utils/denseCell'
import { formatCellValue } from '@/utils/format'
import { styleBarColor, styleClassFor } from '@/utils/styleColors'

/** The schema lookups the derivations need; the schema store provides them. */
export interface CardSchema {
  getEntityType(name: string): EntityType | undefined
  getEnumLabel(
    value: string,
    property?: string,
    entityType?: string | EntityType
  ): string | undefined
  stylesForProperty(
    property: string,
    entityType?: string | EntityType
  ): Record<string, string> | undefined
  getList(id: string): { entity: string } | undefined
}

/**
 * One breakdown group. `key` is the stored value ('' for rows without one);
 * `label` is what the row shows when it is not a badge.
 */
export interface BreakdownItem {
  key: string
  label: string
  count: number
  percentage: number
  /** The entity type the value belongs to, for its badge label and style. */
  entityType?: string
  /** True for a value of an enum property: it renders as a badge, like a list cell. */
  badge: boolean
  /** The bar colour from the value's `styles:` entry; absent means accent. */
  color?: string
}

/** One table cell, resolved once per render rather than per template read. */
export interface TableCell {
  key: string
  /** The detail route when the column links to the row. */
  link?: string
  /** The reader may not see this property on this row. */
  hidden: boolean
  /** The widget for the value, as a list cell renders it; absent means text. */
  widget?: DenseCell
  /** The formatted value, for a link, a relation or a value without a widget. */
  text: string
}

export interface TableRow {
  entity: Entity
  cells: TableCell[]
}

/** The declared values of `property` on a type, when it is an enum there. */
function enumValues(schema: CardSchema, type: string | undefined, property: string) {
  if (!type) return undefined
  const values = schema.getEntityType(type)?.properties[property]?.values
  return values?.length ? values : undefined
}

function barColor(schema: CardSchema, type: string | undefined, property: string, value: string) {
  if (!value) return undefined
  return styleBarColor(styleClassFor(schema.stylesForProperty(property, type), value))
}

/**
 * Orders breakdown groups the way the server orders a list card's groups
 * (`groupCounts` in dashboard_fold.go), so a query card and a list card over
 * the same rows agree: declared enum values first in declared order, then
 * other values by count (most first) and value, and rows without a value last.
 * Declared order rather than count, because a breakdown over a status or a
 * pipeline stage reads in that order.
 */
function compareGroups(rank: (item: BreakdownItem) => number | undefined) {
  return (a: BreakdownItem, b: BreakdownItem): number => {
    if ((a.key === '') !== (b.key === '')) return a.key === '' ? 1 : -1
    const ra = rank(a)
    const rb = rank(b)
    if (ra !== undefined && rb !== undefined) return ra - rb
    if (ra !== undefined || rb !== undefined) return ra !== undefined ? -1 : 1
    return b.count - a.count || (a.key < b.key ? -1 : a.key > b.key ? 1 : 0)
  }
}

/** A query card's breakdown, grouped here over the rows the search returned. */
export function queryBreakdown(
  schema: CardSchema,
  groupBy: string,
  entities: Entity[],
  unknownLabel: string
): BreakdownItem[] {
  // Per value: its count and the type of the first row that had it. A query
  // may match several types; each value takes its label and style from its
  // own type.
  const groups = new Map<string, { count: number; type: string }>()
  for (const entity of entities) {
    const raw = entity.properties[groupBy]
    const key = raw === undefined || raw === null ? '' : String(raw)
    const group = groups.get(key)
    if (group) group.count++
    else groups.set(key, { count: 1, type: entity.type })
  }

  const total = entities.length
  const ranks = new Map<string, number | undefined>()
  const items = [...groups].map(([key, { count, type }]): BreakdownItem => {
    const values = enumValues(schema, type, groupBy)
    const rank = values?.indexOf(key) ?? -1
    ranks.set(key, rank >= 0 ? rank : undefined)
    return {
      key,
      label: key ? (schema.getEnumLabel(key, groupBy, type) ?? key) : unknownLabel,
      count,
      percentage: total > 0 ? (count / total) * 100 : 0,
      entityType: type,
      badge: !!values && key !== '',
      color: barColor(schema, type, groupBy, key),
    }
  })
  return items.sort(compareGroups((item) => ranks.get(item.key)))
}

/**
 * A list card's breakdown, from the groups the server counted. The server
 * already ordered them and resolved each label; the badge and colour come
 * from the list's entity type.
 */
export function listBreakdown(
  schema: CardSchema,
  card: DashboardCardEntry,
  value: DashboardCardValue,
  unknownLabel: string
): BreakdownItem[] {
  const groupBy = card.group_by ?? ''
  const type = card.list ? schema.getList(card.list)?.entity : undefined
  const isEnum = !!enumValues(schema, type, groupBy)
  return (value.groups ?? []).map((g) => ({
    key: g.value,
    label: g.label || unknownLabel,
    count: g.count,
    percentage: value.total > 0 ? (g.count / value.total) * 100 : 0,
    entityType: type,
    badge: isEnum && g.value !== '',
    color: barColor(schema, type, groupBy, g.value),
  }))
}

function cellLink(entity: Entity, col: { link?: string }): string | undefined {
  return col.link === 'detail' ? `/entity/${entity.type}/${entity.id}` : undefined
}

/**
 * A table card's cells, resolved as list cells are. Each row takes its
 * widgets from its own type, since a query may match several. `tz` is the
 * reader's display time zone.
 */
export function tableRows(
  schema: CardSchema,
  card: DashboardCardEntry,
  entities: Entity[],
  tz: string
): TableRow[] {
  const resolve = createDenseCellResolver()
  const columns = card.columns ?? []
  return entities.map((entity) => {
    const type = schema.getEntityType(entity.type)
    const cells = columns.map((col, i): TableCell => {
      const key = `${i}:${col.property ?? col.relation ?? ''}`
      const link = cellLink(entity, col)
      const property = col.property
      // A relation column has no value on the row; it stays blank, as before.
      if (!property) return { key, link, hidden: false, text: '' }
      const value = entity.properties[property]
      return {
        key,
        link,
        hidden: !link && isPropertyRedacted(property, entity._redacted),
        widget: link ? undefined : resolve(value, property, type, tz),
        text: type ? formatCellValue(value, property, type, tz) : String(value ?? ''),
      }
    })
    return { entity, cells }
  })
}
