<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useSchemaStore } from '@/stores'
import { searchEntities, analyze } from '@/api'
import type { Entity, DashboardCard, AnalyzeResult } from '@/types'

const schemaStore = useSchemaStore()

// State
const loading = ref(true)
const cardData = ref<Map<number, { entities: Entity[]; count: number }>>(new Map())
const analysisResult = ref<AnalyzeResult | null>(null)

// Computed
const dashboardConfig = computed(() => schemaStore.dashboard)
const title = computed(() => dashboardConfig.value?.title || 'Dashboard')
const description = computed(() => dashboardConfig.value?.description)
const cards = computed(() => dashboardConfig.value?.cards || [])

// Methods
async function loadData() {
  loading.value = true

  try {
    // Load card data in parallel
    const cardPromises = cards.value.map(async (card, index) => {
      const response = await searchEntities(card.query)
      cardData.value.set(index, {
        entities: response.data,
        count: response.meta.total,
      })
    })

    // Load analysis
    const analysisPromise = analyze()

    await Promise.all([...cardPromises, analysisPromise.then((r) => (analysisResult.value = r))])
  } catch (err) {
    console.error('Dashboard load error:', err)
  } finally {
    loading.value = false
  }
}

function getCardCount(index: number): number {
  return cardData.value.get(index)?.count || 0
}

function getBreakdown(card: DashboardCard, index: number): Array<{ value: string; count: number; percentage: number }> {
  const data = cardData.value.get(index)
  if (!data || !card.group_by) return []

  const groupBy = card.group_by
  const counts: Record<string, number> = {}
  let total = 0

  for (const entity of data.entities) {
    const value = String(entity.properties[groupBy] || 'Unknown')
    counts[value] = (counts[value] || 0) + 1
    total++
  }

  return Object.entries(counts)
    .map(([value, count]) => ({
      value,
      count,
      percentage: total > 0 ? (count / total) * 100 : 0,
    }))
    .sort((a, b) => b.count - a.count)
}

function getTableRows(card: DashboardCard, index: number): Entity[] {
  const data = cardData.value.get(index)
  if (!data) return []

  let entities = [...data.entities]

  // Apply sort
  if (card.sort?.length) {
    const sort = card.sort[0]
    entities.sort((a, b) => {
      const aVal = String(a.properties[sort.property] || '')
      const bVal = String(b.properties[sort.property] || '')
      const cmp = aVal.localeCompare(bVal)
      return sort.direction === 'desc' ? -cmp : cmp
    })
  }

  // Apply limit
  if (card.limit) {
    entities = entities.slice(0, card.limit)
  }

  return entities
}

function getColumnLabel(col: { property?: string; label?: string }): string {
  return col.label || col.property || ''
}

function getCellValue(entity: Entity, col: { property?: string }): string {
  if (!col.property) return ''
  return String(entity.properties[col.property] || '')
}

function getCellLink(entity: Entity, col: { link?: string }): string | undefined {
  if (col.link === 'detail') {
    return `/entity/${entity.type}/${entity.id}`
  }
  return undefined
}

// Lifecycle
onMounted(() => {
  loadData()
})
</script>

<template>
  <div class="dashboard-view">
    <header class="dashboard-header">
      <h1>{{ title }}</h1>
      <p v-if="description" class="description">{{ description }}</p>
    </header>

    <div v-if="loading" class="loading-state">
      <div class="spinner"/>
      <span>Loading dashboard...</span>
    </div>

    <template v-else>
      <div class="dashboard-grid">
        <div
          v-for="(card, index) in cards"
          :key="index"
          class="dashboard-card"
        >
          <div class="card-header">
            <h3>{{ card.title }}</h3>
            <router-link
              :to="`/search?q=${encodeURIComponent(card.query)}`"
              class="card-link"
              title="View in search"
            >
              &#8599;
            </router-link>
          </div>

          <!-- Count display -->
          <div v-if="card.display === 'count'" class="card-count">
            <span class="count-number">{{ getCardCount(index) }}</span>
          </div>

          <!-- Breakdown display -->
          <div v-else-if="card.display === 'breakdown'" class="card-breakdown">
            <div
              v-for="item in getBreakdown(card, index)"
              :key="item.value"
              class="breakdown-row"
            >
              <span class="breakdown-label">{{ item.value }}</span>
              <div class="breakdown-bar-track">
                <div
                  class="breakdown-bar-fill"
                  :style="{ width: `${item.percentage}%` }"
                />
              </div>
              <span class="breakdown-count">{{ item.count }}</span>
            </div>
            <div v-if="getBreakdown(card, index).length === 0" class="no-data">
              No data
            </div>
          </div>

          <!-- Table display -->
          <div v-else-if="card.display === 'table'" class="card-table">
            <table v-if="getTableRows(card, index).length > 0">
              <thead>
                <tr>
                  <th v-for="col in card.columns" :key="col.property">
                    {{ getColumnLabel(col) }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="entity in getTableRows(card, index)" :key="entity.id">
                  <td v-for="col in card.columns" :key="col.property">
                    <router-link
                      v-if="getCellLink(entity, col)"
                      :to="getCellLink(entity, col)!"
                      class="cell-link"
                    >
                      {{ getCellValue(entity, col) }}
                    </router-link>
                    <span v-else>{{ getCellValue(entity, col) }}</span>
                  </td>
                </tr>
              </tbody>
            </table>
            <div v-else class="no-data">No results</div>
          </div>
        </div>
      </div>

      <!-- Validation card -->
      <div class="validation-card">
        <div class="card-header">
          <h3>&#9888; Validation</h3>
          <router-link to="/analyze" class="card-link" title="View full analysis">
            &#8599;
          </router-link>
        </div>
        <div class="validation-content">
          <template v-if="analysisResult">
            <span
              v-if="analysisResult.errors === 0 && analysisResult.warnings === 0"
              class="validation-success"
            >
              &#10003; All checks passed
            </span>
            <template v-else>
              <span v-if="analysisResult.errors > 0" class="badge badge-error">
                {{ analysisResult.errors }} {{ analysisResult.errors === 1 ? 'error' : 'errors' }}
              </span>
              <span v-if="analysisResult.warnings > 0" class="badge badge-warning">
                {{ analysisResult.warnings }} {{ analysisResult.warnings === 1 ? 'warning' : 'warnings' }}
              </span>
              <router-link to="/analyze" class="view-details">
                View details &rarr;
              </router-link>
            </template>
          </template>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.dashboard-view {
  max-width: 1200px;
}

.dashboard-header {
  margin-bottom: 24px;
}

.dashboard-header h1 {
  margin: 0 0 8px;
}

.description {
  color: #64748b;
  margin: 0;
}

.loading-state {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 48px;
  color: #64748b;
}

.spinner {
  width: 24px;
  height: 24px;
  border: 3px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
  margin-bottom: 20px;
}

.dashboard-card {
  background: white;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  padding: 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.card-header h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

.card-link {
  color: #64748b;
  text-decoration: none;
  font-size: 14px;
}

.card-link:hover {
  color: var(--accent-color);
}

/* Count display */
.card-count {
  padding: 16px 0;
}

.count-number {
  font-size: 48px;
  font-weight: 700;
  color: #1e293b;
}

/* Breakdown display */
.card-breakdown {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.breakdown-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.breakdown-label {
  min-width: 80px;
  font-size: 13px;
  color: #64748b;
}

.breakdown-bar-track {
  flex: 1;
  height: 8px;
  background: #f1f5f9;
  border-radius: 4px;
  overflow: hidden;
}

.breakdown-bar-fill {
  height: 100%;
  background: var(--accent-color, #6366f1);
  border-radius: 4px;
  transition: width 0.3s ease;
}

.breakdown-count {
  min-width: 32px;
  text-align: right;
  font-size: 13px;
  font-weight: 600;
  color: #1e293b;
}

/* Table display */
.card-table {
  overflow-x: auto;
}

.card-table table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.card-table th {
  text-align: left;
  padding: 8px;
  border-bottom: 1px solid var(--border-color);
  font-weight: 600;
  color: #64748b;
}

.card-table td {
  padding: 8px;
  border-bottom: 1px solid var(--border-color);
  color: #1e293b;
}

.cell-link {
  color: var(--accent-color);
  text-decoration: none;
}

.cell-link:hover {
  text-decoration: underline;
}

.no-data {
  color: #64748b;
  font-size: 13px;
  padding: 8px 0;
}

/* Validation card */
.validation-card {
  background: white;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  padding: 16px;
}

.validation-content {
  display: flex;
  align-items: center;
  gap: 12px;
}

.validation-success {
  color: #166534;
  font-weight: 600;
  font-size: 14px;
}

.badge {
  font-size: 12px;
  padding: 4px 8px;
  border-radius: 4px;
  font-weight: 500;
}

.badge-error {
  background: #fee2e2;
  color: #dc2626;
}

.badge-warning {
  background: #fef3c7;
  color: #d97706;
}

.view-details {
  margin-left: auto;
  font-size: 13px;
  color: var(--accent-color);
  text-decoration: none;
  font-weight: 500;
}

.view-details:hover {
  text-decoration: underline;
}
</style>
