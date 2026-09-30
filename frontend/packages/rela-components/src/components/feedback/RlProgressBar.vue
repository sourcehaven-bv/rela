<script setup lang="ts">
/**
 * Horizontal progress, either one value or a breakdown by status.
 *
 * `segments` covers the rollup case, where a bar shows how many items are
 * done, in progress and blocked at once. Each segment carries a label, so the
 * meaning is available without relying on the colours.
 */
import { computed } from 'vue'
import RlText from '../common/RlText.vue'

export interface ProgressSegment {
  label: string
  value: number
  tone: 'accent' | 'success' | 'warning' | 'danger' | 'neutral'
}

const props = withDefaults(
  defineProps<{
    /** Single-value mode: the amount done, out of `max`. */
    value?: number
    max?: number
    segments?: ProgressSegment[]
    /** Visible label above the bar. */
    label?: string
    /**
     * Accessible name when there is no visible label, such as a bar inside a
     * row that is already identified by the row's own text.
     */
    ariaLabel?: string
    size?: 'sm' | 'md'
    /** Shows the percentage, or the segment counts, beside the bar. */
    showValue?: boolean
  }>(),
  { max: 100, size: 'md', showValue: false },
)

const total = computed(() =>
  props.segments ? props.segments.reduce((sum, segment) => sum + segment.value, 0) : props.max,
)

const percent = computed(() =>
  total.value > 0 ? Math.round(((props.value ?? 0) / total.value) * 100) : 0,
)

/** Percentages, so the segments fill the track whatever the raw counts are. */
const widths = computed(() =>
  (props.segments ?? []).map((segment) => ({
    ...segment,
    percent: total.value > 0 ? (segment.value / total.value) * 100 : 0,
  })),
)

/*
 * A segmented bar reports the whole breakdown in one string. Reading five
 * separate progress bars would be far more tiring than one summary.
 */
const valueText = computed(() =>
  props.segments
    ? widths.value.map((segment) => `${segment.value} ${segment.label}`).join(', ')
    : `${percent.value}%`,
)
</script>

<template>
  <div class="rl-progress">
    <div v-if="label || showValue" class="rl-progress__header">
      <RlText v-if="label" size="sm" tone="muted">{{ label }}</RlText>
      <RlText v-if="showValue" size="sm" tone="muted">{{ valueText }}</RlText>
    </div>

    <div
      class="rl-progress__track"
      :class="`rl-progress__track--${size}`"
      role="progressbar"
      :aria-label="label ?? ariaLabel"
      :aria-valuemin="0"
      :aria-valuemax="segments ? total : max"
      :aria-valuenow="segments ? total : (value ?? 0)"
      :aria-valuetext="valueText"
    >
      <template v-if="segments">
        <span
          v-for="segment in widths"
          :key="segment.label"
          class="rl-progress__segment"
          :class="`rl-progress__segment--${segment.tone}`"
          :style="{ width: `${segment.percent}%` }"
        />
      </template>
      <span v-else class="rl-progress__segment rl-progress__segment--accent" :style="{ width: `${percent}%` }" />
    </div>
  </div>
</template>

<style scoped>
.rl-progress {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-1);
}

.rl-progress__header {
  display: flex;
  justify-content: space-between;
  gap: var(--rl-space-2);
}

.rl-progress__track {
  display: flex;
  overflow: hidden;
  border-radius: var(--rl-radius-pill);
  background: var(--rl-color-bg-active);
}

.rl-progress__track--sm { height: 4px; }
.rl-progress__track--md { height: 8px; }

.rl-progress__segment {
  height: 100%;
  transition: width var(--rl-duration-slow) var(--rl-ease);
}

.rl-progress__segment--accent { background: var(--rl-color-accent); }
.rl-progress__segment--success { background: var(--rl-color-status-green); }
.rl-progress__segment--warning { background: var(--rl-color-status-amber); }
.rl-progress__segment--danger { background: var(--rl-color-danger); }
.rl-progress__segment--neutral { background: var(--rl-color-status-grey); }

@media (prefers-reduced-motion: reduce) {
  .rl-progress__segment { transition: none; }
}
</style>
