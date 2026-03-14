<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { analyze } from '@/api'
import type { AnalyzeResult } from '@/types'
import { useSchemaStore } from '@/stores'

const schemaStore = useSchemaStore()

// State
const loading = ref(true)
const result = ref<AnalyzeResult | null>(null)
const filterSeverity = ref<'all' | 'error' | 'warning'>('all')
const filterCheckType = ref<string>('')

// Computed
const filteredIssues = computed(() => {
  if (!result.value) return []

  return result.value.issues.filter((issue) => {
    if (filterSeverity.value !== 'all' && issue.severity !== filterSeverity.value) {
      return false
    }
    if (filterCheckType.value && issue.checkType !== filterCheckType.value) {
      return false
    }
    return true
  })
})

const checkTypes = computed(() => {
  if (!result.value) return []
  return Object.keys(result.value.byCheck).sort()
})

const issuesByCheck = computed(() => {
  const grouped: Record<string, typeof filteredIssues.value> = {}
  for (const issue of filteredIssues.value) {
    if (!grouped[issue.checkType]) {
      grouped[issue.checkType] = []
    }
    grouped[issue.checkType].push(issue)
  }
  return grouped
})

// Methods
async function loadAnalysis() {
  loading.value = true
  try {
    result.value = await analyze()
  } catch (err) {
    console.error('Analyze error:', err)
  } finally {
    loading.value = false
  }
}

function getEntityTypeLabel(type: string): string {
  const def = schemaStore.entityTypes.get(type)
  return def?.label || type
}

// Lifecycle
onMounted(() => {
  loadAnalysis()
})
</script>

<template>
  <div class="analyze-view">
    <header class="page-header">
      <h1>Analyze</h1>
      <button class="btn btn-secondary" @click="loadAnalysis" :disabled="loading">
        {{ loading ? 'Refreshing...' : 'Refresh' }}
      </button>
    </header>

    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <span>Running analysis...</span>
    </div>

    <template v-else-if="result">
      <!-- Summary -->
      <div class="summary-cards">
        <div class="summary-card" :class="{ 'has-issues': result.errors > 0 }">
          <span class="summary-count">{{ result.errors }}</span>
          <span class="summary-label">{{ result.errors === 1 ? 'Error' : 'Errors' }}</span>
        </div>
        <div class="summary-card warning" :class="{ 'has-issues': result.warnings > 0 }">
          <span class="summary-count">{{ result.warnings }}</span>
          <span class="summary-label">{{ result.warnings === 1 ? 'Warning' : 'Warnings' }}</span>
        </div>
        <div class="summary-card success" v-if="result.errors === 0 && result.warnings === 0">
          <span class="summary-icon">&#10003;</span>
          <span class="summary-label">All checks passed</span>
        </div>
      </div>

      <!-- Filters -->
      <div v-if="result.issues.length > 0" class="filters">
        <div class="filter-group">
          <label>Severity</label>
          <select v-model="filterSeverity">
            <option value="all">All</option>
            <option value="error">Errors only</option>
            <option value="warning">Warnings only</option>
          </select>
        </div>
        <div class="filter-group">
          <label>Check Type</label>
          <select v-model="filterCheckType">
            <option value="">All checks</option>
            <option v-for="check in checkTypes" :key="check" :value="check">
              {{ check }} ({{ result.byCheck[check] }})
            </option>
          </select>
        </div>
      </div>

      <!-- Issues grouped by check type -->
      <div v-if="filteredIssues.length > 0" class="issues-section">
        <div v-for="(issues, checkType) in issuesByCheck" :key="checkType" class="check-group">
          <h3 class="check-title">
            {{ checkType }}
            <span class="check-count">{{ issues.length }}</span>
          </h3>
          <div class="issues-list">
            <router-link
              v-for="issue in issues"
              :key="`${issue.entityType}-${issue.entityId}`"
              :to="`/entity/${issue.entityType}/${issue.entityId}`"
              class="issue-item"
              :class="issue.severity"
            >
              <span class="issue-badge" :class="issue.severity">
                {{ issue.severity === 'error' ? '!' : '?' }}
              </span>
              <span class="issue-entity-type">{{ getEntityTypeLabel(issue.entityType) }}</span>
              <span class="issue-entity-id">{{ issue.entityId }}</span>
              <span class="issue-message">{{ issue.message }}</span>
            </router-link>
          </div>
        </div>
      </div>

      <div v-else-if="result.issues.length > 0" class="no-results">
        No issues match the current filters
      </div>
    </template>
  </div>
</template>

<style scoped>
.analyze-view {
  max-width: 1000px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-header h1 {
  margin: 0;
}

.btn {
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.15s;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-secondary {
  background: #f1f5f9;
  color: #475569;
}

.btn-secondary:hover:not(:disabled) {
  background: #e2e8f0;
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

/* Summary cards */
.summary-cards {
  display: flex;
  gap: 16px;
  margin-bottom: 24px;
}

.summary-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px 32px;
  background: white;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  min-width: 120px;
}

.summary-card.has-issues {
  border-color: #dc2626;
  background: #fef2f2;
}

.summary-card.warning.has-issues {
  border-color: #d97706;
  background: #fffbeb;
}

.summary-card.success {
  border-color: #16a34a;
  background: #f0fdf4;
}

.summary-count {
  font-size: 36px;
  font-weight: 700;
  color: #1e293b;
}

.summary-card.has-issues .summary-count {
  color: #dc2626;
}

.summary-card.warning.has-issues .summary-count {
  color: #d97706;
}

.summary-icon {
  font-size: 36px;
  color: #16a34a;
}

.summary-label {
  font-size: 14px;
  color: #64748b;
  margin-top: 4px;
}

.summary-card.success .summary-label {
  color: #166534;
  font-weight: 500;
}

/* Filters */
.filters {
  display: flex;
  gap: 16px;
  margin-bottom: 24px;
  padding: 16px;
  background: #f8fafc;
  border-radius: 8px;
}

.filter-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.filter-group label {
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
  text-transform: uppercase;
}

.filter-group select {
  padding: 8px 12px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  font-size: 14px;
  min-width: 160px;
}

/* Issues */
.issues-section {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.check-group {
  background: white;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  overflow: hidden;
}

.check-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  padding: 12px 16px;
  background: #f8fafc;
  font-size: 14px;
  font-weight: 600;
  color: #1e293b;
  border-bottom: 1px solid var(--border-color, #e2e8f0);
}

.check-count {
  background: #e2e8f0;
  color: #64748b;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 12px;
}

.issues-list {
  display: flex;
  flex-direction: column;
}

.issue-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  text-decoration: none;
  color: inherit;
  border-bottom: 1px solid var(--border-color, #e2e8f0);
  transition: background 0.15s;
}

.issue-item:last-child {
  border-bottom: none;
}

.issue-item:hover {
  background: #f8fafc;
}

.issue-badge {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  font-size: 14px;
  font-weight: 700;
  flex-shrink: 0;
}

.issue-badge.error {
  background: #fee2e2;
  color: #dc2626;
}

.issue-badge.warning {
  background: #fef3c7;
  color: #d97706;
}

.issue-entity-type {
  font-size: 11px;
  text-transform: uppercase;
  color: #64748b;
  background: #f1f5f9;
  padding: 4px 8px;
  border-radius: 4px;
  font-weight: 500;
}

.issue-entity-id {
  font-family: monospace;
  font-size: 13px;
  color: #64748b;
}

.issue-message {
  flex: 1;
  font-size: 14px;
  color: #1e293b;
}

.no-results {
  padding: 48px 24px;
  text-align: center;
  color: #64748b;
  background: #f8fafc;
  border-radius: 8px;
}
</style>
