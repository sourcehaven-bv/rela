<script setup lang="ts">
/**
 * The run list (TKT-ZAXRXND): every run the reader can see, filtered by flow
 * and status and paged on the server. The filters and the page live in the
 * query string, so a link keeps them. Each row links to the run and to its
 * subject, when the reader may read the subject.
 */
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlPagination from 'rela-components/components/data/RlPagination.vue'
import RlStatusPill from 'rela-components/components/common/RlStatusPill.vue'
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'
import PageLayout from '@/components/common/PageLayout.vue'
import PageTitle from '@/components/common/PageTitle.vue'
import { FLOW_RUN_STATUSES, listFlowRuns, type FlowRunPage } from '@/api/flows'
import { getErrorMessage, shouldDropHeldContent } from '@/api/errors'
import { isCancelledFetch } from '@/composables/usePageData'
import { useEvents } from '@/composables/useEvents'
import { useFlowList } from '@/composables/useFlowList'
import { useFlowListQuery } from '@/composables/useFlowListQuery'
import { useSpaceStore } from '@/stores/space'
import {
  formatInstant,
  runHref,
  statusLabel,
  statusTone,
  subjectHref,
} from '@/components/flows/flowView'

const PER_PAGE = 25

const { t } = useI18n()
const space = useSpaceStore()
const { flows, flowNamed } = useFlowList()
const { on } = useEvents()
const { param, page, setFilter, setPage, pageCount } = useFlowListQuery(PER_PAGE)

const flowFilter = param('flow')
const statusFilter = param('status')

const result = ref<FlowRunPage | null>(null)
const error = ref('')
let controller: AbortController | null = null

const flowOptions = computed(() => [
  { value: '', label: t('flows.list.allFlows') },
  ...flows.value.map((f) => ({ value: f.name, label: f.label || f.name })),
])
const statusOptions = computed(() => [
  { value: '', label: t('flows.list.allStatuses') },
  ...FLOW_RUN_STATUSES.map((s) => ({ value: s, label: statusLabel(s, t) })),
])

async function load() {
  controller?.abort()
  const ctl = new AbortController()
  controller = ctl
  try {
    result.value = await listFlowRuns(
      { flow: flowFilter.value, status: statusFilter.value, page: page.value, perPage: PER_PAGE },
      ctl.signal
    )
    error.value = ''
  } catch (err) {
    if (isCancelledFetch(err)) return
    if (shouldDropHeldContent(err)) result.value = null
    error.value = getErrorMessage(err, t('flows.list.loadFailed'))
  }
}

watch(() => [flowFilter.value, statusFilter.value, page.value], load, { immediate: true })
on('entity:changed', (data) => {
  if (data?.type === '_flow-run') void load()
})

function subjectLink(flow: string, subject?: string): string {
  const href = subjectHref(subject, flowNamed(flow)?.subject)
  return href ? space.href(href) : ''
}
</script>

<template>
  <PageLayout :data-testid="result ? 'flow-runs-loaded' : 'flow-runs-loading'">
    <template #topbar>
      <PageTitle :title="t('flows.list.title')" />
    </template>

    <div class="flow-list__filters">
      <RlSelect
        :model-value="flowFilter"
        :label="t('flows.list.flow')"
        :options="flowOptions"
        size="sm"
        @update:model-value="setFilter('flow', $event)"
      />
      <RlSelect
        :model-value="statusFilter"
        :label="t('flows.list.status')"
        :options="statusOptions"
        size="sm"
        @update:model-value="setFilter('status', $event)"
      />
    </div>

    <RlCallout v-if="error" tone="danger" class="flow-list__error">{{ error }}</RlCallout>

    <template v-if="result">
      <p v-if="result.runs.length === 0" class="flow-list__empty">{{ t('flows.list.empty') }}</p>
      <table v-else class="flow-list__table">
        <thead>
          <tr>
            <th scope="col">{{ t('flows.list.flow') }}</th>
            <th scope="col">{{ t('flows.list.state') }}</th>
            <th scope="col">{{ t('flows.list.status') }}</th>
            <th scope="col">{{ t('flows.list.subject') }}</th>
            <th scope="col">{{ t('flows.list.started') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in result.runs" :key="r.id" data-testid="flow-run-row">
            <td>
              <RouterLink :to="space.href(runHref(r.id))">{{ r.flow_label || r.flow }}</RouterLink>
            </td>
            <td>{{ r.state_label || r.state }}</td>
            <td>
              <RlStatusPill :label="statusLabel(r.status, t)" :color="statusTone(r.status)" />
            </td>
            <td>
              <RouterLink
                v-if="subjectLink(r.flow, r.subject)"
                :to="subjectLink(r.flow, r.subject)"
              >
                {{ r.subject }}
              </RouterLink>
              <span v-else class="flow-list__muted">{{ t('flows.list.hiddenSubject') }}</span>
            </td>
            <td>{{ formatInstant(r.started) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="result.truncated" class="flow-list__muted">{{ t('flows.list.truncated') }}</p>
      <RlPagination
        v-if="pageCount(result.total) > 1"
        :page="page"
        :page-count="pageCount(result.total)"
        :summary="t('flows.list.summary', { n: result.total })"
        @update:page="setPage"
      />
    </template>
  </PageLayout>
</template>

<style scoped src="../components/flows/flowList.css"></style>
