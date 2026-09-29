<script setup lang="ts">
/**
 * One event on the grid.
 *
 * A chip is a dense surface, like a table cell or a kanban card: it shows a
 * time, a title and a few detail lines, and an empty value renders as nothing
 * rather than as a placeholder. Formatting the values is the caller's job, so
 * the chip has no opinion about dates, currencies or locales.
 */
import { computed } from 'vue'
import type { CalendarDay } from './calendarGrid'
import { compareDays, sameDay } from './calendarGrid'
import type { CalendarDragPayload, CalendarEvent } from './types'

const props = withDefaults(
  defineProps<{
    event: CalendarEvent
    /**
     * The grid day this chip is being rendered in. A multi-day event is drawn
     * once per day it spans, and the chip has to know which day it is to show
     * the right continuation marker.
     */
    day: CalendarDay
    /**
     * True while any segment of this event is being dragged, so every day of a
     * span lifts together and the user can see what they picked up.
     */
    dragging?: boolean
  }>(),
  { dragging: false },
)

const emit = defineEmits<{
  open: [event: CalendarEvent]
  dragstart: [payload: CalendarDragPayload]
  dragend: []
}>()

/**
 * Which segment of a multi-day event this chip is.
 *
 * A spanning event is drawn as one chip per day, so without a marker three
 * identical chips in a row read as three separate events, which is what makes
 * a three-day conference indistinguishable from a daily stand-up. The arrows
 * say "this continues": `→` it runs on, `←` it began earlier, `↔` both.
 */
const span = computed<'single' | 'start' | 'middle' | 'end'>(() => {
  const { startDay, endDay } = props.event
  if (compareDays(startDay, endDay) === 0) return 'single'
  if (sameDay(props.day, startDay)) return 'start'
  if (sameDay(props.day, endDay)) return 'end'
  return 'middle'
})

const SPAN_MARKERS = {
  start: { marker: '→', description: 'Continues on following days' },
  middle: { marker: '↔', description: 'Continues before and after this day' },
  end: { marker: '←', description: 'Continued from earlier days' },
} as const

const spanMarker = computed(() =>
  span.value === 'single' ? undefined : SPAN_MARKERS[span.value],
)

const fields = computed(() => props.event.fields ?? [])
</script>

<template>
  <button
    type="button"
    class="rl-calendar-chip rl-focus-ring"
    :class="[
      `rl-calendar-chip--${event.color ?? 'blue'}`,
      { 'rl-calendar-chip--dragging': dragging },
    ]"
    :draggable="event.draggable ? 'true' : 'false'"
    :title="event.summary"
    @click="emit('open', event)"
    @dragstart="emit('dragstart', { event, day, native: $event })"
    @dragend="emit('dragend')"
  >
    <span class="rl-calendar-chip__headline">
      <span v-if="event.timeLabel" class="rl-calendar-chip__time">{{ event.timeLabel }}</span>
      <span class="rl-calendar-chip__title">{{ event.summary }}</span>
      <!-- The arrow is decorative; the sentence is what is spoken. -->
      <span
        v-if="spanMarker"
        class="rl-calendar-chip__span"
        :title="spanMarker.description"
        :aria-label="spanMarker.description"
        >{{ spanMarker.marker }}</span
      >
    </span>

    <span v-if="fields.length" class="rl-calendar-chip__fields">
      <span v-for="(field, i) in fields" :key="i" class="rl-calendar-chip__field">
        <span v-if="field.label" class="rl-calendar-chip__field-label">{{ field.label }}:</span>
        <span class="rl-calendar-chip__field-value">{{ field.value }}</span>
      </span>
    </span>
  </button>
</template>

<style scoped>
.rl-calendar-chip {
  display: flex;
  flex-direction: column;
  gap: 1px;
  width: 100%;
  /* Never widen the day cell: a long title wraps instead. */
  min-width: 0;
  padding: var(--rl-space-1) var(--rl-space-2);
  border: none;
  border-radius: var(--rl-radius-sm);
  background: var(--rl-calendar-chip-bg);
  color: var(--rl-color-text);
  /*
   * The chip's own muted colour. `--rl-color-text-muted` is tuned for the
   * neutral page surfaces and lands between 4.15 and 4.41 against these
   * pastel backgrounds, under the 4.5 small text needs. This is the grey
   * already in the tag palette, which clears every chip colour.
   */
  --rl-calendar-chip-muted: var(--rl-tag-grey-fg);
  font-family: inherit;
  /* Small rather than extra-small: a chip is the primary content of a day
     cell, not a dense table cell, and a title nobody can read is not
     information. */
  font-size: var(--rl-font-size-sm);
  line-height: var(--rl-line-height-tight);
  text-align: left;
  cursor: pointer;
}

.rl-calendar-chip:hover {
  filter: brightness(0.97);
}

.rl-calendar-chip[draggable='true'] {
  cursor: grab;
}

/*
 * Every segment of a dragged span lifts, not just the one under the cursor.
 * Otherwise picking up the middle of a five-day event gives no sign that the
 * other four days are coming with it.
 */
.rl-calendar-chip--dragging {
  opacity: 0.55;
  outline: 1px dashed var(--rl-calendar-chip-accent);
  outline-offset: 1px;
}

.rl-calendar-chip__headline {
  display: flex;
  align-items: baseline;
  gap: var(--rl-space-1);
  min-width: 0;
}

.rl-calendar-chip__time {
  flex: none;
  color: var(--rl-calendar-chip-muted);
  font-variant-numeric: tabular-nums;
}

.rl-calendar-chip__title {
  /* Wraps to at most two lines rather than truncating at one: "Long sprint"
     losing its second word to an ellipsis is the case this exists for. The
     hard cap keeps one verbose title from pushing its siblings out of the
     cell. */
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  overflow-wrap: anywhere;
}

/* Sits at the far edge so a column of chips can be scanned for continuations
   without reading each title. */
.rl-calendar-chip__span {
  flex: none;
  margin-left: auto;
  padding-left: var(--rl-space-1);
  color: var(--rl-calendar-chip-muted);
  font-size: var(--rl-font-size-xs);
  line-height: 1;
}

.rl-calendar-chip__fields {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

/* One field per line, label and value on the same row. Stacked rather than
   flowing: a run of values with no line breaks reads as a sentence, and two
   person fields become indistinguishable. */
.rl-calendar-chip__field {
  display: flex;
  align-items: baseline;
  gap: var(--rl-space-1);
  min-width: 0;
  font-size: var(--rl-font-size-xs);
}

.rl-calendar-chip__field-label {
  flex: none;
  color: var(--rl-calendar-chip-muted);
}

.rl-calendar-chip__field-value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/*
 * Chip colours reuse the tag palette rather than a private set of hex values.
 * Those tokens are already tuned for contrast, so a calendar restyles with the
 * rest of the library instead of pinning colours a theme cannot reach.
 */
.rl-calendar-chip--grey {
  --rl-calendar-chip-accent: var(--rl-tag-grey-fg);
  --rl-calendar-chip-bg: var(--rl-tag-grey-bg);
}
.rl-calendar-chip--blue {
  --rl-calendar-chip-accent: var(--rl-tag-blue-fg);
  --rl-calendar-chip-bg: var(--rl-tag-blue-bg);
}
.rl-calendar-chip--green {
  --rl-calendar-chip-accent: var(--rl-tag-green-fg);
  --rl-calendar-chip-bg: var(--rl-tag-green-bg);
}
.rl-calendar-chip--amber {
  --rl-calendar-chip-accent: var(--rl-tag-amber-fg);
  --rl-calendar-chip-bg: var(--rl-tag-amber-bg);
}
.rl-calendar-chip--red {
  --rl-calendar-chip-accent: var(--rl-tag-red-fg);
  --rl-calendar-chip-bg: var(--rl-tag-red-bg);
}
.rl-calendar-chip--purple {
  --rl-calendar-chip-accent: var(--rl-tag-purple-fg);
  --rl-calendar-chip-bg: var(--rl-tag-purple-bg);
}
</style>
