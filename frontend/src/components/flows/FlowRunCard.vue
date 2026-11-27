<script setup lang="ts">
/**
 * One flow run (TKT-ZAXRXND): its state and status, why it waits, the step
 * form, the diagram and the timeline, and the controls `_actions` allows.
 * A run in parallel branches (TKT-ZO9IB6V) waits at its join: the card
 * shows each branch with its own reason and step form instead.
 *
 * The run shown is the prop until the card reads the run itself: after a
 * step, a cancel or a retry, and on mount for an active run the prop holds
 * without its timeline. It then emits `changed`, so the host can refresh
 * the list it came from.
 *
 * How the run works (the roles and guards it waits on, the diagram and the
 * timeline) shows only when `_actions.manage` says the reader manages the
 * flow. A reader who only does a step sees the state, whether the step
 * waits for them, and the step form. This is presentation, not access: the
 * server still sends the run to every reader of its subject.
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
import { isCancelledFetch } from '@/api/errors'
import { useConfirm } from '@/composables/useConfirm'
import { useUIStore } from '@/stores'
import { useSpaceStore } from '@/stores/space'
import FlowWaiting from './FlowWaiting.vue'
import FlowStepForm from './FlowStepForm.vue'
import FlowDiagram from './FlowDiagram.vue'
import {
  formatInstant,
  labelOf,
  runHref,
  statusLabel,
  statusTone,
  timelineEntries,
} from './flowView'

const {
  run,
  subjectType = '',
  linkTitle = false,
  hideTitle = false,
} = defineProps<{
  run: FlowRun
  /** The flow's subject type; links and the step form need it. */
  subjectType?: string
  /**
   * Link the title to the run's own page. Only a reader who manages the run
   * gets the link: the page adds nothing for anyone else, who sees the run's
   * state and status on this card.
   */
  linkTitle?: boolean
  /** Leave the flow label out; the run's own page already shows it as the page title. */
  hideTitle?: boolean
}>()

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
  detail.value && detail.value.id === run.id ? detail.value : run
)

async function reload() {
  controller?.abort()
  const ctl = new AbortController()
  controller = ctl
  try {
    detail.value = await getFlowRun(run.id, ctl.signal)
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
// subject's list already sends it, also for a finished run, whose header
// keeps its timeline.
watch(
  () => [run.id, run.generation, run.status],
  () => {
    detail.value = null
    if (run.active && run.log === undefined) void reload()
  },
  { immediate: true }
)

const manage = computed(() => !!shown.value._actions.manage)
const timeline = computed(() => (manage.value ? timelineEntries(shown.value, t) : []))
const places = computed(() => (shown.value.active ? (shown.value.parallel ?? []) : []))
const waiting = computed(() =>
  shown.value.active && places.value.length === 0 ? shown.value.waiting : undefined
)
const showStep = computed(() => !!waiting.value && shown.value._actions.complete && !!subjectType)
/** A reader with the step form open needs no line saying it waits for them. */
const showWaiting = computed(() => !!waiting.value && (manage.value || !showStep.value))

/**
 * Each branch as the run the step form completes: the branch's generation
 * names its step, and its waiting names the step's form.
 */
const branches = computed(() =>
  places.value.map((place) => ({
    place,
    run: { ...shown.value, generation: place.generation, waiting: place.waiting },
    showStep: !!place.waiting && place.complete && !!subjectType,
  }))
)

// The notice for each Completed.problem, keyed literally so the i18n guard
// sees every message in use. The step form shows a blocked step itself, at
// its fields, where the reader is.
const problemKeys: Record<Exclude<NonNullable<FlowCompleted['problem']>, 'blocked'>, string> = {
  guard: 'flows.step.problem.guard',
  moved: 'flows.step.problem.moved',
  settling: 'flows.step.problem.settling',
}

async function onCompleted(result: FlowCompleted) {
  if (result.problem && result.problem !== 'blocked') ui.info(t(problemKeys[result.problem]))
  else if (!result.problem)
    ui.success(result.finished ? t('flows.step.finished') : t('flows.step.done'))
  // A form that stays shows its warnings at the fields; one that goes cannot.
  if (result.moved && result.warnings?.length) ui.warning(t('flows.step.movedWithWarnings'))
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

async function act(kind: 'cancel' | 'retry', request: () => Promise<void>, done: string) {
  if (busy.value) return
  busy.value = kind
  try {
    await request()
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
      <h3 v-if="!hideTitle" class="flow-run-card__title">
        <RouterLink v-if="linkTitle && manage" :to="space.href(runHref(shown.id))">
          {{ labelOf(shown.flow_label, shown.flow) }}
        </RouterLink>
        <template v-else>{{ labelOf(shown.flow_label, shown.flow) }}</template>
      </h3>
      <span class="flow-run-card__state" data-testid="flow-run-state">
        {{ labelOf(shown.state_label, shown.state) }}
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

    <section
      v-if="waiting && showWaiting"
      class="flow-run-card__section"
      :aria-label="t('flows.waiting.title')"
    >
      <h4 v-if="manage" class="flow-run-card__heading">{{ t('flows.waiting.title') }}</h4>
      <FlowWaiting
        :waiting="waiting"
        :detailed="manage"
        :can-complete="showStep"
        :failed-message="shown.failed_message"
      />
    </section>

    <section
      v-if="places.length > 0"
      class="flow-run-card__section"
      :aria-label="t('flows.run.parallel')"
    >
      <h4 class="flow-run-card__heading">{{ t('flows.run.parallel') }}</h4>
      <div
        v-for="b in branches"
        :key="b.place.branch"
        class="flow-run-card__branch"
        :class="{ 'flow-run-card__step': b.showStep }"
        data-testid="flow-run-branch"
        :data-step-anchor="b.showStep ? '' : undefined"
        :data-branch="b.place.branch"
      >
        <header class="flow-run-card__header">
          <span class="flow-run-card__muted">
            {{ t('flows.run.branch', { branch: b.place.branch }) }}
          </span>
          <span class="flow-run-card__state">{{
            labelOf(b.place.state_label, b.place.state)
          }}</span>
          <RlStatusPill
            :label="statusLabel(b.place.status, t)"
            :color="statusTone(b.place.status)"
          />
        </header>
        <p v-if="b.place.arrived" class="flow-run-card__muted" data-testid="flow-branch-arrived">
          {{ t('flows.run.arrived') }}
        </p>
        <FlowWaiting
          v-else-if="b.place.waiting && (manage || !b.showStep)"
          :waiting="b.place.waiting"
          :detailed="manage"
          :can-complete="b.showStep"
          :failed-message="shown.failed_message"
        />
        <FlowStepForm
          v-if="b.showStep"
          :run="b.run"
          :subject-type="subjectType"
          @completed="onCompleted"
          @stale="onStale"
        />
      </div>
    </section>

    <section
      v-if="showStep"
      class="flow-run-card__section flow-run-card__step"
      data-step-anchor
      :aria-label="t('flows.step.title')"
    >
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

    <details v-if="manage && shown.diagram" class="flow-run-card__section">
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
/* `?run=` scrolls the card to the top of the pane; on a phone the entity
   page's sticky top bar would cover it, so stop below the bar. */
.flow-run-card {
  scroll-margin-top: var(--rl-inline-edit-sticky-top, 0px);
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  padding: var(--space-md);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--radius-lg);
  background: var(--rl-color-bg);
}

/* `?run=` scrolls the step form to the top of the pane; stop below the
   entity page's sticky bar here too. */
.flow-run-card__step {
  scroll-margin-top: var(--rl-inline-edit-sticky-top, 0px);
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

.flow-run-card__branch {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
  padding: var(--space-sm) 0;
  border-top: 1px solid var(--rl-color-border);
}

.flow-run-card__muted {
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}

.flow-run-card__actions {
  display: flex;
  gap: var(--space-sm);
}
</style>
