<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { analyze } from '@/api'
import type { AnalyzeResult, AnalyzeIssue } from '@/types'
import { useSchemaStore } from '@/stores'
import { useScriptErrorStore } from '@/stores/scriptError'
import { useBackTarget } from '@/composables/useBackTarget'
import BackButton from '@/components/common/BackButton.vue'

const router = useRouter()
const schemaStore = useSchemaStore()
const scriptErrorStore = useScriptErrorStore()
const backTarget = useBackTarget()

// Check type definitions with descriptions. Three-way contract:
//   1. `runAnalysis()` in internal/dataentry/analyze.go produces sections
//      with these names, in this order.
//   2. The keys below match those `section.Name` values exactly
//      (`byCheck` is keyed by them).
//   3. `e2e/tests/fixtures.ts` ANALYSIS_CHECKS asserts the same ordered
//      list against the rendered cards.
// `TestRunAnalysisSectionNames` in analyze_test.go pins the Go side so a
// rename can't silently regress GH#785 (hidden cards inflating the badge).
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
  {
    key: 'Duplicates',
    label: 'Duplicates',
    description: 'Entities with identical titles',
  },
  {
    key: 'ID Gaps',
    label: 'ID Gaps',
    description: 'Missing numbers in auto-generated ID sequences',
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

function getEntityTitle(issue: AnalyzeIssue): string {
  return issue.title?.trim() || issue.entityId
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

// An issue is clickable if it has a structured Lua-failure envelope
// (opens ScriptErrorDialog) or a real entity (navigates). LoadError
// rows have neither and stay inert.
function isClickable(issue: AnalyzeIssue): boolean {
  if (issue.scriptError) return true
  return Boolean(issue.entityId && issue.entityType)
}

function onIssueClick(issue: AnalyzeIssue, ev: Event) {
  if (issue.scriptError) {
    const trigger = ev.currentTarget instanceof HTMLElement ? ev.currentTarget : null
    scriptErrorStore.show(issue.scriptError, trigger)
    return
  }
  if (issue.entityId && issue.entityType) {
    router.push(`/entity/${issue.entityType}/${issue.entityId}`)
  }
}

// Lifecycle
onMounted(() => {
  loadAnalysis()
})
</script>

<template>
  <div class="analyze-view">
    <header class="page-header">
      <div class="header-left">
        <BackButton v-if="backTarget" :target="backTarget" />
        <div>
          <h1>Analysis</h1>
          <p class="subtitle">Validation checks across all entities and relations</p>
        </div>
      </div>
      <button class="btn btn-secondary" :disabled="loading" @click="loadAnalysis">
        {{ loading ? 'Refreshing...' : 'Refresh' }}
      </button>
    </header>

    <div v-if="loading" class="loading-state">
      <div class="spinner"/>
      <span>Running analysis...</span>
    </div>

    <template v-else-if="result">
      <!-- Summary badge -->
      <div v-if="result.errors > 0 || result.warnings > 0" class="summary-badge">
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
            <div v-if="shouldShowIssues(checkType.key) && getFilteredIssuesForCheck(checkType.key).length > 0" class="issues-table-wrapper">
            <table class="issues-table">
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
                  v-for="(issue, idx) in getFilteredIssuesForCheck(checkType.key)"
                  :key="`${checkType.key}-${idx}-${issue.entityType}-${issue.entityId}`"
                  class="issue-row"
                  :class="{ clickable: isClickable(issue) }"
                  :tabindex="isClickable(issue) ? 0 : -1"
                  @click="onIssueClick(issue, $event)"
                  @keydown.enter="onIssueClick(issue, $event)"
                  @keydown.space.prevent="onIssueClick(issue, $event)"
                >
                  <td class="entity-cell">
                    <template v-if="issue.entityId">
                      <span class="entity-title">{{ getEntityTitle(issue) }}</span>
                      <span class="entity-id">{{ issue.entityId }}</span>
                    </template>
                    <template v-else>
                      <span class="entity-empty">&mdash;</span>
                    </template>
                  </td>
                  <td>
                    <span v-if="issue.entityType" class="type-badge">{{ getEntityTypeLabel(issue.entityType) }}</span>
                    <span v-else class="entity-empty">&mdash;</span>
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
            </div>
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

.header-left {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.subtitle {
  margin: 0;
  font-size: 14px;
  color: var(--muted-text);
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
  background: var(--hover-bg);
  color: var(--text-color);
}

.btn-secondary:hover:not(:disabled) {
  background: var(--border-color);
}

.loading-state {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 48px;
  color: var(--muted-text);
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
  background: color-mix(in srgb, var(--error-color) 15%, transparent);
  color: var(--error-color);
}

.badge.warning {
  background: color-mix(in srgb, var(--warning-color) 15%, transparent);
  color: var(--warning-color);
}

/* Check cards */
.check-cards {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.check-card {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
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
  color: var(--text-color);
}

.check-count {
  background: var(--border-color);
  color: var(--muted-text);
  padding: 2px 10px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 600;
}

.check-count.has-issues {
  background: color-mix(in srgb, var(--warning-color) 15%, transparent);
  color: var(--warning-color);
}

.check-description {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
}

.no-issues {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px 20px;
  color: var(--success-color);
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
  background: var(--hover-bg);
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--muted-text);
  border-bottom: 1px solid var(--border-color);
}

.issues-table td {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
  font-size: 14px;
}

.issue-row {
  transition: background 0.15s;
}

.issue-row.clickable {
  cursor: pointer;
}

.issue-row.clickable:hover,
.issue-row.clickable:focus-visible {
  background: var(--hover-bg);
  outline: none;
}

.issue-row:last-child td {
  border-bottom: none;
}

/* `display: flex` on the <td> collapses the cell box so its border
 * doesn't span the row height (visible discontinuity between rows).
 * Keep the td as a normal table-cell and stack the two spans with
 * block display + margin instead. */
.entity-title {
  display: block;
  color: var(--accent-color, #6366f1);
  font-weight: 500;
}

.entity-id {
  display: block;
  margin-top: 2px;
  font-family: monospace;
  font-size: 12px;
  color: var(--muted-text);
}

.entity-empty {
  color: var(--muted-text);
  font-size: 14px;
}

.type-badge {
  display: inline-block;
  padding: 4px 8px;
  background: var(--hover-bg);
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--muted-text);
}

.message-cell {
  color: var(--text-color);
  overflow-wrap: anywhere;
  word-break: break-word;
}

.severity-badge {
  display: inline-block;
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
}

.severity-badge.error {
  background: color-mix(in srgb, var(--error-color) 15%, transparent);
  color: var(--error-color);
}

.severity-badge.warning {
  background: color-mix(in srgb, var(--warning-color) 15%, transparent);
  color: var(--warning-color);
}

.issues-table-wrapper {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}

@media (max-width: 768px) {
  .issues-table th,
  .issues-table td {
    padding: 8px 10px;
    font-size: 12px;
  }
}
</style>
