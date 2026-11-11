<script setup lang="ts">
/**
 * Why an active run waits (TKT-ZAXRXND): the roles it waits for and whether
 * the reader holds one, its timers, its change branches with the guard's
 * verdict now, outputs that have not settled, a claim and a stranded run.
 * Everything here has passed the reader's gate on the server.
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FlowWaiting } from '@/api/flows'
import { formatInstant } from './flowView'

const props = defineProps<{ waiting: FlowWaiting }>()

const { t } = useI18n()

const roles = computed(() => (props.waiting.roles ?? []).join(', '))

function branchVerdict(b: { holds: boolean; failed?: boolean }): string {
  if (b.failed) return t('flows.waiting.guardFailed')
  return b.holds ? t('flows.waiting.guardHolds') : t('flows.waiting.guardNotYet')
}
</script>

<template>
  <div class="flow-waiting" data-testid="flow-waiting">
    <p v-if="waiting.stranded" class="flow-waiting__alert" role="alert">
      {{ t('flows.waiting.stranded') }}
    </p>
    <p v-if="roles" class="flow-waiting__line" data-testid="flow-waiting-roles">
      {{ t('flows.waiting.roles', { roles }) }}
      <strong v-if="waiting.you_hold">{{ t('flows.waiting.youHold') }}</strong>
      <span v-else>{{ t('flows.waiting.youDoNotHold') }}</span>
    </p>
    <p v-else-if="!waiting.stranded" class="flow-waiting__line">
      {{ t('flows.waiting.noRoles') }}
    </p>
    <p v-if="waiting.claimed_by" class="flow-waiting__line">
      {{ t('flows.waiting.claimedBy', { who: waiting.claimed_by }) }}
    </p>
    <p v-if="waiting.pending" class="flow-waiting__line" data-testid="flow-waiting-pending">
      {{ t('flows.waiting.pending', { n: waiting.pending.failures }) }}
      <template v-if="waiting.pending.next_attempt">
        {{ t('flows.waiting.nextAttempt', { at: formatInstant(waiting.pending.next_attempt) }) }}
      </template>
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
          <span>{{ tm.label || tm.name }}</span>
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

.flow-waiting__muted {
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}
</style>
