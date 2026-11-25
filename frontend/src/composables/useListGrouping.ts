/**
 * Sections for a list configured with `group_by:`.
 *
 * The server sorts a grouped list by the group property and sends the rows in
 * one piece (up to `max_rows`); this splits them. It is split on the client
 * because the sections are a view of rows already fetched: asking the server
 * for them would need a second grouping model on the wire for no rows it does
 * not already send.
 *
 * Three kinds of section, by the property:
 * - An enum gets one section per declared value, in declared order, including
 *   values no row has, unless the list's filters rule the value out. An empty
 *   section is still a place to add a row to, the way an empty board column
 *   is. `groups:` only renames and colours them.
 * - A date with `buckets: relative` gets the buckets from utils/dateBuckets,
 *   empty ones left out: an empty "Tomorrow" says nothing a reader needs.
 * - Anything else gets one section per distinct value, in the order the rows
 *   arrive.
 * A row with no value goes in a trailing "(none)" section, shown only when a
 * row needs it.
 *
 * A computed grouping (`groups:` with conditions and no property) is decided
 * by the server, which stamps each row's `_group`. Its sections come in
 * declared order, empty ones left out; a row in no group goes in a trailing
 * "Other" section.
 */
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useRelationColumns, OTHER_COLUMN } from '@/composables/useRelationColumns'
import type { DotColor } from 'rela-components/types'
import { namedColor } from '@/utils/styleColors'
import { useUIStore } from '@/stores'
import { formatCellValue } from '@/utils/format'
import { BUCKET_ORDER, bucketDate, bucketOf, defaultBucketLabel } from '@/utils/dateBuckets'
import {
  calendarBucketDate,
  calendarBucketOf,
  calendarBucketColor,
  calendarBucketTitle,
  compareCalendarBuckets,
  type CalendarBucket,
} from '@/utils/calendarBuckets'
import { t } from '@/i18n'
import type {
  CalendarBuckets,
  DateBucket,
  Entity,
  EntityType,
  ListGroupBy,
  ListResponse,
  PageScope,
} from '@/types'

export interface ListSection {
  id: string
  title: string
  color?: DotColor
  /** True when the reader closed this section; see `useListGrouping`. */
  collapsed?: boolean
  /**
   * Property values a row added from this section's Add control starts
   * with, so the new row lands in the section it was added to. Absent for a
   * section no single value describes: "(none)", or a bucket spanning days.
   */
  prefill?: Record<string, unknown>
  /** Edges a row added from this section starts with (relation grouping). */
  prefillRelations?: Record<string, { id: string; type: string }[]>
  /**
   * The bucket this section holds, for a bucketed list. Lets a caller format
   * a row for its bucket without deciding the bucket a second time.
   */
  bucket?: DateBucket
  items: Entity[]
}

export interface GroupOptions {
  now: Date
  /** Display time zone for date buckets and datetime titles. */
  tz: string
  /** Keep enum sections no row falls in. Off for the flyout's quick look. */
  includeEmpty?: boolean
  /**
   * The `filter[...]` params the rows were read with. An empty section for a
   * value they rule out is dropped: a "Done" section under `status != done`
   * can never hold a row, and offering to add one there contradicts the list.
   */
  filterParams?: Record<string, unknown>
}

function paramValues(value: unknown): string[] {
  if (Array.isArray(value)) return value.map(String)
  if (value === undefined || value === null || value === '') return []
  return String(value).split(',')
}

/**
 * Whether the filters admit `value` for `property`. Reads only the equality
 * forms (`eq`, `ne`, `in`); any other operator is taken to admit everything,
 * so an unread filter can only leave a section in, never take one out.
 */
export function filtersAdmit(
  params: Record<string, unknown> | undefined,
  property: string,
  value: string
): boolean {
  if (!params) return true
  const base = `filter[${property}]`
  const eq = [...paramValues(params[base]), ...paramValues(params[`${base}[eq]`])]
  if (eq.length > 0 && !eq.includes(value)) return false
  const within = paramValues(params[`${base}[in]`] ?? params[`${base}[in][]`])
  if (within.length > 0 && !within.includes(value)) return false
  return !paramValues(params[`${base}[ne]`]).includes(value)
}

const NONE_ID = 'none'

function isEmptyValue(value: unknown): boolean {
  return value === undefined || value === null || value === ''
}

/** Mirrors ListGroupBy.Computed on the server. */
function isComputedGroupBy(groupBy: ListGroupBy): boolean {
  return !groupBy.property && !groupBy.relation
}

/** Splits rows into sections. See the file comment for the rules. */
export function groupRows(
  rows: Entity[],
  groupBy: ListGroupBy,
  entityType: EntityType | undefined,
  options: GroupOptions
): ListSection[] {
  if (isComputedGroupBy(groupBy)) return computedSections(rows, groupBy)
  const property = groupBy.property ?? ''
  const def = entityType?.properties[property]
  if (groupBy.buckets && typeof groupBy.buckets === 'object') {
    return calendarSections(rows, groupBy, groupBy.buckets, options, def?.type === 'date')
  }
  if (groupBy.buckets) return bucketSections(rows, groupBy, options, def?.type === 'date')

  const declared = def?.values ?? []
  const overrides = new Map((groupBy.groups ?? []).map((g) => [g.value, g]))
  const byKey = new Map<string, ListSection>()

  for (const value of declared) {
    const override = overrides.get(value)
    byKey.set(value, {
      id: `value:${value}`,
      title: override?.label || def?.labels?.[value] || value,
      color: namedColor(override?.color),
      prefill: { [property]: value },
      items: [],
    })
  }

  const none: ListSection = { id: NONE_ID, title: t('list.group.none'), items: [] }
  for (const row of rows) {
    const value = row.properties[property]
    if (isEmptyValue(value)) {
      none.items.push(row)
      continue
    }
    const key = String(value)
    let section = byKey.get(key)
    if (!section) {
      section = {
        id: `value:${key}`,
        title: formatCellValue(value, property, entityType, options.tz) || key,
        prefill: { [property]: value },
        items: [],
      }
      byKey.set(key, section)
    }
    section.items.push(row)
  }

  const includeEmpty = options.includeEmpty ?? true
  const sections = [...byKey.values()].filter(
    (s) =>
      s.items.length > 0 ||
      (includeEmpty && filtersAdmit(options.filterParams, property, s.id.slice('value:'.length)))
  )
  if (none.items.length > 0) sections.push(none)
  return sections
}

const OTHER_ID = 'other'

/** Sections for a computed grouping; see the file comment. */
function computedSections(rows: Entity[], groupBy: ListGroupBy): ListSection[] {
  const byId = new Map<string, ListSection>()
  for (const g of groupBy.groups ?? []) {
    if (!g.id) continue
    byId.set(g.id, {
      id: `group:${g.id}`,
      title: g.label || g.id,
      color: namedColor(g.color),
      items: [],
    })
  }
  const other: ListSection = { id: OTHER_ID, title: t('list.group.other'), items: [] }
  for (const row of rows) {
    ;((row._group && byId.get(row._group)) || other).items.push(row)
  }
  const sections = [...byId.values()].filter((s) => s.items.length > 0)
  if (other.items.length > 0) sections.push(other)
  return sections
}

/**
 * `isDate` is true for a `date` property. It decides how a value is read
 * (bucketOf), and only a date's sections prefill: a datetime needs a time as
 * well as a day, and no bucket names one.
 */
function bucketSections(
  rows: Entity[],
  groupBy: ListGroupBy,
  options: GroupOptions,
  isDate: boolean
): ListSection[] {
  const { now, tz } = options
  const byBucket = new Map<DateBucket, Entity[]>()
  for (const row of rows) {
    const bucket = bucketOf(row.properties[groupBy.property ?? ''], now, tz, isDate)
    const items = byBucket.get(bucket) ?? []
    items.push(row)
    byBucket.set(bucket, items)
  }
  return BUCKET_ORDER.flatMap((bucket): ListSection[] => {
    const items = byBucket.get(bucket)
    if (!items) return []
    const date = isDate ? bucketDate(bucket, now, tz) : undefined
    return [
      {
        id: `bucket:${bucket}`,
        title: groupBy.labels?.[bucket] || defaultBucketLabel(bucket),
        color: bucket === 'overdue' ? 'red' : undefined,
        prefill: date && groupBy.property ? { [groupBy.property]: date } : undefined,
        bucket,
        items,
      },
    ]
  })
}

/**
 * Calendar sections (utils/calendarBuckets), empty ones left out, ordered by
 * their start day rather than by row arrival: the server orders by instant,
 * and a naive datetime is read as wall-clock time, so arrival order could
 * split a day.
 */
function calendarSections(
  rows: Entity[],
  groupBy: ListGroupBy,
  buckets: CalendarBuckets,
  options: GroupOptions,
  isDate: boolean
): ListSection[] {
  const { now, tz } = options
  const byId = new Map<string, { bucket: CalendarBucket; items: Entity[] }>()
  for (const row of rows) {
    const bucket = calendarBucketOf(
      row.properties[groupBy.property ?? ''],
      buckets,
      now,
      tz,
      isDate
    )
    const entry = byId.get(bucket.id) ?? { bucket, items: [] }
    entry.items.push(row)
    byId.set(bucket.id, entry)
  }
  const desc = buckets.order === 'desc'
  return [...byId.values()]
    .sort((a, b) => compareCalendarBuckets(a.bucket, b.bucket, desc))
    .map(({ bucket, items }) => {
      const date = isDate ? calendarBucketDate(bucket) : undefined
      return {
        id: bucket.id,
        title:
          (bucket.fixed && groupBy.labels?.[bucket.fixed]) || calendarBucketTitle(bucket, now, tz),
        color: calendarBucketColor(bucket),
        prefill: date && groupBy.property ? { [groupBy.property]: date } : undefined,
        bucket: bucket.meta,
        items,
      }
    })
}

/** Where a list's closed sections are remembered, per browser. */
export function collapsedStorageKey(listId: string): string {
  return `rela:list-groups:${listId}`
}

function readCollapsed(listId: string): Set<string> {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(collapsedStorageKey(listId)) ?? '[]')
    return new Set(
      Array.isArray(parsed) ? parsed.filter((id): id is string => typeof id === 'string') : []
    )
  } catch {
    // why: a corrupt entry costs the reader their closed sections, not the list.
    return new Set()
  }
}

/** How often bucket boundaries are re-checked, so "today" moves at midnight. */
const BUCKET_TICK_MS = 60_000

/**
 * A grouped list's sections, with the reader's closed sections remembered in
 * localStorage. Returns `grouped: false` and no sections for a flat list.
 *
 * `truncated` is true when the server had more rows than `max_rows` let the
 * list load: the sections then hold a prefix of the list, and the counts in
 * their headings are counts of that prefix.
 */
export function useListGrouping(options: {
  listId: () => string
  groupBy: () => ListGroupBy | undefined
  entityType: () => EntityType | undefined
  response: () => ListResponse<Entity> | undefined
  /** The `filter[...]` params the rows were read with; see GroupOptions. */
  filterParams?: () => Record<string, unknown>
  includeEmpty?: boolean
  /** The page tab the list sits in; picks the sections for `offered_by`. */
  pageScope?: () => PageScope | undefined
  worldParam?: () => string | undefined
}) {
  const uiStore = useUIStore()
  const now = ref(new Date())
  const timer = setInterval(() => {
    if (options.groupBy()?.buckets) now.value = new Date()
  }, BUCKET_TICK_MS)
  onScopeDispose(() => clearInterval(timer))

  const collapsed = ref(readCollapsed(options.listId()))
  watch(options.listId, (id) => {
    collapsed.value = readCollapsed(id)
  })

  const grouped = computed(() => options.groupBy() !== undefined)

  const relationColumns = useRelationColumns(
    computed(() => (options.groupBy()?.relation ? options.groupBy() : undefined)),
    computed(() => options.pageScope?.()),
    computed(() => options.worldParam?.())
  )

  function relationSections(groupBy: ListGroupBy, rows: Entity[]): ListSection[] {
    const relation = groupBy.relation as string
    const targetType = relationColumns.targetType.value ?? ''
    const byId = new Map<string, ListSection>(
      relationColumns.columns.value.map((c) => [
        c.value,
        {
          id: `rel:${c.value}`,
          title: c.label,
          color: c.color,
          prefillRelations: { [relation]: [{ id: c.value, type: targetType }] },
          items: [],
        },
      ])
    )
    const other: ListSection = {
      id: `rel:${OTHER_COLUMN}`,
      title: t('list.group.other'),
      items: [],
    }
    for (const row of rows) {
      const column = relationColumns.columnOf(row)
      ;(byId.get(column) ?? other).items.push(row)
    }
    const sections = [...byId.values()].filter(
      (s) => s.items.length > 0 || (options.includeEmpty ?? true)
    )
    if (other.items.length > 0) sections.push(other)
    return sections
  }

  const sections = computed<ListSection[]>(() => {
    const groupBy = options.groupBy()
    if (!groupBy) return []
    const rows = options.response()?.data ?? []
    if (groupBy.relation) {
      return relationSections(groupBy, rows).map((section) => ({
        ...section,
        collapsed: collapsed.value.has(section.id),
      }))
    }
    return groupRows(rows, groupBy, options.entityType(), {
      now: now.value,
      tz: uiStore.effectiveTimezone,
      includeEmpty: options.includeEmpty,
      filterParams: options.filterParams?.(),
    }).map((section) => ({ ...section, collapsed: collapsed.value.has(section.id) }))
  })

  const truncated = computed(() => grouped.value && options.response()?.meta.has_more === true)
  const total = computed(() => options.response()?.meta.total ?? 0)

  function setCollapsed(sectionId: string, isCollapsed: boolean) {
    const next = new Set(collapsed.value)
    if (isCollapsed) next.add(sectionId)
    else next.delete(sectionId)
    collapsed.value = next
    try {
      localStorage.setItem(collapsedStorageKey(options.listId()), JSON.stringify([...next]))
    } catch {
      // why: storage full or disabled: the choice holds for this visit only.
    }
  }

  return { grouped, sections, truncated, total, setCollapsed }
}
