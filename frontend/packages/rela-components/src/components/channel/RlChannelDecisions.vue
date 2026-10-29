<script setup lang="ts">
/**
 * The decisions a session waits for, as buttons: one per
 * `ChannelDecisionView`, in the order given.
 *
 * Only a message moves a session. A decision button stands for one: the app
 * posts it on `decide`, or, for a decision with `opensForm`, opens the
 * composer with its type. The library holds no state and posts nothing.
 *
 * The buttons sit in a `role="group"` named "Decisions". Each reads its
 * label, then hidden text with its effect (`channelDecisionHint`). The
 * pending one (its `decisionPendingKey` is in `pending`) shows the
 * library's pending treatment and stays focusable as `aria-disabled`; every
 * other one is `disabled` while any decision key is pending, including
 * `DECISIONS_BLOCKED_KEY`, which blocks them without marking one. An empty
 * list renders nothing. The row wraps, so a narrow panel keeps every
 * button visible.
 */
import { computed } from 'vue'
import RlButton from '../common/RlButton.vue'
import { anyDecisionPending, decisionPendingKey } from './channelFields'
import type { ChannelDecisionView } from './types'
import { useMessages } from '../../composables/useMessages'

const props = defineProps<{
  decisions: ChannelDecisionView[]
  /** Operations in flight; decision keys come from `decisionPendingKey`. */
  pending?: ReadonlySet<string>
  /** Ids of elements that describe the group, such as the message it sits on. */
  describedBy?: string
}>()

const emit = defineEmits<{ decide: [key: string] }>()

const messages = useMessages()
const blocked = computed(() => anyDecisionPending(props.pending))

function isPending(d: ChannelDecisionView): boolean {
  return props.pending?.has(decisionPendingKey(d.key)) ?? false
}

function hint(d: ChannelDecisionView): string {
  return messages.channelDecisionHint({ to: d.toLabel, form: d.opensForm })
}
</script>

<template>
  <div
    v-if="decisions.length"
    role="group"
    class="rl-channel-decisions"
    :aria-label="messages.channelText({ key: 'decisions' })"
    :aria-describedby="describedBy"
  >
    <RlButton
      v-for="d in decisions"
      :key="d.key"
      size="sm"
      variant="secondary"
      icon="workflow"
      :tone="d.danger ? 'danger' : 'default'"
      :loading="isPending(d)"
      :pending-label="messages.channelText({ key: 'sending' })"
      :disabled="blocked && !isPending(d)"
      :data-decision="d.key"
      @click="emit('decide', d.key)"
    >
      {{ d.label }}<span class="rl-visually-hidden">{{ hint(d) }}</span>
    </RlButton>
  </div>
</template>

<style scoped>
.rl-channel-decisions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-2);
  min-width: 0;
}
</style>
