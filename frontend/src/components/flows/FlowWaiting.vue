<script setup lang="ts">
/**
 * Why an active run waits (TKT-ZAXRXND): the roles it waits for and whether
 * the reader holds one, its timers, its change branches with the guard's
 * verdict now, outputs that have not settled, a claim and a stranded run.
 * Everything here has passed the reader's gate on the server.
 *
 * Without `detailed`, for a reader who does not manage the flow, it says
 * only whom the step waits for and whether it is being processed or stuck:
 * the roles, guards, timers and attempts describe how the flow works, which
 * such a reader has no use for. The last error of a job step comes only
 * to a manager: the server leaves it out for everyone else.
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FlowJob, FlowWaiting } from '@/api/flows'
import { formatInstant, labelOf } from './flowView'

const props = defineProps<{
  waiting: FlowWaiting
  detailed: boolean
  /** The card shows the reader a form for this step. */
  canComplete?: boolean
  /** The flow's `failed_message:`, said instead of the default to a reader who does not manage the run. */
  failedMessage?: string
}>()

const { t } = useI18n()

const roles = computed(() => (props.waiting.roles ?? []).join(', '))

// The keys are literal so the i18n guard sees them used.
const failKindKeys: Record<NonNullable<FlowJob['fail_kind']>, string> = {
  exhausted: 'flows.waiting.jobFailKind.exhausted',
  permanent: 'flows.waiting.jobFailKind.permanent',
  timeout: 'flows.waiting.jobFailKind.timeout',
  lost: 'flows.waiting.jobFailKind.lost',
}

function jobLine(job: FlowJob): string {
  if (job.phase === 'failed') {
    return t(
      'flows.waiting.jobFailed',
      { n: job.failed, why: t(failKindKeys[job.fail_kind ?? 'exhausted']) },
      job.failed
    )
  }
  const attempt = { n: Math.min(job.failed + 1, job.attempts), max: job.attempts }
  if (job.phase === 'waiting') {
    const at = props.waiting.pending?.next_attempt
    // After a failed attempt the step is retrying, not merely waiting.
    if (job.failed > 0) {
      return at
        ? t('flows.waiting.jobRetryingAt', { ...attempt, at: formatInstant(at) })
        : t('flows.waiting.jobRetrying', attempt)
    }
    return at
      ? t('flows.waiting.jobWaitingAt', { ...attempt, at: formatInstant(at) })
      : t('flows.waiting.jobWaiting', attempt)
  }
  return t('flows.waiting.jobRunning', attempt)
}

/** The one line a reader who does not manage the flow sees. */
const plainLine = computed(() => {
  const w = props.waiting
  if (w.stranded) return ''
  if (w.pending?.job?.phase === 'failed') return props.failedMessage || t('flows.waiting.failedPlain')
  if (w.pending) return t('flows.waiting.processing')
  if (w.roles?.length && !w.you_hold) return t('flows.waiting.forOthers')
  // A role the reader holds without a form here, or a timer: nothing to do.
  if (w.roles?.length && props.canComplete) return t('flows.waiting.forYou')
  return t('flows.waiting.waiting')
})

function branchVerdict(b: { holds: boolean; failed?: boolean }): string {
  if (b.failed) return t('flows.waiting.guardFailed')
  return b.holds ? t('flows.waiting.guardHolds') : t('flows.waiting.guardNotYet')
}
</script>

<template>
  <div v-if="!detailed" class="flow-waiting" data-testid="flow-waiting">
    <p v-if="waiting.stranded" class="flow-waiting__alert" role="alert">
      {{ t('flows.waiting.stranded') }}
    </p>
    <p
      v-else
      class="flow-waiting__line"
      :role="waiting.pending?.job?.phase === 'failed' ? 'alert' : undefined"
      data-testid="flow-waiting-plain"
    >
      {{ plainLine }}
    </p>
  </div>
  <div v-else class="flow-waiting" data-testid="flow-waiting">
    <p v-if="waiting.stranded" class="flow-waiting__alert" role="alert">
      {{ t('flows.waiting.stranded') }}
    </p>
    <p v-if="roles" class="flow-waiting__line" data-testid="flow-waiting-roles">
      {{ t('flows.waiting.roles', { roles }) }}
      <strong v-if="waiting.you_hold">{{ t('flows.waiting.youHold') }}</strong>
      <span v-else>{{ t('flows.waiting.youDoNotHold') }}</span>
    </p>
    <p v-else-if="!waiting.stranded && !waiting.pending" class="flow-waiting__line">
      {{ t('flows.waiting.noRoles') }}
    </p>
    <p v-if="waiting.claimed_by" class="flow-waiting__line">
      {{ t('flows.waiting.claimedBy', { who: waiting.claimed_by }) }}
    </p>
    <p
      v-if="waiting.pending?.job"
      class="flow-waiting__line"
      :role="waiting.pending.job.phase === 'failed' ? 'alert' : undefined"
      data-testid="flow-waiting-job"
    >
      {{ jobLine(waiting.pending.job) }}
    </p>
    <p v-else-if="waiting.pending" class="flow-waiting__line" data-testid="flow-waiting-pending">
      {{ t('flows.waiting.pending', { n: waiting.pending.failures }) }}
      <template v-if="waiting.pending.next_attempt">
        {{ t('flows.waiting.nextAttempt', { at: formatInstant(waiting.pending.next_attempt) }) }}
      </template>
    </p>
    <p
      v-if="waiting.pending?.job?.error"
      class="flow-waiting__line flow-waiting__muted"
      data-testid="flow-waiting-job-error"
    >
      {{ t('flows.waiting.jobError') }}
      <code class="flow-waiting__error">{{ waiting.pending.job.error }}</code>
    </p>

    <div v-if="waiting.branches?.length" class="flow-waiting__group">
      <h4 class="flow-waiting__heading">{{ t('flows.waiting.branches') }}</h4>
      <ul class="flow-waiting__list">
        <li v-for="(b, i) in waiting.branches" :key="i" data-testid="flow-branch">
          <span v-if="b.fork?.length">
            {{ t('flows.waiting.branchFork', { states: b.fork.join(', ') }) }}
          </span>
          <span v-else>{{ t('flows.waiting.branchTo', { state: b.label || b.to }) }}</span>
          <code v-if="b.guard" class="flow-waiting__guard">{{ b.guard }}</code>
          <span
            class="flow-waiting__verdict"
            :class="{ 'is-holds': b.holds, 'is-failed': b.failed }"
          >
            {{ branchVerdict(b) }}
          </span>
          <span v-if="b.roles?.length" class="flow-waiting__muted">
            {{ t('flows.waiting.branchRoles', { roles: b.roles.join(', ') }) }}
          </span>
        </li>
      </ul>
    </div>

    <div v-if="waiting.timers?.length" class="flow-waiting__group">
      <h4 class="flow-waiting__heading">{{ t('flows.waiting.timers') }}</h4>
      <ul class="flow-waiting__list">
        <li v-for="tm in waiting.timers" :key="tm.name" data-testid="flow-timer">
          <span>{{ labelOf(tm.label, tm.name) }}</span>
          <span class="flow-waiting__muted">
            {{
              tm.due
                ? t('flows.waiting.timerDue', { state: tm.to, at: formatInstant(tm.due) })
                : t('flows.waiting.timerTo', { state: tm.to })
            }}
          </span>
        </li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
.flow-waiting {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
}

.flow-waiting__line,
.flow-waiting__alert {
  margin: 0;
}

.flow-waiting__alert {
  color: var(--rl-color-danger);
}

.flow-waiting__group {
  margin-top: var(--space-xs);
}

.flow-waiting__heading {
  margin: 0 0 var(--space-xs);
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}

.flow-waiting__list {
  margin: 0;
  padding-left: var(--space-lg);
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
}

.flow-waiting__list li {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
  align-items: baseline;
}

.flow-waiting__guard {
  font-size: var(--font-size-sm);
}

.flow-waiting__verdict {
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}

.flow-waiting__verdict.is-holds {
  color: var(--rl-color-success-fg);
}

.flow-waiting__verdict.is-failed {
  color: var(--rl-color-danger);
}

.flow-waiting__error {
  font-size: var(--font-size-sm);
  overflow-wrap: anywhere;
}

.flow-waiting__muted {
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}
</style>
