<script setup lang="ts">
/**
 * The month or week grid: a weekday header row, then one cell per visible day.
 *
 * It owns no state. The days and the events are passed in and every
 * interaction is re-emitted, so navigating between months, fetching the events
 * and writing a drag back all stay with the app.
 *
 * A multi-day event is drawn once per day it covers, rather than as one bar
 * laid across the row. Per-day chips keep each cell independently scrollable
 * and capped, and the chip's own continuation marker carries what the bar
 * would have shown.
 */
import { computed } from 'vue'
import type { CalendarDay, CalendarView } from './calendarGrid'
import { dayKey, daysBetween, isSameMonth, sameDay, weekdayLabels } from './calendarGrid'
import type { CalendarDragPayload, CalendarEvent } from './types'
import RlCalendarEventChip from './RlCalendarEventChip.vue'

const props = withDefaults(
  defineProps<{
    /** Month opens a whole-weeks grid; week opens a single, taller row. */
    view?: CalendarView
    /**
     * The days to draw: consecutive and in order, as `visibleDays` returns
     * them. Events are placed by their offset from the first day, so a list
     * with gaps would misplace them.
     */
    days: CalendarDay[]
    /**
     * The day the period is centred on. In a month view it decides which cells
     * are spill days from the neighbouring months.
     */
    anchor: CalendarDay
    /** Marked as today. Which day that is depends on the app's display
     * timezone, so it is passed in rather than read from the clock here. */
    today?: CalendarDay
    /**
     * Every event to place, with its real extent. A chip appears in each
     * visible day between its `startDay` and `endDay`, so an event reaching
     * into the period from outside still shows on the days it covers.
     */
    events: CalendarEvent[]
    /** Column headers. Defaults to the platform's locale weekday names. */
    weekdayLabels?: string[]
    /**
     * Most chips to draw in one cell before collapsing the rest into a
     * "+n more" button. Zero means no cap.
     */
    maxPerDay?: number
    /** Id of the event being dragged, so every day of a span shows it is in
     * flight. */
    draggingId?: string
  }>(),
  { view: 'month', maxPerDay: 0 },
)

const emit = defineEmits<{
  /** The "+n more" button in a capped cell. */
  expand: [day: CalendarDay]
  open: [event: CalendarEvent]
  dragstart: [payload: CalendarDragPayload]
  dragend: []
  drop: [day: CalendarDay, native: DragEvent]
}>()

const headers = computed(() => props.weekdayLabels ?? weekdayLabels())

/**
 * Events per day, bucketed once for the whole grid.
 *
 * Asking each cell to filter the event list would walk every event 42 times in
 * a month view. Here each event instead walks only the days it actually
 * covers, found by its offset from the first visible day, so the work is
 * proportional to what is drawn rather than to cells times events.
 */
const byDay = computed(() => {
  const buckets = new Map<string, CalendarEvent[]>()
  for (const day of props.days) buckets.set(dayKey(day), [])
  if (props.days.length === 0) return buckets

  const first = props.days[0]
  for (const event of props.events) {
    // Clamp the event's real extent to the visible window. An event reaching
    // in from outside still fills the days it covers here, which is what lets
    // its chip read as a continuation rather than as a fresh start.
    const from = Math.max(0, daysBetween(first, event.startDay))
    const to = Math.min(props.days.length - 1, daysBetween(first, event.endDay))
    for (let i = from; i <= to; i++) {
      buckets.get(dayKey(props.days[i]))!.push(event)
    }
  }

  // All-day events lead, then timed ones in clock order. Sorting by the label
  // works because it is zero-padded `HH:MM`; an app formatting times some
  // other way should pass its events in the order it wants them shown, since
  // a stable sort leaves equal keys alone.
  for (const bucket of buckets.values()) {
    bucket.sort((a, b) => (a.timeLabel ?? '').localeCompare(b.timeLabel ?? ''))
  }
  return buckets
})

function eventsFor(day: CalendarDay): CalendarEvent[] {
  const all = byDay.value.get(dayKey(day)) ?? []
  return props.maxPerDay > 0 ? all.slice(0, props.maxPerDay) : all
}

function hiddenCount(day: CalendarDay): number {
  if (props.maxPerDay <= 0) return 0
  return Math.max(0, (byDay.value.get(dayKey(day)) ?? []).length - props.maxPerDay)
}

function isToday(day: CalendarDay): boolean {
  return props.today ? sameDay(day, props.today) : false
}

/** A spill day: in the grid because a week straddles a month boundary. */
function isOutside(day: CalendarDay): boolean {
  return props.view === 'month' && !isSameMonth(day, props.anchor)
}

/**
 * A drop target must cancel dragover, or the browser treats the cell as
 * refusing the drag and never fires `drop`.
 */
function onDragOver(native: DragEvent) {
  native.preventDefault()
}

function onDrop(day: CalendarDay, native: DragEvent) {
  native.preventDefault()
  emit('drop', day, native)
}
</script>

<template>
  <div class="rl-calendar-grid" :class="`rl-calendar-grid--${view}`">
    <div v-for="(label, i) in headers" :key="`h-${i}`" class="rl-calendar-grid__weekday">
      {{ label }}
    </div>

    <div
      v-for="day in days"
      :key="dayKey(day)"
      class="rl-calendar-grid__day"
      :class="{
        'rl-calendar-grid__day--today': isToday(day),
        'rl-calendar-grid__day--outside': isOutside(day),
      }"
      @dragover="onDragOver"
      @drop="onDrop(day, $event)"
    >
      <div class="rl-calendar-grid__day-number">
        <span v-if="isToday(day)" class="rl-visually-hidden">Today, </span>{{ day.day }}
      </div>

      <div class="rl-calendar-grid__events">
        <RlCalendarEventChip
          v-for="event in eventsFor(day)"
          :key="event.id"
          :event="event"
          :day="day"
          :dragging="event.id === draggingId"
          @open="emit('open', $event)"
          @dragstart="emit('dragstart', $event)"
          @dragend="emit('dragend')"
        />

        <button
          v-if="hiddenCount(day) > 0"
          type="button"
          class="rl-calendar-grid__more rl-focus-ring"
          @click="emit('expand', day)"
        >
          +{{ hiddenCount(day) }} more
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
/*
 * Grid lines are a 1px gap over a border-coloured background, with the cells
 * painted on top: every seam is then exactly one line, with no doubling at the
 * joins and no half-pixel rounding. Subtle by design — the lines should
 * structure the month, not compete with the events in it.
 */
.rl-calendar-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 1px;
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-border);
  overflow: hidden;
}

/*
 * A week grid is a header row plus one day row, so the day row needs an
 * explicit height: `min-height` on a grid item does not open a row that has
 * nothing else holding it, which would leave a seven-column strip above dead
 * space. The header row stays `auto` so it is not stretched with the days.
 */
.rl-calendar-grid--week {
  grid-template-rows: auto minmax(420px, auto);
}

.rl-calendar-grid__weekday {
  padding: var(--rl-space-1) var(--rl-space-2);
  background: var(--rl-color-bg-sunken);
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
  font-weight: var(--rl-font-weight-semibold);
  text-align: center;
}

.rl-calendar-grid__day {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-1);
  /* Sized for readable chips (two-line titles) rather than the densest
     possible grid. A month being taller than a small viewport is the accepted
     cost of chips people can actually read. */
  min-height: 128px;
  /* Lets a long chip truncate instead of stretching the column. */
  min-width: 0;
  padding: var(--rl-space-1);
  background: var(--rl-color-bg);
}

.rl-calendar-grid__day--outside {
  background: var(--rl-color-bg-sunken);
}

.rl-calendar-grid__day--outside .rl-calendar-grid__day-number {
  color: var(--rl-color-text-muted);
}

.rl-calendar-grid__day--today .rl-calendar-grid__day-number {
  background: var(--rl-color-accent);
  color: var(--rl-color-text-inverse);
  border-radius: var(--rl-radius-pill);
}

.rl-calendar-grid__day-number {
  align-self: flex-start;
  min-width: 1.5rem;
  padding: 1px var(--rl-space-1);
  font-size: var(--rl-font-size-xs);
  text-align: center;
}

.rl-calendar-grid__events {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.rl-calendar-grid__more {
  padding: 1px var(--rl-space-1);
  border: none;
  border-radius: var(--rl-radius-sm);
  background: none;
  color: var(--rl-color-text-muted);
  font-family: inherit;
  font-size: var(--rl-font-size-xs);
  text-align: left;
  cursor: pointer;
}

.rl-calendar-grid__more:hover {
  text-decoration: underline;
}
</style>
