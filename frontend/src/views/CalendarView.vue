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
import { useWorld } from '@/composables/useWorld'
import { renderMarkdown } from '@/utils/markdown'
import { buildFilterKey, parseWhereClause } from '@/utils/filters'
import { viewHeaderMarkdown, viewFooterMarkdown } from '@/types/config'
import type { CalendarConfig, CalendarSourceConfig } from '@/types/config'
import type { Entity, ListParams } from '@/types'
import CalendarGrid from '@/components/calendar/CalendarGrid.vue'
import EntityPreviewModal from '@/components/calendar/EntityPreviewModal.vue'
import CalendarLegend from '@/components/calendar/CalendarLegend.vue'
import {
  useCalendarEvents,
  eventsByDay,
  type CalendarEvent,
  type CalendarSourceData,
} from '@/composables/useCalendarEvents'
import {
  applyDayDelta,
  dayFromKey,
  dayKey,
  daysBetween,
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
const { isWorldBound, worldParam } = useWorld()
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

/**
 * Sources the reader has switched off, by index, carried in the URL.
 *
 * In the URL rather than component state so a filtered calendar is shareable
 * and survives a refresh — the same reasoning as `view` and `date`. Indices
 * rather than names because index is a source's identity here (two sources may
 * share an entity type, differing only by `where:`).
 *
 * An index that no longer exists — the config changed under an old link — is
 * ignored rather than erroring: the worst case is showing a source the sender
 * had hidden, which is visibly harmless.
 */
const hiddenSources = computed<number[]>({
  get: () => {
    const raw = router.currentRoute.value.query.hide
    if (typeof raw !== 'string' || !raw) return []
    return raw
      .split(',')
      .map((n) => Number.parseInt(n, 10))
      .filter((n) => Number.isInteger(n) && n >= 0)
  },
  set: (indices) => {
    const query = { ...router.currentRoute.value.query }
    if (indices.length) query.hide = [...indices].sort((a, b) => a - b).join(',')
    else delete query.hide
    void router.replace({ query })
  },
})

function toggleSource(index: number) {
  const next = new Set(hiddenSources.value)
  if (next.has(index)) next.delete(index)
  else next.add(index)
  hiddenSources.value = [...next]
}

const weekStart = computed(() => config.value?.week_start ?? 'monday')
const days = computed(() => visibleDays(view.value, anchor.value, weekStart.value))
const today = computed(() => todayIn(timezone.value))

/**
 * Per-source query params. The window is a half-open range, so a timed event
 * late on the last visible day is included — a `lte` bound of that day would
 * mean "<= midnight" and silently drop it.
 */
function paramsFor(source: CalendarSourceConfig): ListParams {
  const pad = source.end_date ? source.max_span : 0
  const b = windowBounds(view.value, anchor.value, weekStart.value, timezone.value, pad)
  const params: ListParams = {}
  params[`filter[${source.date}][gte]`] = b.gte
  params[`filter[${source.date}][lt]`] = b.lt

  // A source's `where:` clauses are evaluated by the SERVER, in the same
  // request as the date window. Pushing them down rather than filtering the
  // response client-side keeps the filter running against the raw entity,
  // which is what stops calendar membership varying by principal when a
  // clause names a `visible:`-redacted property.
  for (const clause of source.where ?? []) {
    const parsed = parseWhereClause(clause)
    if (!parsed) {
      // Config was validated at load, so an unparseable clause means the two
      // parsers disagree. Refuse to widen the query silently.
      console.warn(`[calendar] ignoring unparseable where clause: ${clause}`)
      continue
    }
    params[buildFilterKey(parsed.property, parsed.operator) as `filter[${string}]`] = parsed.value
  }
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
/** True when any chip field names a relation: only then must the server embed
 * related entities so their titles can be resolved (kanban does the same). */
const hasRelationFields = computed(
  () => config.value?.event?.fields?.some((f) => !!f.relation) ?? false
)

// The world rides every source query: the grid is that world's projection,
// exactly as a list or a board is. Without it a `?world=published` calendar
// showed the DEFAULT faces' dates under a read-only framing.
const sourceQueries = computed(() =>
  (config.value?.sources ?? []).map((source) => ({
    source,
    params: {
      ...paramsFor(source),
      ...(worldParam.value ? { world: worldParam.value } : {}),
      ...(hasRelationFields.value ? { include: '*' } : {}),
    },
  }))
)

/**
 * A fetched source's rows, identified by its INDEX in `sources`.
 *
 * Index is the only real identity: two sources over the same entity type and
 * date property differing only by `where:` is the documented way to express OR
 * (the filter language has none), so any key built from type+property would
 * collide and render one source's entities under the other's colour.
 */
interface SourceResult {
  entities: Entity[]
  included: Record<string, Entity>
  truncated: boolean
}

const fetched = ref<SourceResult[]>([])
/**
 * True only until the FIRST window arrives.
 *
 * Deliberately not "a fetch is in flight": navigating months would then blank
 * the grid on every step, and paging through a few months flickers badly. The
 * previous month stays on screen and is replaced when the new data lands —
 * stale-while-revalidate, the same shape the kanban board uses to keep an SSE
 * refetch from blanking it.
 */
const initialLoad = ref(true)
/** A refetch is in flight. Drives a subtle busy hint, never a blanked grid. */
const refreshing = ref(false)
const loadError = ref('')

// pageState mirrors DynamicForm's `form-state-*` contract: a stable signal
// that this screen has finished resolving, so a screenshot{} capture can wait
// for it rather than hanging until its timeout.
const pageState = computed<'pending' | 'loaded' | 'error'>(() => {
  if (loadError.value) return 'error'
  return initialLoad.value ? 'pending' : 'loaded'
})

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

  // Cleared before the empty-source early return, not after: otherwise a failed
  // load followed by a switch to a calendar with no sources leaves the error
  // banner up and (since the template gates the grid on it) hides the grid.
  loadError.value = ''

  if (!queries.length) {
    fetched.value = []
    initialLoad.value = false
    refreshing.value = false
    inFlight = null
    return
  }
  refreshing.value = true
  try {
    const results = await Promise.all(
      queries.map(async ({ source, params }): Promise<SourceResult> => {
        const res = await listAllEntities(source.entity, params, controller.signal)
        // Publish into the cache entry the optimistic write and SSE
        // invalidation target, so a drag updates the grid immediately.
        queryCache.setQueryData([...entityKeys.listParams(source.entity, params)], res)
        return {
          entities: res.data,
          included: res.included ?? {},
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
    if (generation === refetchGeneration) {
      initialLoad.value = false
      refreshing.value = false
    }
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
  (config.value?.sources ?? []).map((source, i) => ({
    source,
    entities: fetched.value[i]?.entities ?? [],
    included: fetched.value[i]?.included ?? {},
    schema: schemaStore.getEntityType(source.entity),
  }))
)

const allEvents = useCalendarEvents(config, sourceData, timezone, (rel) =>
  schemaStore.getInverseName(rel) ?? ''
)

/**
 * Events minus any whose source is toggled off.
 *
 * Filtered after fetching rather than by skipping the request: toggling is a
 * glance-level gesture, and refetching on every click would make it feel
 * slower than the thing it is hiding. The data is already in the cache.
 */
const events = computed(() =>
  hiddenSources.value.length
    ? allEvents.value.filter((ev) => !hiddenSources.value.includes(ev.sourceIndex))
    : allEvents.value
)
const byDay = computed(() => eventsByDay(events.value, days.value))

/** True when any source hit the page cap: the grid is incomplete and says so
 * rather than looking merely quiet. */
const truncated = computed(() => fetched.value.some((r) => r.truncated))

/** An event longer than its source's max_span may fall outside the fetched
 * window, so the caveat is surfaced instead of the event silently missing. */
const longEventWarning = computed(() => {
  const sources = config.value?.sources ?? []
  for (const ev of events.value) {
    // By index, not by entity type: two sources over the same type differing
    // only by `where:` would otherwise both resolve to the first one's span.
    const src = sources[ev.sourceIndex]
    if (!src?.end_date) continue
    if (daysBetween(ev.startDay, ev.endDay) > src.max_span) return true
  }
  return false
})

// No `?? 4` fallback: the server normalizes every calendar default at config
// load, so a second default here would silently diverge if that one changed.
const maxPerDay = computed(() => config.value?.max_events_per_day ?? 0)
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

// ANDs in `!isWorldBound`, for the reason KanbanView's canUpdate documents:
// `_actions` knows nothing about the request's world, and a non-default world
// is READ-ONLY on this API. The stake is the same as on a board — the write is
// triggered by a DRAG, so an event that accepts the gesture and animates to a
// new day has already told the reader their change landed. Since a bare write
// carries no `?world=`, the server has no parameter to refuse: the reschedule
// would silently hit the DEFAULT face of an entity the reader is viewing
// through a world.
function canUpdate(entity: Entity): boolean {
  return actionAllowed(entity, 'update') && !isWorldBound.value
}

/**
 * Clicking a chip PREVIEWS the entity rather than editing it.
 *
 * The previous behaviour jumped straight into `edit_form` when one was
 * configured, which put a save button in front of someone who had only clicked
 * a small chip to find out what it was. The modal answers that question first
 * and offers Edit as an explicit next step.
 */
const preview = ref<{ type: string; id: string } | null>(null)

function openEvent(ev: CalendarEvent) {
  preview.value = { type: ev.entityType, id: ev.entity.id }
}

function closePreview() {
  preview.value = null
}

function createNew() {
  const form = config.value?.create_form
  if (form) void router.push(`/form/${form}`)
}

/**
 * Whether to offer the "New" button.
 *
 * Deliberately NOT derived from an entity's `_actions`: that answers "may I
 * create something of source #0's type", which for a multi-source calendar may
 * not even be what `create_form` targets — and it returns nothing useful when
 * the grid is empty, which is exactly when the button matters most.
 *
 * The button is an affordance, not a gate: the server rejects an unauthorized
 * create regardless. So this shows it whenever a form is configured and lets
 * the real check happen where it is enforced, rather than guessing from
 * unrelated data and hiding a button the user may legitimately use.
 */
function canCreate(): boolean {
  // ANDs in the world like canUpdate: a create lands in the default world, so
  // a "+ New" on a world-bound calendar offers a write this request cannot
  // carry (the same reasoning KanbanView's canCreate documents).
  return !!config.value?.create_form && !isWorldBound.value
}

// --- Drag to reschedule ---

/**
 * The event being dragged, and the day the drag STARTED from.
 *
 * The grabbed day matters for a multi-day event: it is drawn once per day it
 * covers, so grabbing its middle segment and dropping one day later means
 * "shift by one", not "move the start here". Measuring from the event's start
 * instead made that gesture jump by the offset — grabbing day 3 of an 11-15
 * event and moving it one day landed it on 14-18 rather than 12-16.
 */
const dragged = ref<{ event: CalendarEvent; from: CalendarDay } | null>(null)

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
    // settleOptimistic invalidates the list key the grid's entries descend
    // from, so the refetch happens through the cache. Calling refetchGrid()
    // here as well would issue a second round-trip per source and flash the
    // loading state over an already-correct optimistic grid.
    await settleOptimistic(queryCache, context)
    await refetchGrid()
  },
})

function onDragStart(payload: { event: CalendarEvent; day: CalendarDay; native: DragEvent }) {
  dragged.value = { event: payload.event, from: payload.day }
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
  const held = dragged.value
  dragged.value = null
  if (!held) return
  const ev = held.event

  // Defence in depth: :draggable="false" stops a drag starting here, but an
  // external drag source can still fire this handler.
  if (!canUpdate(ev.entity)) return

  // Measured from the day the user GRABBED, not the event's start: dragging
  // the middle of a span one day right means "shift by one day".
  const delta = daysBetween(held.from, day)
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

// Deliberately no watcher materializing the anchor into the URL on a view
// switch. Writing the anchor back here changes the query, which retriggers the
// fetch watcher above, which re-renders and writes again — a refetch loop that
// issued 16 requests where 2 were due. The anchor already reaches the URL
// whenever the user actually navigates, which is when a shareable link matters.
</script>

<template>
  <div class="calendar-view" :data-testid="`page-state-${pageState}`">
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
        <!-- A quiet hint, not a spinner over the grid: the previous period
             stays readable while the next one loads. -->
        <span v-if="refreshing && !initialLoad" class="calendar-refreshing" aria-live="polite">
          updating…
        </span>
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

    <CalendarLegend
      :sources="config?.sources ?? []"
      :hidden="hiddenSources"
      @toggle="toggleSource"
    />

    <div v-if="truncated" class="truncation-banner truncation-banner--cap" role="alert">
      Some events are not shown — this period returned more entities than the calendar can load.
    </div>
    <div v-if="longEventWarning" class="truncation-banner truncation-banner--span" role="alert">
      Some long events may not be shown; increase <code>max_span</code> for this calendar's sources.
    </div>

    <div v-if="loadError" class="error-state">{{ loadError }}</div>
    <div v-else-if="initialLoad" class="loading-state">Loading…</div>

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
      :dragging-id="dragged?.event.id"
      @expand="expandDay"
      @open="openEvent"
      @dragstart="onDragStart"
      @dragend="onDragEnd"
      @dragover="onDragOver"
      @drop="onDrop"
    />

    <p v-if="!initialLoad && !refreshing && !loadError && !events.length" class="calendar-empty">
      <!-- Distinguishes "nothing here" from "you hid it": telling a reader the
           period is empty when they switched every source off is misleading,
           and leaves them without the hint that the legend is the way back. -->
      <template v-if="allEvents.length">
        All sources are hidden — use the legend above to show them again.
      </template>
      <template v-else>No events in this period.</template>
    </p>

    <!-- eslint-disable-next-line vue/no-v-html -- sanitized by renderMarkdown -->
    <div v-if="footerHtml" class="view-info view-info--bottom" v-html="footerHtml" />

    <EntityPreviewModal
      v-if="preview"
      :open="!!preview"
      :entity-type="preview.type"
      :entity-id="preview.id"
      :edit-form="config?.edit_form"
      @close="closePreview"
    />
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

.calendar-refreshing {
  color: var(--muted-text);
  font-size: var(--font-size-sm);
}

.calendar-views .btn.active {
  background: var(--accent-color);
  color: #fff;
}

.calendar-empty {
  margin-top: var(--space-md);
  color: var(--muted-text);
}
</style>
