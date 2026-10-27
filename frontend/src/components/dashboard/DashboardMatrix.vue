<script setup lang="ts">
import { computed } from 'vue'
import type { DashboardCell, DashboardMatrix } from '@/types'
import { styleFill } from '@/utils/styleColors'

/**
 * A matrix card (TKT-2SGPJZ): rows by columns, each non-empty cell a link to
 * the card's list filtered on both values. A cell takes the colour the server
 * resolved from `tone_by`; without one it is shaded by its count.
 */
const props = defineProps<{
  matrix: DashboardMatrix
  /** The card's list, which a cell opens. */
  list: string
  /** The card title, for the grid's accessible name. */
  title: string
  /** True when the card was computed with `tone_by`. */
  toned: boolean
  /** The world the card was counted in, if not the default. */
  world?: string
}>()

const cellsByKey = computed(() => {
  const out = new Map<string, DashboardCell>()
  for (const cell of props.matrix.cells) out.set(JSON.stringify([cell.row, cell.column]), cell)
  return out
})

const maxCount = computed(() => Math.max(1, ...props.matrix.cells.map((c) => c.count)))

function cellAt(row: string, column: string): DashboardCell | undefined {
  return cellsByKey.value.get(JSON.stringify([row, column]))
}

function cellStyle(cell: DashboardCell): Record<string, string> | undefined {
  if (cell.color) return styleFill(cell.color)
  if (props.toned) return undefined
  // Shade by count: the accent, mixed in proportion to the busiest cell. The
  // stylesheet does the mixing; the cell only carries its share.
  const pct = Math.round(15 + (cell.count / maxCount.value) * 45)
  return { '--cell-shade': `${pct}%` }
}

function cellLink(row: string, column: string) {
  return {
    path: `/list/${encodeURIComponent(props.list)}`,
    query: {
      [`filter[${props.matrix.row_property}]`]: row,
      [`filter[${props.matrix.column_property}]`]: column,
      // The list opens in the world the card was counted in.
      ...(props.world ? { world: props.world } : {}),
    },
  }
}
</script>

<template>
  <div class="dashboard-matrix">
    <table :aria-label="title">
      <thead>
        <tr>
          <td />
          <th v-for="col in matrix.columns" :key="col.value" scope="col">{{ col.label }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in matrix.rows" :key="row.value">
          <th scope="row">{{ row.label }}</th>
          <td v-for="col in matrix.columns" :key="col.value">
            <router-link
              v-if="cellAt(row.value, col.value)"
              :to="cellLink(row.value, col.value)"
              class="matrix-cell"
              :class="{ 'matrix-cell--shaded': !toned }"
              :style="cellStyle(cellAt(row.value, col.value)!)"
              :aria-label="`${row.label}, ${col.label}: ${cellAt(row.value, col.value)!.count}`"
              data-testid="matrix-cell"
            >
              {{ cellAt(row.value, col.value)!.count }}
            </router-link>
            <span v-else class="matrix-cell matrix-cell--empty" aria-hidden="true" />
          </td>
        </tr>
      </tbody>
    </table>
    <p v-if="matrix.unplaced" class="matrix-note">
      {{ matrix.unplaced }} without a {{ matrix.row_property }} or {{ matrix.column_property }}
    </p>
    <p v-if="matrix.truncated" class="matrix-note">Not every value is shown.</p>
  </div>
</template>

<style scoped>
.dashboard-matrix {
  overflow-x: auto;
}

table {
  border-collapse: separate;
  border-spacing: 2px;
  font-size: var(--font-size-sm);
}

th {
  font-weight: 500;
  color: var(--rl-color-text-muted);
  padding: 2px var(--space-xs);
  text-align: center;
}

th[scope='row'] {
  text-align: right;
}

.matrix-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 28px;
  border-radius: var(--radius-sm);
  color: var(--rl-color-text);
  font-weight: 600;
  text-decoration: none;
  border: 1px solid var(--rl-color-border);
}

.matrix-cell--shaded {
  background: color-mix(in srgb, var(--rl-color-accent) var(--cell-shade, 0%), transparent);
}

.matrix-cell--empty {
  border-style: dashed;
  opacity: 0.5;
}

a.matrix-cell:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 2px var(--rl-color-bg),
    0 0 0 4px var(--rl-color-focus);
}

.matrix-note {
  margin: var(--space-xs) 0 0;
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}
</style>
