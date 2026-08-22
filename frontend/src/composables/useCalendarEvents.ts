import { computed, type ComputedRef, type Component } from 'vue'
import type { CalendarConfig, CalendarSourceConfig, KanbanCardField } from '@/types/config'
import type { Entity } from '@/types'
import type { EntityType } from '@/types/schema'
import { formatCellValue } from '@/utils/format'
import { densePropertyRoutingHint } from '@/widgets/viewRouting'
import { defaultRegistry } from '@/widgets/registry'
import {
  compareDays,
  dayKey,
  eventDay,
  eventMinutes,
  type CalendarDay,
  type DateKind,
} from '@/utils/calendarGrid'

/** One resolved chip field: a widget when the property has one, else text. */
export interface CalendarEventField {
  key: string
  component?: Component
  propertyName?: string
  modelValue?: unknown
  text: string
}

/**
 * An entity placed on the grid.
 *
 * Carries the entity itself, not just presentation strings: a chip needs
 * identity to open the right form, the ACL affordance to decide whether it may
 * be dragged, and the source's date binding to know what a drag should write.
 * This is the difference between a calendar view and a calendar feed, whose
 * event model is deliberately lossy.
 */
export interface CalendarEvent {
  id: string
  entity: Entity
  entityType: string
  summary: string
  /** Inclusive first day the event occupies. */
  startDay: CalendarDay
  /** Inclusive last day; equals startDay for single-day events. */
  endDay: CalendarDay
  timed: boolean
  /** `HH:MM` in the display timezone; empty for all-day events. */
  timeLabel: string
  /** Minutes past midnight, for ordering and the week hour axis. */
  minutes: number | null
  color: string
  fields: CalendarEventField[]
  /** Which properties a drag must rewrite, and how to interpret them. */
  dateProperty: string
  endDateProperty?: string
  dateKind: DateKind
}

/** A source paired with the entities fetched for it. */
export interface CalendarSourceData {
  source: CalendarSourceConfig
  entities: Entity[]
  schema: EntityType | undefined
}

function propString(entity: Entity, name: string | undefined): string {
  if (!name) return ''
  const v = entity.properties?.[name]
  return v == null ? '' : String(v)
}

/**
 * Turn fetched entities into positioned events.
 *
 * Widget resolution for chip fields happens ONCE per (source, property) here
 * rather than per event: resolve() walks a registry and can warn, so doing it
 * per event would repeat that work for every chip on the grid.
 */
export function useCalendarEvents(
  config: ComputedRef<CalendarConfig | undefined>,
  sourceData: ComputedRef<CalendarSourceData[]>,
  timezone: ComputedRef<string>
): ComputedRef<CalendarEvent[]> {
  const fieldWidgets = computed(() => {
    const byType = new Map<string, Map<string, { component: Component; propertyName: string; preformatted: boolean }>>()
    for (const { source, schema } of sourceData.value) {
      if (!schema || byType.has(source.entity)) continue
      const perProperty = new Map<string, { component: Component; propertyName: string; preformatted: boolean }>()
      for (const field of config.value?.event?.fields ?? []) {
        if (!field.property || field.relation) continue
        // A field naming a property this source's type lacks is skipped, not an
        // error: sources are heterogeneous by design, so a chip field only has
        // to make sense for the sources that have it.
        if (!schema.properties?.[field.property]) continue
        const hint = densePropertyRoutingHint(schema.properties[field.property], field.property)
        perProperty.set(field.property, {
          component: defaultRegistry.resolveFromHint(hint),
          propertyName: hint.propertyName,
          preformatted: hint.preformatted,
        })
      }
      byType.set(source.entity, perProperty)
    }
    return byType
  })

  return computed(() => {
    const tz = timezone.value
    const cfg = config.value
    if (!cfg) return []

    const events: CalendarEvent[] = []

    sourceData.value.forEach(({ source, entities, schema }) => {
      const kind: DateKind =
        schema?.properties?.[source.date]?.type === 'datetime' ? 'datetime' : 'date'
      const widgets = fieldWidgets.value.get(source.entity)

      for (const entity of entities) {
        const rawStart = propString(entity, source.date)
        const startDay = eventDay(rawStart, kind, tz)
        // An entity with no usable date is simply not on the calendar. Not an
        // error — a task without a due date is a normal task.
        if (!startDay) continue

        const rawEnd = propString(entity, source.end_date)
        // An end before the start is treated as a single-day event rather than
        // a negative span, which would render as nothing at all.
        const parsedEnd = rawEnd ? eventDay(rawEnd, kind, tz) : null
        const endDay = parsedEnd && compareDays(parsedEnd, startDay) > 0 ? parsedEnd : startDay

        const minutes = eventMinutes(rawStart, kind, tz)
        events.push({
          id: `${source.entity}:${entity.id}`,
          entity,
          entityType: source.entity,
          summary: resolveSummary(entity, source),
          startDay,
          endDay,
          timed: kind === 'datetime',
          timeLabel: minutes == null ? '' : formatMinutes(minutes),
          minutes,
          color: source.color ?? 'blue',
          fields: resolveFields(entity, cfg.event?.fields, widgets, schema, tz),
          dateProperty: source.date,
          endDateProperty: source.end_date,
          dateKind: kind,
        })
      }
    })

    // Deterministic order. Without the tiebreakers, two all-day events on the
    // same day have equal keys and reshuffle between renders as fetches settle.
    return events.sort((a, b) => {
      const byDay = compareDays(a.startDay, b.startDay)
      if (byDay !== 0) return byDay
      // All-day events sort before timed ones on the same day.
      if ((a.minutes ?? -1) !== (b.minutes ?? -1)) return (a.minutes ?? -1) - (b.minutes ?? -1)
      return a.id.localeCompare(b.id)
    })
  })
}

function resolveSummary(entity: Entity, source: CalendarSourceConfig): string {
  if (source.summary) {
    const v = propString(entity, source.summary)
    if (v) return v
  }
  // No summary configured and none resolved: fall back to a conventional
  // title property before the id, matching how other dense surfaces label a
  // row. Validation already rejects a source whose type has neither.
  const title = propString(entity, 'title')
  if (title) return title
  // Never an empty chip: an untitled event still has to be clickable.
  return entity.id
}

function resolveFields(
  entity: Entity,
  fields: KanbanCardField[] | undefined,
  widgets: Map<string, { component: Component; propertyName: string; preformatted: boolean }> | undefined,
  schema: EntityType | undefined,
  tz: string
): CalendarEventField[] {
  if (!fields?.length) return []
  const out: CalendarEventField[] = []

  for (const field of fields) {
    if (!field.property || field.relation) continue
    const text = formatCellValue(entity.properties?.[field.property], field.property, schema, tz)
    // Dense-surface rule: an empty value renders as nothing, not a placeholder.
    if (!text) continue

    const widget = widgets?.get(field.property)
    out.push({
      key: field.property,
      component: widget?.component,
      propertyName: widget?.propertyName,
      modelValue: widget?.preformatted ? text : entity.properties?.[field.property],
      text,
    })
  }
  return out
}

function formatMinutes(minutes: number): string {
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`
}

/** Events grouped by the day key of every day they occupy, so a multi-day
 * event appears in each of its cells. */
export function eventsByDay(events: CalendarEvent[]): Map<string, CalendarEvent[]> {
  const map = new Map<string, CalendarEvent[]>()
  for (const ev of events) {
    let cursor = ev.startDay
    // Bounded by the span itself; a malformed end was already clamped to the
    // start, so this cannot run away.
    while (compareDays(cursor, ev.endDay) <= 0) {
      const key = dayKey(cursor)
      const list = map.get(key)
      if (list) list.push(ev)
      else map.set(key, [ev])
      cursor = { ...cursor, day: cursor.day + 1 }
      // Re-normalize across a month boundary.
      const js = new Date(cursor.year, cursor.month - 1, cursor.day)
      cursor = { year: js.getFullYear(), month: js.getMonth() + 1, day: js.getDate() }
    }
  }
  return map
}
