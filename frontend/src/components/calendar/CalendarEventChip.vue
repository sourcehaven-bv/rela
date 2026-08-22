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
import type { CalendarEvent } from '@/composables/useCalendarEvents'

defineProps<{
  event: CalendarEvent
  /** Whether this event may be dragged (the ACL affordance, not a style). */
  draggable: boolean
}>()

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
    <span v-if="event.timeLabel" class="calendar-chip-time">{{ event.timeLabel }}</span>
    <span class="calendar-chip-title">{{ event.summary }}</span>
    <span v-if="event.fields.length" class="calendar-chip-fields">
      <template v-for="field in event.fields" :key="field.key">
        <component
          :is="field.component"
          v-if="field.component"
          :model-value="field.modelValue"
          :property-name="field.propertyName"
          mode="display"
        />
        <span v-else class="calendar-chip-field">{{ field.text }}</span>
      </template>
    </span>
  </button>
</template>

<style scoped>
.calendar-chip {
  display: flex;
  align-items: baseline;
  gap: var(--space-xs);
  width: 100%;
  padding: 2px var(--space-xs);
  border: none;
  border-left: 3px solid var(--calendar-chip-accent, var(--color-primary));
  border-radius: var(--radius-sm);
  background: var(--calendar-chip-bg, var(--color-surface-alt));
  color: var(--color-text);
  font-size: var(--font-size-sm);
  text-align: left;
  cursor: pointer;
  /* A chip must never widen its day cell: a long title truncates instead. */
  min-width: 0;
  overflow: hidden;
}

.calendar-chip:hover {
  filter: brightness(0.97);
}

.calendar-chip[draggable='true'] {
  cursor: grab;
}

.calendar-chip-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.calendar-chip-time {
  flex: none;
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
}

.calendar-chip-fields {
  display: flex;
  flex: none;
  gap: var(--space-xs);
  margin-left: auto;
  color: var(--color-text-muted);
}

/* Palette tokens. The theme owns the actual colours; config only names one. */
.calendar-chip--blue {
  --calendar-chip-accent: var(--color-cal-blue, #3b82f6);
}
.calendar-chip--green {
  --calendar-chip-accent: var(--color-cal-green, #10b981);
}
.calendar-chip--amber {
  --calendar-chip-accent: var(--color-cal-amber, #f59e0b);
}
.calendar-chip--red {
  --calendar-chip-accent: var(--color-cal-red, #ef4444);
}
.calendar-chip--violet {
  --calendar-chip-accent: var(--color-cal-violet, #8b5cf6);
}
.calendar-chip--slate {
  --calendar-chip-accent: var(--color-cal-slate, #64748b);
}
</style>
