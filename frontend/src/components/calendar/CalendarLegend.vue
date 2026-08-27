<script setup lang="ts">
/**
 * Source legend: names each source and toggles its events on and off.
 *
 * A multi-source calendar mixes things that mean different things — tasks and
 * meetings, or the same type split by `where:` — and a colour alone only says
 * "these differ", not what each one is. The legend names them and lets a
 * reader take one away.
 *
 * Hidden sources live in the URL (see CalendarView), so a filtered view is
 * shareable and survives a refresh, like the period and view are.
 *
 * Rendered only for 2+ sources: a single-source calendar has nothing to
 * distinguish, and a lone legend entry beside its own grid is noise.
 */
import type { CalendarSourceConfig } from '@/types/config'

defineProps<{
  sources: CalendarSourceConfig[]
  /** Indices of sources currently hidden. */
  hidden: number[]
}>()

defineEmits<{ (e: 'toggle', index: number): void }>()

/** A source's legend name: its label, else the entity type it projects. */
function sourceLabel(source: CalendarSourceConfig): string {
  return source.label || source.entity
}
</script>

<template>
  <div v-if="sources.length > 1" class="calendar-legend">
    <button
      v-for="(source, i) in sources"
      :key="i"
      type="button"
      class="calendar-legend-item"
      :class="[
        `calendar-legend-item--${source.color ?? 'blue'}`,
        { 'calendar-legend-item--off': hidden.includes(i) },
      ]"
      :aria-pressed="!hidden.includes(i)"
      @click="$emit('toggle', i)"
    >
      <span class="calendar-legend-swatch" aria-hidden="true" />
      <span class="calendar-legend-label">{{ sourceLabel(source) }}</span>
    </button>
  </div>
</template>

<style scoped>
.calendar-legend {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-xs);
  margin-bottom: var(--space-sm);
}

.calendar-legend-item {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  padding: 2px var(--space-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: none;
  color: var(--text-color);
  font-size: var(--font-size-sm);
  cursor: pointer;
}

.calendar-legend-item:hover {
  background: var(--hover-bg);
}

/* A hidden source stays legible rather than disappearing: it is the control
   for bringing itself back, so it must still read as its own name. */
.calendar-legend-item--off {
  color: var(--muted-text);
  text-decoration: line-through;
}

.calendar-legend-item--off .calendar-legend-swatch {
  background: none;
  border: 1px solid var(--legend-swatch, var(--accent-color));
}

.calendar-legend-swatch {
  width: 10px;
  height: 10px;
  border-radius: 2px;
  background: var(--legend-swatch, var(--accent-color));
}

.calendar-legend-item--blue {
  --legend-swatch: var(--badge-blue);
}
.calendar-legend-item--green {
  --legend-swatch: var(--badge-green);
}
.calendar-legend-item--amber {
  --legend-swatch: var(--badge-orange);
}
.calendar-legend-item--red {
  --legend-swatch: var(--badge-red);
}
.calendar-legend-item--violet {
  --legend-swatch: var(--badge-purple);
}
.calendar-legend-item--slate {
  --legend-swatch: var(--badge-gray);
}
</style>
