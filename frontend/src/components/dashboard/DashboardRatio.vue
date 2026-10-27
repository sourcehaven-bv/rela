<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { DashboardGroup } from '@/types'

/**
 * A ratio card (TKT-2SGPJZ): how many of the list's rows its `match:` scope
 * admits, as `matched / total (pct)`, overall and per group.
 */
defineProps<{
  matched: number
  total: number
  groups?: DashboardGroup[]
}>()

const { t } = useI18n()

function pct(matched: number, total: number): string {
  return total > 0 ? `${Math.round((matched / total) * 100)}%` : '–'
}
</script>

<template>
  <div class="dashboard-ratio">
    <p class="ratio-total" data-testid="ratio-total">
      <span class="ratio-number">{{ matched }} / {{ total }}</span
      >{{ ' ' }}<span class="ratio-pct">({{ pct(matched, total) }})</span>
    </p>
    <ul v-if="groups?.length" class="ratio-groups">
      <li v-for="g in groups" :key="g.value" data-testid="ratio-group">
        <span class="ratio-label">{{ g.label || t('dashboard.unknown') }}</span>
        <span class="ratio-bar-track" aria-hidden="true">
          <span
            class="ratio-bar-fill"
            :style="{ width: g.count > 0 ? `${(g.matched / g.count) * 100}%` : '0%' }"
          />
        </span>
        <span class="ratio-value"
          >{{ g.matched }} / {{ g.count }} ({{ pct(g.matched, g.count) }})</span
        >
      </li>
    </ul>
  </div>
</template>

<style scoped>
.ratio-total {
  margin: 0 0 var(--space-sm);
}

.ratio-number {
  font-size: 24px;
  font-weight: 600;
}

.ratio-pct {
  color: var(--rl-color-text-muted);
}

.ratio-groups {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: var(--space-xs);
}

.ratio-groups li {
  display: grid;
  grid-template-columns: minmax(60px, 1fr) 2fr auto;
  align-items: center;
  gap: var(--space-sm);
  font-size: var(--font-size-sm);
}

.ratio-bar-track {
  height: 8px;
  background: var(--rl-color-bg-sunken);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.ratio-bar-fill {
  display: block;
  height: 100%;
  background: var(--rl-color-accent);
}

.ratio-value {
  color: var(--rl-color-text-muted);
  white-space: nowrap;
}
</style>
