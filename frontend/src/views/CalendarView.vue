<script setup lang="ts">
/**
 * Month/week calendar over date-bearing entities.
 *
 * Two things differ from KanbanView on purpose.
 *
 * The fetch is WINDOWED. A board partitions every entity of one type, so it
 * pages to exhaustion; a calendar shows one period, and a dated corpus is
 * exactly the kind that overruns the page cap. Each source is fetched with a
 * half-open date range, so payload stays proportional to what is on screen.
 * Windowing is not a server-load win — the API filters in memory after loading
 * the type — it is what keeps the result under the truncation cap.
 *
 * Day placement is computed in the DISPLAY timezone, never the browser's, and
 * never by adding milliseconds. All of that arithmetic lives in
 * utils/calendarGrid.ts as pure functions so the DST and zone-boundary cases
 * are table-tested rather than asserted through a mounted grid.
 */
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useMutation, useQueryCache } from '@pinia/colada'
import { listAllEntities, updateEntity, getErrorMessage } from '@/api'
import { entityKeys } from '@/queries/entities'
import { beginOptimistic, rollbackOptimistic, settleOptimistic } from '@/queries/optimisticList'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import { actionAllowed } from '@/utils/affordancesWarning'
import { renderMarkdown } from '@/utils/markdown'
import { viewHeaderMarkdown, viewFooterMarkdown } from '@/types/config'
import type { CalendarConfig } from '@/types/config'
import type { Entity, ListParams } from '@/types'
import CalendarGrid from '@/components/calendar/CalendarGrid.vue'
import {
  useCalendarEvents,
  eventsByDay,
  type CalendarEvent,
  type CalendarSourceData,
} from '@/composables/useCalendarEvents'
import {
  addDays,
  applyDayDelta,
  dayFromKey,
  dayKey,
  daysBetween,
  sameDay,
  shiftAnchor,
  todayIn,
  visibleDays,
  windowBounds,
  type CalendarDay,
  type CalendarView as ViewKind,
} from '@/utils/calendarGrid'

const props = defineProps<{ id: string }>()

const router = useRouter()
const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const queryCache = useQueryCache()

const config = computed(() => schemaStore.getCalendar(props.id) as CalendarConfig | undefined)
const timezone = computed(() => uiStore.effectiveTimezone)

// View and anchor are URL state: without it a refresh loses your place and
// "the week of the launch" is unlinkable.
const view = computed<ViewKind>({
  get: () => {
    const q = router.currentRoute.value.query.view
    return q === 'week' || q === 'month' ? q : (config.value?.default_view ?? 'month')
  },
  set: (v) => {
    void router.replace({ query: { ...router.currentRoute.value.query, view: v } })
  },
})

const anchor = computed<CalendarDay>({
  get: () => {
    const q = router.currentRoute.value.query.date
    const parsed = typeof q === 'string' ? dayFromKey(q) : null
    return parsed ?? todayIn(timezone.value)
  },
  set: (d) => {
    void router.replace({ query: { ...router.currentRoute.value.query, date: dayKey(d) } })
  },
})

const weekStart = computed(() => config.value?.week_start ?? 'monday')
const days = computed(() => visibleDays(view.value, anchor.value, weekStart.value))
const today = computed(() => todayIn(timezone.value))

/**
 * Per-source query params. The window is a half-open range, so a timed event
 * late on the last visible day is included — a `lte` bound of that day would
 * mean "<= midnight" and silently drop it.
 */
function paramsFor(dateProperty: string, pastPadDays: number): ListParams {
  const b = windowBounds(view.value, anchor.value, weekStart.value, timezone.value, pastPadDays)
  const params: ListParams = {}
  params[`filter[${dateProperty}][gte]`] = b.gte
  params[`filter[${dateProperty}][lt]`] = b.lt
  return params
}

/**
 * The grid's data, fetched per source through the shared query cache.
 *
 * The KEY SHAPE is load-bearing in two directions, and an earlier version of
 * this component broke both by inventing a `['entities','calendar',…]` key:
 *
 *   - `useEvents` invalidates `entityKeys.type(<type>)` on SSE entity events,
 *     which only prefix-matches a key whose second element is the entity type.
 *     A calendar-namespaced key silently opted out of live updates.
 *   - `beginOptimistic` writes through `entityKeys.list(<type>)`. A grid reading
 *     a different entry would never show the optimistic move, so a dragged
 *     event would visibly snap back until the refetch landed.
 *
 * `entityKeys.listParams` gives both: it descends from `list(type)` — so the
 * optimistic write and the SSE invalidation both reach it — while the params
 * segment keeps each window in its own cache entry.
 */
const sourceQueries = computed(() =>
  (config.value?.sources ?? []).map((source) => ({
    source,
    params: paramsFor(source.date, source.end_date ? (source.max_span ?? 31) : 0),
  }))
)

interface SourceResult {
  key: string
  entities: Entity[]
  truncated: boolean
}

const fetched = ref<SourceResult[]>([])
const loading = ref(true)
const loadError = ref('')

/**
 * Sequence number of the newest refetch.
 *
 * Navigating months quickly starts overlapping fetches, and without this the
 * LAST ONE TO RESOLVE wins — which may be the older window. Clicking "next"
 * twice would then settle on the wrong month's events, with no error and a
 * period label that disagrees with the grid. The in-flight request is also
 * aborted, so a superseded window stops paging rather than finishing work
 * whose result is discarded.
 */
let refetchGeneration = 0
let inFlight: AbortController | null = null

async function refetchGrid() {
  const queries = sourceQueries.value
  const generation = ++refetchGeneration

  inFlight?.abort()
  const controller = new AbortController()
  inFlight = controller

  if (!queries.length) {
    fetched.value = []
    loading.value = false
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    const results = await Promise.all(
      queries.map(async ({ source, params }): Promise<SourceResult> => {
        const res = await listAllEntities(source.entity, params, controller.signal)
        // Publish into the cache entry the optimistic write and SSE
        // invalidation target, so a drag updates the grid immediately.
        queryCache.setQueryData([...entityKeys.listParams(source.entity, params)], res)
        return {
          key: source.entity + ':' + source.date,
          entities: res.data,
          truncated: res.meta?.has_more === true,
        }
      })
    )
    // A superseded fetch must not publish: its window is no longer on screen.
    if (generation !== refetchGeneration) return
    fetched.value = results
  } catch (err) {
    // An abort is this component superseding itself, not a failure to report.
    if (generation !== refetchGeneration || controller.signal.aborted) return
    loadError.value = getErrorMessage(err, 'Failed to load calendar')
  } finally {
    if (generation === refetchGeneration) loading.value = false
  }
}

// Refetch when the window, the calendar, or the display timezone changes.
watch(
  () => [props.id, view.value, dayKey(anchor.value), timezone.value].join('|'),
  () => {
    void refetchGrid()
  },
  { immediate: true }
)

const sourceData = computed<CalendarSourceData[]>(() =>
  (config.value?.sources ?? []).map((source) => ({
    source,
    entities: fetched.value.find((r) => r.key === source.entity + ':' + source.date)?.entities ?? [],
    schema: schemaStore.getEntityType(source.entity),
  }))
)

const events = useCalendarEvents(config, sourceData, timezone)
const byDay = computed(() => eventsByDay(events.value))

/** True when any source hit the page cap: the grid is incomplete and says so
 * rather than looking merely quiet. */
const truncated = computed(() => fetched.value.some((r) => r.truncated))

/** An event longer than its source's max_span may fall outside the fetched
 * window, so the caveat is surfaced instead of the event silently missing. */
const longEventWarning = computed(() => {
  for (const ev of events.value) {
    const src = config.value?.sources.find((s) => s.entity === ev.entityType)
    if (!src?.end_date) continue
    if (daysBetween(ev.startDay, ev.endDay) > (src.max_span ?? 31)) return true
  }
  return false
})

const maxPerDay = computed(() => config.value?.max_events_per_day ?? 4)
const expandedDays = ref<Set<string>>(new Set())

function eventsForDay(day: CalendarDay): CalendarEvent[] {
  return byDay.value.get(dayKey(day)) ?? []
}

function visibleEventsForDay(day: CalendarDay): CalendarEvent[] {
  const all = eventsForDay(day)
  if (view.value === 'week' || expandedDays.value.has(dayKey(day))) return all
  return all.slice(0, maxPerDay.value)
}

function hiddenCount(day: CalendarDay): number {
  return Math.max(0, eventsForDay(day).length - visibleEventsForDay(day).length)
}

function expandDay(day: CalendarDay) {
  // In place: the obvious alternative, opening a day view, does not exist yet.
  expandedDays.value = new Set(expandedDays.value).add(dayKey(day))
}

const headerHtml = computed(() => renderMarkdown(viewHeaderMarkdown(config.value) ?? ''))
const footerHtml = computed(() => renderMarkdown(viewFooterMarkdown(config.value) ?? ''))

const periodLabel = computed(() => {
  const a = anchor.value
  if (view.value === 'month') {
    return new Date(a.year, a.month - 1, 1).toLocaleDateString(undefined, {
      month: 'long',
      year: 'numeric',
    })
  }
  const week = days.value
  const fmt = (d: CalendarDay) =>
    new Date(d.year, d.month - 1, d.day).toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
    })
  return `${fmt(week[0])} – ${fmt(week[week.length - 1])}`
})

const weekdayLabels = computed(() =>
  days.value.slice(0, 7).map((d) =>
    new Date(d.year, d.month - 1, d.day).toLocaleDateString(undefined, { weekday: 'short' })
  )
)

function go(delta: number) {
  anchor.value = shiftAnchor(view.value, anchor.value, delta)
}

function goToday() {
  anchor.value = todayIn(timezone.value)
}

function canUpdate(entity: Entity): boolean {
  return actionAllowed(entity, 'update')
}

function openEvent(ev: CalendarEvent) {
  const form = config.value?.edit_form
  void router.push(form ? `/form/${form}/${ev.entity.id}` : `/entity/${ev.entityType}/${ev.entity.id}`)
}

function createNew() {
  const form = config.value?.create_form
  if (form) void router.push(`/form/${form}`)
}

function canCreate(): boolean {
  const first = sourceData.value[0]?.entities?.[0]
  return first ? actionAllowed(first, 'create') : true
}

// --- Drag to reschedule ---

const dragged = ref<CalendarEvent | null>(null)

const { mutate: reschedule } = useMutation({
  mutation: ({ event, updates }: { event: CalendarEvent; updates: Record<string, string> }) =>
    updateEntity(event.entityType, event.entity.id, { properties: updates }),
  onMutate({ event, updates }) {
    return beginOptimistic(
      queryCache,
      entityKeys.list(event.entityType),
      event.entity.id,
      (e) => ({ ...e, properties: { ...e.properties, ...updates } })
    )
  },
  onError(err, _vars, context) {
    rollbackOptimistic(queryCache, context)
    uiStore.error(getErrorMessage(err, 'Failed to move event'))
  },
  async onSettled(_d, _e, _v, context) {
    await settleOptimistic(queryCache, context)
    await refetchGrid()
  },
})

function onDragStart(payload: { event: CalendarEvent; native: DragEvent }) {
  dragged.value = payload.event
  if (payload.native.dataTransfer) {
    payload.native.dataTransfer.effectAllowed = 'move'
    payload.native.dataTransfer.setData('text/plain', payload.event.entity.id)
  }
}

function onDragOver(e: DragEvent) {
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
}

function onDrop(e: DragEvent, day: CalendarDay) {
  e.preventDefault()
  const ev = dragged.value
  dragged.value = null
  if (!ev) return

  // Defence in depth: :draggable="false" stops a drag starting here, but an
  // external drag source can still fire this handler.
  if (!canUpdate(ev.entity)) return

  const delta = daysBetween(ev.startDay, day)
  if (delta === 0) return

  const nextStart = applyDayDelta(
    String(ev.entity.properties?.[ev.dateProperty] ?? ''),
    ev.dateKind,
    delta,
    timezone.value
  )
  // A value we cannot parse is a value we must not overwrite.
  if (nextStart === null) {
    uiStore.error('Could not move this event: its date could not be read.')
    return
  }

  const updates: Record<string, string> = { [ev.dateProperty]: nextStart }

  // Start and end move by the same whole-day delta, in ONE patch: a two-write
  // sequence would leave the entity briefly ending before it starts.
  if (ev.endDateProperty) {
    const rawEnd = ev.entity.properties?.[ev.endDateProperty]
    if (rawEnd != null && rawEnd !== '') {
      const nextEnd = applyDayDelta(String(rawEnd), ev.dateKind, delta, timezone.value)
      if (nextEnd === null) {
        uiStore.error('Could not move this event: its end date could not be read.')
        return
      }
      updates[ev.endDateProperty] = nextEnd
    }
  }

  reschedule({ event: ev, updates })
}

function onDragEnd() {
  dragged.value = null
}

// Keep the anchor's month meaningful when the view switches to week.
watch(view, (v) => {
  if (v === 'week' && !days.value.some((d) => sameDay(d, anchor.value))) {
    anchor.value = addDays(anchor.value, 0)
  }
})
</script>

<template>
  <div class="calendar-view">
    <header class="page-header">
      <div class="header-left">
        <h1>{{ config?.title || props.id }}</h1>
      </div>
      <div class="header-actions">
        <button v-if="config?.create_form && canCreate()" class="btn btn-primary" @click="createNew">
          + New
        </button>
      </div>
    </header>

    <div class="calendar-toolbar">
      <div class="calendar-nav">
        <button class="btn" aria-label="Previous period" @click="go(-1)">‹</button>
        <button class="btn" @click="goToday">Today</button>
        <button class="btn" aria-label="Next period" @click="go(1)">›</button>
        <span class="calendar-period">{{ periodLabel }}</span>
      </div>
      <div class="calendar-views">
        <button
          class="btn"
          :class="{ active: view === 'month' }"
          :aria-pressed="view === 'month'"
          @click="view = 'month'"
        >
          Month
        </button>
        <button
          class="btn"
          :class="{ active: view === 'week' }"
          :aria-pressed="view === 'week'"
          @click="view = 'week'"
        >
          Week
        </button>
      </div>
    </div>

    <!-- eslint-disable-next-line vue/no-v-html -- sanitized by renderMarkdown -->
    <div v-if="headerHtml" class="view-info view-info--top" v-html="headerHtml" />

    <div v-if="truncated" class="truncation-banner" role="alert">
      Some events are not shown — this period returned more entities than the calendar can load.
    </div>
    <div v-if="longEventWarning" class="truncation-banner" role="alert">
      Some long events may not be shown; increase <code>max_span</code> for this calendar's sources.
    </div>

    <div v-if="loadError" class="error-state">{{ loadError }}</div>
    <div v-else-if="loading && !events.length" class="loading-state">Loading…</div>

    <CalendarGrid
      v-else
      :view="view"
      :days="days"
      :weekday-labels="weekdayLabels"
      :anchor="anchor"
      :today="today"
      :events-for-day="visibleEventsForDay"
      :hidden-count="hiddenCount"
      :can-update="canUpdate"
      @expand="expandDay"
      @open="openEvent"
      @dragstart="onDragStart"
      @dragend="onDragEnd"
      @dragover="onDragOver"
      @drop="onDrop"
    />

    <p v-if="!loading && !loadError && !events.length" class="calendar-empty">
      No events in this period.
    </p>

    <!-- eslint-disable-next-line vue/no-v-html -- sanitized by renderMarkdown -->
    <div v-if="footerHtml" class="view-info view-info--bottom" v-html="footerHtml" />
  </div>
</template>

<style scoped>
.calendar-view {
  max-width: 100%;
}

.calendar-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  margin-bottom: var(--space-md);
  flex-wrap: wrap;
}

.calendar-nav,
.calendar-views {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
}

.calendar-period {
  margin-left: var(--space-sm);
  font-size: var(--font-size-lg);
  font-weight: 600;
}

.calendar-views .btn.active {
  background: var(--color-primary);
  color: var(--color-on-primary, #fff);
}

.calendar-empty {
  margin-top: var(--space-md);
  color: var(--color-text-muted);
}
</style>
