<script setup lang="ts">
/**
 * Source legend: names each source and toggles its events on and off.
 *
 * A multi-source calendar mixes things that mean different things — tasks and
 * meetings, or one team's work and another's — and a colour alone only says
 * "these differ", not what each one is. The legend names them and lets a
 * reader take one away.
 *
 * Sources are identified by id rather than by position, so hidden ids can be
 * kept in a URL and still mean the same thing after the source list is
 * reordered.
 */
import { computed } from 'vue'
import type { CalendarSource } from './types'

const props = withDefaults(
  defineProps<{
    sources: CalendarSource[]
    /** Ids of the sources currently hidden. */
    hidden?: string[]
    /**
     * Draw the legend even for a single source. Off by default: a lone entry
     * beside its own grid names something nothing else is competing with.
     */
    showSingle?: boolean
  }>(),
  { hidden: () => [], showSingle: false },
)

const emit = defineEmits<{ toggle: [id: string] }>()

const visible = computed(() => props.showSingle || props.sources.length > 1)

function isHidden(id: string): boolean {
  return props.hidden.includes(id)
}
</script>

<template>
  <div v-if="visible" class="rl-calendar-legend">
    <button
      v-for="source in sources"
      :key="source.id"
      type="button"
      class="rl-calendar-legend__item rl-focus-ring"
      :class="[
        `rl-calendar-legend__item--${source.color ?? 'blue'}`,
        { 'rl-calendar-legend__item--off': isHidden(source.id) },
      ]"
      :aria-pressed="!isHidden(source.id)"
      @click="emit('toggle', source.id)"
    >
      <span class="rl-calendar-legend__swatch" aria-hidden="true" />
      <span class="rl-calendar-legend__label">{{ source.label }}</span>
    </button>
  </div>
</template>

<style scoped>
.rl-calendar-legend {
  display: flex;
  flex-wrap: wrap;
  gap: var(--rl-space-2);
}

.rl-calendar-legend__item {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  min-height: var(--rl-control-height-sm);
  padding: 2px var(--rl-space-3);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-pill);
  background: none;
  color: var(--rl-color-text);
  font-family: inherit;
  font-size: var(--rl-font-size-sm);
  cursor: pointer;
}

.rl-calendar-legend__item:hover {
  background: var(--rl-color-bg-hover);
}

/*
 * A hidden source stays legible rather than fading out: it is the control for
 * bringing itself back, so it has to keep reading as its own name. The
 * strike-through carries the state, not the contrast.
 */
.rl-calendar-legend__item--off {
  color: var(--rl-color-text-muted);
}

.rl-calendar-legend__item--off .rl-calendar-legend__label {
  text-decoration: line-through;
}

/* Hollow when off, so the state survives a greyscale print and does not rest
   on colour alone. */
.rl-calendar-legend__item--off .rl-calendar-legend__swatch {
  background: none;
  border: 1px solid var(--rl-calendar-legend-swatch);
}

.rl-calendar-legend__swatch {
  flex: none;
  width: 10px;
  height: 10px;
  border-radius: 2px;
  background: var(--rl-calendar-legend-swatch);
}

/* The same palette the chips use, so a swatch matches the events it stands
   for. */
.rl-calendar-legend__item--grey {
  --rl-calendar-legend-swatch: var(--rl-tag-grey-fg);
}
.rl-calendar-legend__item--blue {
  --rl-calendar-legend-swatch: var(--rl-tag-blue-fg);
}
.rl-calendar-legend__item--green {
  --rl-calendar-legend-swatch: var(--rl-tag-green-fg);
}
.rl-calendar-legend__item--amber {
  --rl-calendar-legend-swatch: var(--rl-tag-amber-fg);
}
.rl-calendar-legend__item--red {
  --rl-calendar-legend-swatch: var(--rl-tag-red-fg);
}
.rl-calendar-legend__item--purple {
  --rl-calendar-legend-swatch: var(--rl-tag-purple-fg);
}
</style>
