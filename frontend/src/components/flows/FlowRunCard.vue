<script setup lang="ts">
/**
 * One flow run (TKT-ZAXRXND): its state and status, why it waits, the step
 * form, the diagram and the timeline, and the controls `_actions` allows.
 *
 * The run shown is the prop until the card reads the run itself: after a
 * step, a cancel or a retry, and on mount for an active run the prop holds
 * without its timeline. It then emits `changed`, so the host can refresh
 * the list it came from.
 */
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlStatusPill from 'rela-components/components/common/RlStatusPill.vue'
import RlTimeline from 'rela-components/components/feedback/RlTimeline.vue'
import {
  cancelFlowRun,
  getFlowRun,
  retryFlowRun,
  type FlowCompleted,
  type FlowRun,
} from '@/api/flows'
import { getErrorMessage, shouldDropHeldContent } from '@/api/errors'
import { isCancelledFetch } from '@/composables/usePageData'
import { useConfirm } from '@/composables/useConfirm'
import { useUIStore } from '@/stores'
import { useSpaceStore } from '@/stores/space'
import FlowWaiting from './FlowWaiting.vue'
import FlowStepForm from './FlowStepForm.vue'
import FlowDiagram from './FlowDiagram.vue'
import { formatInstant, runHref, statusLabel, statusTone, timelineEntries } from './flowView'

const props = withDefaults(
  defineProps<{
    run: FlowRun
    /** The flow's subject type; links and the step form need it. */
    subjectType?: string
    /** Link the title to the run's own page. */
    linkTitle?: boolean
  }>(),
  { subjectType: '', linkTitle: false }
)

const emit = defineEmits<{
  /** The run changed through this card. */
  changed: []
  /** The run is no longer readable. */
  gone: []
}>()

const { t } = useI18n()
const ui = useUIStore()
const space = useSpaceStore()
const { confirm } = useConfirm()

const detail = ref<FlowRun | null>(null)
const busy = ref<'' | 'cancel' | 'retry'>('')
let controller: AbortController | null = null

/** The full run when it has loaded for this id, else the prop. */
const shown = computed<FlowRun>(() =>
  detail.value && detail.value.id === props.run.id ? detail.value : props.run
)

async function reload() {
  controller?.abort()
  const ctl = new AbortController()
  controller = ctl
  try {
    detail.value = await getFlowRun(props.run.id, ctl.signal)
  } catch (err) {
    if (isCancelledFetch(err)) return
    if (shouldDropHeldContent(err)) {
      detail.value = null
      emit('gone')
      return
    }
    ui.error(getErrorMessage(err, t('flows.run.loadFailed')))
  }
}

// The prop changes when the host reads its list again; follow it. A list
// row without a timeline is a summary, so the card reads the detail; the
// subject's list already sends it, and a finished run has none to read.
watch(
  () => [props.run.id, props.run.generation, props.run.status],
  () => {
    detail.value = null
    if (props.run.active && props.run.log === undefined) void reload()
  },
  { immediate: true }
)

const timeline = computed(() => timelineEntries(shown.value, t))
const waiting = computed(() => (shown.value.active ? shown.value.waiting : undefined))
const showStep = computed(
  () => !!waiting.value && shown.value._actions.complete && !!props.subjectType
)

// The notice for each Completed.problem, keyed literally so the i18n guard
// sees every message in use.
const problemKeys: Record<NonNullable<FlowCompleted['problem']>, string> = {
  guard: 'flows.step.problem.guard',
  moved: 'flows.step.problem.moved',
  settling: 'flows.step.problem.settling',
}

async function onCompleted(result: FlowCompleted) {
  if (result.problem) ui.info(t(problemKeys[result.problem]))
  else ui.success(result.finished ? t('flows.step.finished') : t('flows.step.done'))
  await reload()
  emit('changed')
}

async function onStale() {
  await reload()
  emit('changed')
}

async function cancel() {
  const ok = await confirm({
    title: t('flows.run.cancelTitle'),
    message: t('flows.run.cancelMessage', { flow: shown.value.flow_label }),
    confirmLabel: t('flows.run.cancel'),
    danger: true,
  })
  if (!ok) return
  await act('cancel', () => cancelFlowRun(shown.value.id), t('flows.run.cancelled'))
}

async function retry() {
  await act('retry', () => retryFlowRun(shown.value.id), t('flows.run.retried'))
}

async function act(kind: 'cancel' | 'retry', run: () => Promise<void>, done: string) {
  if (busy.value) return
  busy.value = kind
  try {
    await run()
    ui.success(done)
  } catch (err) {
    ui.error(getErrorMessage(err, t('flows.run.actionFailed')))
  } finally {
    busy.value = ''
  }
  await reload()
  emit('changed')
}
</script>

<template>
  <article class="flow-run-card" data-testid="flow-run" :data-run-id="shown.id">
    <header class="flow-run-card__header">
      <h3 class="flow-run-card__title">
        <RouterLink v-if="linkTitle" :to="space.href(runHref(shown.id))">
          {{ shown.flow_label || shown.flow }}
        </RouterLink>
        <template v-else>{{ shown.flow_label || shown.flow }}</template>
      </h3>
      <span class="flow-run-card__state" data-testid="flow-run-state">
        {{ shown.state_label || shown.state }}
      </span>
      <RlStatusPill
        :label="statusLabel(shown.status, t)"
        :color="statusTone(shown.status)"
        data-testid="flow-run-status"
      />
    </header>

    <p class="flow-run-card__meta">
      <span v-if="shown.started">{{
        t('flows.run.started', { at: formatInstant(shown.started) })
      }}</span>
      <span v-if="shown.completed">
        {{ t('flows.run.completedAt', { at: formatInstant(shown.completed) }) }}
      </span>
    </p>

    <section v-if="waiting" class="flow-run-card__section" :aria-label="t('flows.waiting.title')">
      <h4 class="flow-run-card__heading">{{ t('flows.waiting.title') }}</h4>
      <FlowWaiting :waiting="waiting" />
    </section>

    <section v-if="showStep" class="flow-run-card__section" :aria-label="t('flows.step.title')">
      <h4 class="flow-run-card__heading">{{ t('flows.step.title') }}</h4>
      <FlowStepForm
        :run="shown"
        :subject-type="subjectType"
        @completed="onCompleted"
        @stale="onStale"
      />
    </section>

    <div v-if="shown._actions.cancel || shown._actions.retry" class="flow-run-card__actions">
      <RlButton
        v-if="shown._actions.retry"
        variant="secondary"
        size="sm"
        :loading="busy === 'retry'"
        :pending-label="t('flows.run.retrying')"
        data-testid="flow-retry"
        @click="retry"
      >
        {{ t('flows.run.retry') }}
      </RlButton>
      <RlButton
        v-if="shown._actions.cancel"
        variant="secondary"
        tone="danger"
        size="sm"
        :loading="busy === 'cancel'"
        :pending-label="t('flows.run.cancelling')"
        data-testid="flow-cancel"
        @click="cancel"
      >
        {{ t('flows.run.cancel') }}
      </RlButton>
    </div>

    <details v-if="shown.diagram" class="flow-run-card__section">
      <summary>{{ t('flows.run.diagram') }}</summary>
      <FlowDiagram :source="shown.diagram" />
    </details>

    <details v-if="timeline.length > 0" class="flow-run-card__section" open>
      <summary>{{ t('flows.run.timeline') }}</summary>
      <RlTimeline :entries="timeline" :heading-level="4" />
    </details>
  </article>
</template>

<style scoped>
.flow-run-card {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  padding: var(--space-md);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--radius-lg);
  background: var(--rl-color-bg);
}

.flow-run-card__header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-sm);
}

.flow-run-card__title {
  margin: 0;
  font-size: var(--font-size-base);
}

.flow-run-card__state {
  font-weight: 600;
}

.flow-run-card__meta {
  margin: 0;
  display: flex;
  gap: var(--space-md);
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}

.flow-run-card__heading {
  margin: 0 0 var(--space-xs);
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}

.flow-run-card__section summary {
  cursor: pointer;
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}

.flow-run-card__actions {
  display: flex;
  gap: var(--space-sm);
}
</style>
