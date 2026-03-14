<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { analyze } from '@/api'
import type { AnalyzeResult, AnalyzeIssue } from '@/types'
import { useSchemaStore } from '@/stores'

const router = useRouter()
const schemaStore = useSchemaStore()

// Check type definitions with descriptions (matching v1)
const CHECK_TYPES = [
  {
    key: 'Properties',
    label: 'Properties',
    description: 'Property validation errors (required fields, invalid values, ID patterns)',
  },
  {
    key: 'Cardinality',
    label: 'Cardinality',
    description: 'Relation cardinality constraint violations',
  },
  {
    key: 'Validations',
    label: 'Validations',
    description: 'Custom validation rules defined in the metamodel',
  },
  {
    key: 'Orphans',
    label: 'Orphans',
    description: 'Entities with no incoming or outgoing relations',
  },
]

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

const issuesByCheck = computed(() => {
  const grouped: Record<string, AnalyzeIssue[]> = {}
  for (const issue of filteredIssues.value) {
    if (!grouped[issue.checkType]) {
      grouped[issue.checkType] = []
    }
    grouped[issue.checkType].push(issue)
  }
  return grouped
})

// Get issue count for a check type
function getCheckCount(checkKey: string): number {
  return result.value?.byCheck[checkKey] || 0
}

// Get filtered issues for a check type
function getFilteredIssuesForCheck(checkKey: string): AnalyzeIssue[] {
  return issuesByCheck.value[checkKey] || []
}

// Should we show issues for this check type based on filters?
function shouldShowIssues(checkKey: string): boolean {
  if (!filterCheckType.value) return true
  return filterCheckType.value === checkKey
}

// Get the entity title from the graph (fallback to ID)
function getEntityTitle(issue: AnalyzeIssue): string {
  // For now, capitalize first letter as title approximation
  // In v1, this comes from the entity properties
  const id = issue.entityId
  return id.charAt(0).toUpperCase() + id.slice(1).replace(/-/g, ' ')
}

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

function navigateToEntity(issue: AnalyzeIssue) {
  router.push(`/entity/${issue.entityType}/${issue.entityId}`)
}

// Lifecycle
onMounted(() => {
  loadAnalysis()
})
</script>

<template>
  <div class="analyze-view">
    <header class="page-header">
      <div>
        <h1>Analysis</h1>
        <p class="subtitle">Validation checks across all entities and relations</p>
      </div>
      <button class="btn btn-secondary" @click="loadAnalysis" :disabled="loading">
        {{ loading ? 'Refreshing...' : 'Refresh' }}
      </button>
    </header>

    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <span>Running analysis...</span>
    </div>

    <template v-else-if="result">
      <!-- Summary badge -->
      <div class="summary-badge" v-if="result.errors > 0 || result.warnings > 0">
        <span v-if="result.errors > 0" class="badge error">
          {{ result.errors }} {{ result.errors === 1 ? 'error' : 'errors' }}
        </span>
        <span v-if="result.warnings > 0" class="badge warning">
          {{ result.warnings }} {{ result.warnings === 1 ? 'warning' : 'warnings' }}
        </span>
      </div>

      <!-- Check type cards -->
      <div class="check-cards">
        <div
          v-for="checkType in CHECK_TYPES"
          :key="checkType.key"
          class="check-card"
        >
          <div class="check-header">
            <h3 class="check-title">
              {{ checkType.label }}
              <span class="check-count" :class="{ 'has-issues': getCheckCount(checkType.key) > 0 }">
                {{ getCheckCount(checkType.key) }}
              </span>
            </h3>
            <p class="check-description">{{ checkType.description }}</p>
          </div>

          <div v-if="getCheckCount(checkType.key) === 0" class="no-issues">
            <span class="check-icon">&#10003;</span>
            No issues
          </div>

          <template v-else>
            <table class="issues-table" v-if="shouldShowIssues(checkType.key) && getFilteredIssuesForCheck(checkType.key).length > 0">
              <thead>
                <tr>
                  <th>Entity</th>
                  <th>Type</th>
                  <th>Message</th>
                  <th>Severity</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="issue in getFilteredIssuesForCheck(checkType.key)"
                  :key="`${issue.entityType}-${issue.entityId}`"
                  class="issue-row"
                  @click="navigateToEntity(issue)"
                >
                  <td class="entity-cell">
                    <span class="entity-title">{{ getEntityTitle(issue) }}</span>
                    <span class="entity-id">{{ issue.entityId }}</span>
                  </td>
                  <td>
                    <span class="type-badge">{{ getEntityTypeLabel(issue.entityType) }}</span>
                  </td>
                  <td class="message-cell">{{ issue.message }}</td>
                  <td>
                    <span class="severity-badge" :class="issue.severity">
                      {{ issue.severity.toUpperCase() }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </template>
        </div>
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
  align-items: flex-start;
  margin-bottom: 24px;
}

.page-header h1 {
  margin: 0 0 4px;
}

.subtitle {
  margin: 0;
  font-size: 14px;
  color: #64748b;
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

/* Summary badge */
.summary-badge {
  display: flex;
  gap: 8px;
  margin-bottom: 24px;
}

.badge {
  display: inline-flex;
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 600;
}

.badge.error {
  background: #fef2f2;
  color: #dc2626;
}

.badge.warning {
  background: #fffbeb;
  color: #d97706;
}

/* Check cards */
.check-cards {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.check-card {
  background: white;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  overflow: hidden;
}

.check-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color, #e2e8f0);
}

.check-title {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0 0 4px;
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
}

.check-count {
  background: #e2e8f0;
  color: #64748b;
  padding: 2px 10px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 600;
}

.check-count.has-issues {
  background: #fef3c7;
  color: #d97706;
}

.check-description {
  margin: 0;
  font-size: 13px;
  color: #64748b;
}

.no-issues {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px 20px;
  color: #16a34a;
  font-size: 14px;
}

.check-icon {
  font-size: 16px;
}

/* Issues table */
.issues-table {
  width: 100%;
  border-collapse: collapse;
}

.issues-table th {
  text-align: left;
  padding: 10px 16px;
  background: #f8fafc;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: #64748b;
  border-bottom: 1px solid var(--border-color, #e2e8f0);
}

.issues-table td {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color, #e2e8f0);
  font-size: 14px;
}

.issue-row {
  cursor: pointer;
  transition: background 0.15s;
}

.issue-row:hover {
  background: #f8fafc;
}

.issue-row:last-child td {
  border-bottom: none;
}

.entity-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.entity-title {
  color: var(--accent-color, #6366f1);
  font-weight: 500;
}

.entity-id {
  font-family: monospace;
  font-size: 12px;
  color: #94a3b8;
}

.type-badge {
  display: inline-block;
  padding: 4px 8px;
  background: #f1f5f9;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  color: #64748b;
}

.message-cell {
  color: #475569;
}

.severity-badge {
  display: inline-block;
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
}

.severity-badge.error {
  background: #fef2f2;
  color: #dc2626;
}

.severity-badge.warning {
  background: #fef3c7;
  color: #d97706;
}
</style>
