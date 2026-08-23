<script setup lang="ts">
/**
 * One event on the grid.
 *
 * A chip is a DENSE surface, like a list cell or a kanban card: property values
 * render through the widget registry in display mode, and an empty value
 * renders as nothing rather than a placeholder. Widget resolution happens once
 * per configured field in the parent and is passed in, not recomputed per chip
 * — resolving per event would walk the registry (and possibly warn) once per
 * event per render.
 */
import { computed } from 'vue'
import type { CalendarEvent } from '@/composables/useCalendarEvents'
import CardFieldList from '@/components/common/CardFieldList.vue'
import { compareDays, sameDay, type CalendarDay } from '@/utils/calendarGrid'

const props = defineProps<{
  event: CalendarEvent
  /** The grid day this chip is being rendered in. A multi-day event renders
   * once per day it spans, and the chip has to know WHICH day to show the
   * right continuation marker. */
  day: CalendarDay
  /** Whether this event may be dragged (the ACL affordance, not a style). */
  draggable: boolean
}>()

/**
 * Which segment of a multi-day event this chip is.
 *
 * A spanning event is drawn as one chip per day, so without a marker three
 * consecutive identical chips read as three separate events — the thing that
 * makes a conference indistinguishable from a daily standup. The arrows say
 * "this continues": `→` it runs on, `←` it began earlier, `↔` both.
 */
const span = computed<'single' | 'start' | 'middle' | 'end'>(() => {
  const { startDay, endDay } = props.event
  if (compareDays(startDay, endDay) === 0) return 'single'
  if (sameDay(props.day, startDay)) return 'start'
  if (sameDay(props.day, endDay)) return 'end'
  return 'middle'
})

const spanMarker = computed(() => {
  switch (span.value) {
    case 'start':
      return '→'
    case 'end':
      return '←'
    case 'middle':
      return '↔'
    default:
      return ''
  }
})

/** Spoken by screen readers in place of the arrow, which is decorative. */
const spanDescription = computed(() => {
  switch (span.value) {
    case 'start':
      return 'Continues on following days'
    case 'end':
      return 'Continued from earlier days'
    case 'middle':
      return 'Continues before and after this day'
    default:
      return ''
  }
})

defineEmits<{
  (e: 'open', event: CalendarEvent): void
  (e: 'dragstart', payload: { event: CalendarEvent; native: DragEvent }): void
  (e: 'dragend'): void
}>()
</script>

<template>
  <button
    type="button"
    class="calendar-chip"
    :class="[`calendar-chip--${event.color}`, { 'calendar-chip--timed': event.timed }]"
    :draggable="draggable ? 'true' : 'false'"
    :title="event.summary"
    @click="$emit('open', event)"
    @dragstart="$emit('dragstart', { event, native: $event })"
    @dragend="$emit('dragend')"
  >
    <span class="calendar-chip-headline">
      <span v-if="event.timeLabel" class="calendar-chip-time">{{ event.timeLabel }}</span>
      <span class="calendar-chip-title">{{ event.summary }}</span>
      <span
        v-if="spanMarker"
        class="calendar-chip-span"
        :title="spanDescription"
        :aria-label="spanDescription"
        >{{ spanMarker }}</span
      >
    </span>
    <CardFieldList :fields="event.fields" :entity-type="event.entityType" />
  </button>
</template>

<style scoped>
.calendar-chip {
  display: flex;
  flex-direction: column;
  gap: 1px;
  width: 100%;
  padding: var(--space-xs);
  border: none;
  border-left: 3px solid var(--calendar-chip-accent, var(--accent-color));
  border-radius: var(--radius-sm);
  background: var(--calendar-chip-bg, var(--hover-bg));
  color: var(--text-color);
  /* Base rather than sm: a chip is the primary content of a day cell, not a
     dense table cell, and a title nobody can read is not information. */
  font-size: var(--font-size-base);
  line-height: 1.3;
  text-align: left;
  cursor: pointer;
  /* Never widen the day cell — a long title wraps instead. */
  min-width: 0;
}

.calendar-chip:hover {
  filter: brightness(0.97);
}

.calendar-chip[draggable='true'] {
  cursor: grab;
}

.calendar-chip-title {
  /* Wraps to at most two lines rather than truncating at one: "Long sprint"
     losing its second word to an ellipsis is the case this exists for. A hard
     cap keeps one verbose title from pushing every sibling out of the cell. */
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  overflow-wrap: anywhere;
}

.calendar-chip-headline {
  display: flex;
  align-items: baseline;
  gap: var(--space-xs);
  min-width: 0;
}

.calendar-chip-time {
  flex: none;
  color: var(--muted-text);
  font-variant-numeric: tabular-nums;
}

/* Sits at the far edge so the eye can scan a column of chips for continuations
   without reading each title. */
.calendar-chip-span {
  flex: none;
  margin-left: auto;
  padding-left: var(--space-xs);
  color: var(--muted-text);
  font-size: var(--font-size-sm);
  line-height: 1;
}

/* Source colours map onto the EXISTING badge palette rather than a private set
   of hex values: those tokens are already themed for light and dark, so a
   calendar restyles with the rest of the app instead of pinning colours the
   theme cannot reach. */
.calendar-chip--blue {
  --calendar-chip-accent: var(--badge-blue);
}
.calendar-chip--green {
  --calendar-chip-accent: var(--badge-green);
}
.calendar-chip--amber {
  --calendar-chip-accent: var(--badge-orange);
}
.calendar-chip--red {
  --calendar-chip-accent: var(--badge-red);
}
.calendar-chip--violet {
  --calendar-chip-accent: var(--badge-purple);
}
.calendar-chip--slate {
  --calendar-chip-accent: var(--badge-gray);
}
</style>
