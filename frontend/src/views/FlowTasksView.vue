<script setup lang="ts">
/**
 * My flow tasks (TKT-ZAXRXND): the runs that wait for a role the reader
 * holds on their subject, paged on the server. Each row links to the run,
 * where the step form is, and to the subject.
 */
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlPagination from 'rela-components/components/data/RlPagination.vue'
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'
import PageLayout from '@/components/common/PageLayout.vue'
import PageTitle from '@/components/common/PageTitle.vue'
import { listFlowTasks, type FlowTaskPage } from '@/api/flows'
import { getErrorMessage, shouldDropHeldContent } from '@/api/errors'
import { isCancelledFetch } from '@/composables/usePageData'
import { useEvents } from '@/composables/useEvents'
import { useFlowList } from '@/composables/useFlowList'
import { useFlowListQuery } from '@/composables/useFlowListQuery'
import { useSpaceStore } from '@/stores/space'
import { formatInstant, runHref, subjectHref } from '@/components/flows/flowView'

const PER_PAGE = 25

const { t } = useI18n()
const space = useSpaceStore()
const { flows, flowNamed } = useFlowList()
const { on } = useEvents()
const { param, page, setFilter, setPage, pageCount } = useFlowListQuery(PER_PAGE)

const flowFilter = param('flow')

const result = ref<FlowTaskPage | null>(null)
const error = ref('')
let controller: AbortController | null = null

const flowOptions = computed(() => [
  { value: '', label: t('flows.list.allFlows') },
  ...flows.value.map((f) => ({ value: f.name, label: f.label || f.name })),
])

async function load() {
  controller?.abort()
  const ctl = new AbortController()
  controller = ctl
  try {
    result.value = await listFlowTasks(
      { flow: flowFilter.value, page: page.value, perPage: PER_PAGE },
      ctl.signal
    )
    error.value = ''
  } catch (err) {
    if (isCancelledFetch(err)) return
    if (shouldDropHeldContent(err)) result.value = null
    error.value = getErrorMessage(err, t('flows.tasks.loadFailed'))
  }
}

watch(() => [flowFilter.value, page.value], load, { immediate: true })
on('entity:changed', (data) => {
  if (data?.type === '_flow-run') void load()
})

function subjectLink(flow: string, subject: string): string {
  const href = subjectHref(subject, flowNamed(flow)?.subject)
  return href ? space.href(href) : ''
}
</script>

<template>
  <PageLayout :data-testid="result ? 'flow-tasks-loaded' : 'flow-tasks-loading'">
    <template #topbar>
      <PageTitle :title="t('flows.tasks.title')" />
    </template>

    <div class="flow-list__filters">
      <RlSelect
        :model-value="flowFilter"
        :label="t('flows.list.flow')"
        :options="flowOptions"
        size="sm"
        @update:model-value="setFilter('flow', $event)"
      />
    </div>

    <RlCallout v-if="error" tone="danger" class="flow-list__error">{{ error }}</RlCallout>

    <template v-if="result">
      <p v-if="result.tasks.length === 0" class="flow-list__empty">{{ t('flows.tasks.empty') }}</p>
      <table v-else class="flow-list__table">
        <thead>
          <tr>
            <th scope="col">{{ t('flows.list.flow') }}</th>
            <th scope="col">{{ t('flows.list.state') }}</th>
            <th scope="col">{{ t('flows.list.subject') }}</th>
            <th scope="col">{{ t('flows.list.started') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="task in result.tasks"
            :key="`${task.run}:${task.branch ?? ''}`"
            data-testid="flow-task-row"
          >
            <td>
              <RouterLink :to="space.href(runHref(task.run))">
                {{ task.flow_label || task.flow }}
              </RouterLink>
            </td>
            <td>
              {{
                task.branch
                  ? t('flows.tasks.branchState', {
                      state: task.state_label || task.state,
                      branch: task.branch,
                    })
                  : task.state_label || task.state
              }}
            </td>
            <td>
              <RouterLink
                v-if="subjectLink(task.flow, task.subject)"
                :to="subjectLink(task.flow, task.subject)"
              >
                {{ task.subject }}
              </RouterLink>
              <span v-else>{{ task.subject }}</span>
            </td>
            <td>{{ formatInstant(task.started) }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="result.truncated" class="flow-list__muted">{{ t('flows.tasks.truncated') }}</p>
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
