<script setup lang="ts">
/**
 * The month/week grid: weekday header row, then one cell per visible day.
 *
 * Split from CalendarView so the view keeps data and interaction concerns and
 * this keeps layout — and so neither grows into the god-component the lint
 * budget exists to prevent. It owns no state: days, events and the ACL
 * predicate are all passed in, and every interaction is re-emitted upward.
 */
import CalendarEventChip from './CalendarEventChip.vue'
import type { CalendarEvent } from '@/composables/useCalendarEvents'
import type { Entity } from '@/types'
import { dayKey, isSameMonth, sameDay, type CalendarDay, type CalendarView } from '@/utils/calendarGrid'

defineProps<{
  view: CalendarView
  days: CalendarDay[]
  weekdayLabels: string[]
  anchor: CalendarDay
  today: CalendarDay
  /** Events to render for a day, already capped by the parent. */
  eventsForDay: (day: CalendarDay) => CalendarEvent[]
  /** How many further events the cap is hiding for a day. */
  hiddenCount: (day: CalendarDay) => number
  canUpdate: (entity: Entity) => boolean
}>()

defineEmits<{
  (e: 'expand', day: CalendarDay): void
  (e: 'open', event: CalendarEvent): void
  (e: 'dragstart', payload: { event: CalendarEvent; native: DragEvent }): void
  (e: 'dragend'): void
  (e: 'dragover', native: DragEvent): void
  (e: 'drop', native: DragEvent, day: CalendarDay): void
}>()
</script>

<template>
    <div class="calendar-grid" :class="`calendar-grid--${view}`">
      <div v-for="(label, i) in weekdayLabels" :key="`h-${i}`" class="calendar-weekday">
        {{ label }}
      </div>

      <div
        v-for="day in days"
        :key="dayKey(day)"
        class="calendar-day"
        :class="{
          'calendar-day--today': sameDay(day, today),
          'calendar-day--outside': view === 'month' && !isSameMonth(day, anchor),
        }"
        @dragover="(e: DragEvent) => $emit('dragover', e)"
        @drop="$emit('drop', $event, day)"
      >
        <div class="calendar-day-number">{{ day.day }}</div>
        <div class="calendar-day-events">
          <CalendarEventChip
            v-for="ev in eventsForDay(day)"
            :key="ev.id"
            :event="ev"
            :draggable="canUpdate(ev.entity)"
            @open="(e) => $emit('open', e)"
            @dragstart="(p) => $emit('dragstart', p)"
            @dragend="$emit('dragend')"
          />
          <button
            v-if="hiddenCount(day) > 0"
            type="button"
            class="calendar-more"
            @click="$emit('expand', day)"
          >
            +{{ hiddenCount(day) }} more
          </button>
        </div>
      </div>
    </div>
</template>

<style scoped>
.calendar-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 1px;
  background: var(--color-border);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.calendar-weekday {
  padding: var(--space-xs) var(--space-sm);
  background: var(--color-surface-alt);
  font-size: var(--font-size-sm);
  font-weight: 600;
  color: var(--color-text-muted);
  text-align: center;
}

.calendar-day {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-height: 96px;
  padding: var(--space-xs);
  background: var(--color-surface);
  /* min-width:0 lets a long chip truncate instead of stretching the column. */
  min-width: 0;
}

/* A week grid is a header row plus ONE day row, so the day row needs an
   explicit height: min-height on a grid item does not open a row that has
   nothing else holding it, which leaves a seven-column strip above dead space.
   The header row stays auto so it is not stretched with the days. */
.calendar-grid--week {
  grid-template-rows: auto minmax(420px, auto);
}

.calendar-day--outside {
  background: var(--color-surface-alt);
}

.calendar-day--outside .calendar-day-number {
  color: var(--color-text-muted);
}

.calendar-day--today .calendar-day-number {
  background: var(--color-primary);
  color: var(--color-on-primary, #fff);
  border-radius: 50%;
}

.calendar-day-number {
  align-self: flex-start;
  min-width: 1.5rem;
  padding: 1px 4px;
  font-size: var(--font-size-sm);
  text-align: center;
}

.calendar-day-events {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.calendar-more {
  padding: 1px var(--space-xs);
  border: none;
  background: none;
  color: var(--color-text-muted);
  font-size: var(--font-size-sm);
  text-align: left;
  cursor: pointer;
}

.calendar-more:hover {
  text-decoration: underline;
}
</style>
