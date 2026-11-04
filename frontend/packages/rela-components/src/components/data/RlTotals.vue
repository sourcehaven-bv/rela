<script setup lang="ts">
/**
 * Totals under a set of rows: a subtotal, the tax on it and the amount due.
 *
 * Values arrive formatted. Money and quantities are formatted by the app,
 * which knows the currency, the locale and the precision; this only lines
 * them up. The figures are tabular and right-aligned, so the digits stand
 * under one another and under the column they sum.
 */
import type { TotalRow } from './types'

defineProps<{
  rows: TotalRow[]
  /** Names the list for assistive tech, such as "Quote totals". */
  label?: string
}>()
</script>

<template>
  <dl class="rl-totals" :aria-label="label">
    <div
      v-for="row in rows"
      :key="row.label"
      class="rl-totals__row"
      :class="{ 'rl-totals__row--emphasis': row.emphasis }"
    >
      <dt class="rl-totals__label">{{ row.label }}</dt>
      <dd class="rl-totals__value">{{ row.value }}</dd>
    </div>
  </dl>
</template>

<style scoped>
.rl-totals {
  display: grid;
  grid-template-columns: auto auto;
  justify-content: end;
  gap: var(--rl-space-1) var(--rl-space-4);
  margin: 0;
  font-size: var(--rl-font-size-sm);
  font-variant-numeric: tabular-nums;
  color: var(--rl-color-text-muted);
}

/* Each pair is a grid row of its own parent's grid, so the labels and the
   values each share one column edge. */
.rl-totals__row { display: contents; }

.rl-totals__label,
.rl-totals__value {
  margin: 0;
  text-align: right;
}

.rl-totals__row--emphasis {
  font-weight: var(--rl-font-weight-semibold);
  color: var(--rl-color-text);
}
</style>
