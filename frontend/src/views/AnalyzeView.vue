<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { analyze } from '@/api'
import type { AnalyzeResult, AnalyzeIssue } from '@/types'
import { useBackTarget } from '@/composables/useBackTarget'
import BackButton from '@/components/common/BackButton.vue'
import PageLayout from '@/components/common/PageLayout.vue'
import PageTitle from '@/components/common/PageTitle.vue'
import IssuesTable from '@/components/common/IssuesTable.vue'

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

// Whether a check reported fewer issues than it found. The server caps each
// check, so for a truncated one the count above is the cap, not the total —
// which is why the count renders as "100+" and the card says the list is
// partial. Without this an operator fixes all 100, re-runs, sees 100 again,
// and concludes analysis is broken.
function isTruncated(checkKey: string): boolean {
  return result.value?.truncatedChecks?.includes(checkKey) ?? false
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

// Lifecycle
onMounted(() => {
  loadAnalysis()
})
</script>

<template>
  <PageLayout class="analyze-view">
    <template v-if="backTarget" #scope-nav>
      <BackButton :target="backTarget" />
    </template>

    <template #topbar>
      <PageTitle title="Analysis" subtitle="Validation checks across all entities and relations" />
    </template>

    <template #actions>
      <button class="btn btn-secondary" :disabled="loading" @click="loadAnalysis">
        {{ loading ? 'Refreshing...' : 'Refresh' }}
      </button>
    </template>

    <div v-if="loading" class="loading-state">
      <div class="spinner" />
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
        <div v-for="checkType in CHECK_TYPES" :key="checkType.key" class="check-card">
          <div class="check-header">
            <h3 class="check-title">
              {{ checkType.label }}
              <span class="check-count" :class="{ 'has-issues': getCheckCount(checkType.key) > 0 }">
                {{ getCheckCount(checkType.key) }}{{ isTruncated(checkType.key) ? '+' : '' }}
              </span>
            </h3>
            <p class="check-description">{{ checkType.description }}</p>
          </div>

          <p v-if="isTruncated(checkType.key)" class="check-truncated" role="status">
            Showing the first {{ getCheckCount(checkType.key) }} issues &mdash; this check found
            more. Fix these and re-run to see the rest.
          </p>

          <div v-if="getCheckCount(checkType.key) === 0" class="no-issues">
            <span class="check-icon">&#10003;</span>
            No issues
          </div>

          <template v-else>
            <IssuesTable
              v-if="
                shouldShowIssues(checkType.key) &&
                getFilteredIssuesForCheck(checkType.key).length > 0
              "
              :issues="getFilteredIssuesForCheck(checkType.key)"
            />
          </template>
        </div>
      </div>
    </template>
  </PageLayout>
</template>

<style scoped>
.analyze-view {
  max-width: 1000px;
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

.check-truncated {
  margin: 8px 0 0;
  padding: 8px 12px;
  border-left: 3px solid var(--warning-color);
  background: var(--hover-bg);
  border-radius: 4px;
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
</style>
