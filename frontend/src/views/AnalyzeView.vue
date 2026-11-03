<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { analyze } from '@/api'
import type { AnalyzeResult, AnalyzeIssue } from '@/types'
import { useBackTarget } from '@/composables/useBackTarget'
import BackButton from '@/components/common/BackButton.vue'
import PageLayout from '@/components/common/PageLayout.vue'
import PageTitle from '@/components/common/PageTitle.vue'
import IssuesTable from '@/components/common/IssuesTable.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlStatusRegion from 'rela-components/components/feedback/RlStatusRegion.vue'

const { t } = useI18n()
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
// `label` and `description` are message keys, written literally so the
// unused-key check can see them.
const CHECK_TYPES = [
  {
    key: 'Properties',
    label: 'analyze.check.properties.label',
    description: 'analyze.check.properties.description',
  },
  {
    key: 'Cardinality',
    label: 'analyze.check.cardinality.label',
    description: 'analyze.check.cardinality.description',
  },
  {
    key: 'Validations',
    label: 'analyze.check.validations.label',
    description: 'analyze.check.validations.description',
  },
  {
    key: 'Orphans',
    label: 'analyze.check.orphans.label',
    description: 'analyze.check.orphans.description',
  },
  {
    key: 'Duplicates',
    label: 'analyze.check.duplicates.label',
    description: 'analyze.check.duplicates.description',
  },
  {
    key: 'ID Gaps',
    label: 'analyze.check.idGaps.label',
    description: 'analyze.check.idGaps.description',
  },
] as const

// State
const loading = ref(true)

// pageState mirrors DynamicForm's `form-state-*` contract: a stable signal
// that this screen has finished resolving, so a screenshot{} capture can wait
// for it rather than hanging until its timeout.
const loadError = ref(false)
const pageState = computed<'pending' | 'loaded' | 'error'>(() => {
  if (loadError.value) return 'error'
  return loading.value ? 'pending' : 'loaded'
})
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
  loadError.value = false
  try {
    result.value = await analyze()
  } catch (err) {
    console.error('Analyze error:', err)
    loadError.value = true
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
  <PageLayout class="analyze-view" :data-testid="`page-state-${pageState}`">
    <template v-if="backTarget" #scope-nav>
      <BackButton :target="backTarget" />
    </template>

    <template #topbar>
      <PageTitle :title="t('analyze.title')" :subtitle="t('analyze.subtitle')" />
    </template>

    <template #actions>
      <RlButton
        variant="secondary"
        :loading="loading"
        :pending-label="t('analyze.refreshing')"
        @click="loadAnalysis"
      >
        {{ t('analyze.refresh') }}
      </RlButton>
    </template>

    <RlStatusRegion v-if="loading" :pending-label="t('analyze.analysing')">
      {{ t('analyze.running') }}
    </RlStatusRegion>

    <template v-else-if="result">
      <!-- Summary badge -->
      <div v-if="result.errors > 0 || result.warnings > 0" class="summary-badge">
        <span v-if="result.errors > 0" class="badge error">
          {{ t('analyze.errorCount', { n: result.errors }, result.errors) }}
        </span>
        <span v-if="result.warnings > 0" class="badge warning">
          {{ t('analyze.warningCount', { n: result.warnings }, result.warnings) }}
        </span>
      </div>

      <!-- Check type cards -->
      <div class="check-cards">
        <section
          v-for="checkType in CHECK_TYPES"
          :key="checkType.key"
          class="check-card"
          :aria-labelledby="`check-${checkType.key}-title`"
        >
          <div class="check-header">
            <h3 :id="`check-${checkType.key}-title`" class="check-title">
              {{ t(checkType.label) }}
              <span class="check-count" :class="{ 'has-issues': getCheckCount(checkType.key) > 0 }">
                {{ getCheckCount(checkType.key) }}{{ isTruncated(checkType.key) ? '+' : '' }}
              </span>
            </h3>
            <p class="check-description">{{ t(checkType.description) }}</p>
          </div>

          <p v-if="isTruncated(checkType.key)" class="check-truncated" role="status">
            {{ t('analyze.truncated', { n: getCheckCount(checkType.key) }) }}
          </p>

          <div v-if="getCheckCount(checkType.key) === 0" class="no-issues">
            <span class="check-icon">&#10003;</span>
            {{ t('analyze.noIssues') }}
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
        </section>
      </div>
    </template>
  </PageLayout>
</template>

<style scoped>
.analyze-view {
  max-width: 1000px;
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
  background: color-mix(in srgb, var(--rl-color-danger) 15%, transparent);
  color: var(--rl-color-danger);
}

.badge.warning {
  background: color-mix(in srgb, var(--rl-color-status-amber) 15%, transparent);
  color: var(--rl-color-status-amber);
}

/* Check cards */
.check-cards {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.check-card {
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: 8px;
  overflow: hidden;
}

.check-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--rl-color-border, #e2e8f0);
}

.check-title {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0 0 4px;
  font-size: 16px;
  font-weight: 600;
  color: var(--rl-color-text);
}

.check-count {
  background: var(--rl-color-border);
  color: var(--rl-color-text-muted);
  padding: 2px 10px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 600;
}

.check-count.has-issues {
  background: color-mix(in srgb, var(--rl-color-status-amber) 15%, transparent);
  color: var(--rl-color-status-amber);
}

.check-description {
  margin: 0;
  font-size: 13px;
  color: var(--rl-color-text-muted);
}

.check-truncated {
  margin: 8px 0 0;
  padding: 8px 12px;
  border-left: 3px solid var(--rl-color-status-amber);
  background: var(--rl-color-bg-hover);
  border-radius: 4px;
  font-size: 13px;
  color: var(--rl-color-text-muted);
}

.no-issues {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px 20px;
  color: var(--rl-color-status-green);
  font-size: 14px;
}

.check-icon {
  font-size: 16px;
}
</style>
