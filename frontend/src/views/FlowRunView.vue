<script setup lang="ts">
/**
 * One flow run on its own page (TKT-ZAXRXND), reached from the run list,
 * the task list or a run card's title. A run the reader cannot see answers
 * 404 like a hidden entity, and the page says it is not found.
 */
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'
import PageLayout from '@/components/common/PageLayout.vue'
import PageTitle from '@/components/common/PageTitle.vue'
import FlowRunCard from '@/components/flows/FlowRunCard.vue'
import { getFlowRun, type FlowRun } from '@/api/flows'
import { ApiError, getErrorMessage, shouldDropHeldContent } from '@/api/errors'
import { isCancelledFetch } from '@/composables/usePageData'
import { useEvents } from '@/composables/useEvents'
import { useFlowList } from '@/composables/useFlowList'
import { useSpaceStore } from '@/stores/space'
import { subjectHref } from '@/components/flows/flowView'

const props = defineProps<{ id: string }>()

const { t } = useI18n()
const space = useSpaceStore()
const { flowNamed } = useFlowList()
const { on } = useEvents()

const run = ref<FlowRun | null>(null)
const error = ref('')
let controller: AbortController | null = null

const subjectType = computed(() => (run.value ? (flowNamed(run.value.flow)?.subject ?? '') : ''))
const subjectLink = computed(() => {
  const href = subjectHref(run.value?.subject, subjectType.value)
  return href ? space.href(href) : ''
})

async function load() {
  controller?.abort()
  const ctl = new AbortController()
  controller = ctl
  try {
    run.value = await getFlowRun(props.id, ctl.signal)
    error.value = ''
  } catch (err) {
    if (isCancelledFetch(err)) return
    if (shouldDropHeldContent(err)) run.value = null
    error.value =
      err instanceof ApiError && err.status === 404
        ? t('flows.run.notFound')
        : getErrorMessage(err, t('flows.run.loadFailed'))
  }
}

watch(
  () => props.id,
  () => {
    run.value = null
    void load()
  },
  { immediate: true }
)

on('entity:changed', (data) => {
  if (data?.type === '_flow-run') void load()
})
</script>

<template>
  <PageLayout :data-testid="run ? 'flow-run-loaded' : 'flow-run-loading'">
    <template #topbar>
      <PageTitle :title="run ? run.flow_label || run.flow : t('flows.run.title')" />
    </template>

    <RlCallout v-if="error" tone="danger" class="flow-run-view__error">{{ error }}</RlCallout>

    <template v-if="run">
      <p class="flow-run-view__subject">
        <RouterLink v-if="subjectLink" :to="subjectLink" data-testid="flow-run-subject">
          {{ t('flows.run.openSubject', { subject: run.subject }) }}
        </RouterLink>
        <span v-else>{{ t('flows.list.hiddenSubject') }}</span>
      </p>
      <FlowRunCard :run="run" :subject-type="subjectType" @changed="load" @gone="load" />
    </template>
  </PageLayout>
</template>

<style scoped>
.flow-run-view__error {
  margin-bottom: var(--space-lg);
}

.flow-run-view__subject {
  margin: 0 0 var(--space-md);
}
</style>
