<script setup lang="ts">
/**
 * The waiting detail of one inbox row (TKT-V05706X): the message that moved
 * the session into its state, its choices and the session decisions, as the
 * channel page shows them.
 *
 * The decisions and choices are built by the page's own builders
 * (useSessionDecisions over inboxHead), from the decision inputs the entry
 * carries. A one-click decision or choice posts from the row with the
 * entry's generation; a form decision or "Other answer" emits `answer`, and
 * the inbox opens the channel in a drawer to start it there.
 *
 * Choices show only while the waiting message has no reply (offersChoices).
 * With messages after it, the row says how many, and "Open channel" emits
 * `open`. No buttons show until the channel type's descriptor is held.
 *
 * `afterPost` runs once a post lands: the inbox reads itself again and
 * moves the focus. `refetch` reads the inbox again after a refusal.
 */
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { postMessage } from '@/api/channels'
import { useConfirm } from '@/composables/useConfirm'
import { useChannelSchemaStore } from '@/stores/channelSchema'
import { useSchemaStore, useUIStore } from '@/stores'
import { browserTimeZone } from '@/utils/format'
import type { ChannelStart, ChannelTypeSchema, InboxEntry } from '@/types/channels'
import type { ChannelDecisionView } from 'rela-components/components/channel/types'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlChannelDecisions from 'rela-components/components/channel/RlChannelDecisions.vue'
import RlChannelMessage from 'rela-components/components/channel/RlChannelMessage.vue'
import {
  enumOptionsFor,
  messageView,
  presentationView,
  rowDecisions,
} from '@/components/entity/channels/channelView'
import { handleChannelRefusal, shownPending } from '@/components/entity/channels/decisionGlue'
import { useChannelLinks } from '@/components/entity/channels/useChannelLinks'
import { usePendingKeys } from '@/components/entity/channels/usePendingKeys'
import { useSessionDecisions } from '@/components/entity/channels/useSessionDecisions'
import { inboxHead, inboxMessages, newerCount, offersChoices } from './inboxDecisionHead'

const props = defineProps<{
  entry: InboxEntry
  afterPost: () => Promise<void>
  refetch: () => Promise<void>
}>()

const emit = defineEmits<{ answer: [start: ChannelStart]; open: [] }>()

/** A message longer than this many characters of text is folded. */
const FOLD_CHARS = 600

const { t } = useI18n()
const ui = useUIStore()
const schema = useSchemaStore()
const channelSchema = useChannelSchemaStore()
const { confirm } = useConfirm()
const tz = browserTimeZone()

const typeName = computed(() => props.entry.channel.type)
const held = computed(() => channelSchema.channelType(typeName.value))
/**
 * Until the descriptor is held, a stand-in type without a session, so the
 * builders return nothing. The cast is safe: the stand-in only reaches
 * decisionViews and choiceViews, which read `session` and `message_types`,
 * and every button is also gated on `held` in the template.
 */
const ct = computed<ChannelTypeSchema>(
  () =>
    held.value ??
    ({
      name: typeName.value,
      attach_to: [],
      cardinality: 'one',
      lifecycle: [],
      message_types: [],
    } as unknown as ChannelTypeSchema)
)
const messageType = (name: string) => channelSchema.messageType(typeName.value, name)

const head = computed(() => inboxHead(props.entry))
const messages = computed(() => inboxMessages(props.entry))
/**
 * The waiting message's seq, while it is shown and not removed. A removed
 * message takes no reply, so a decision on its row must not name it.
 */
const cause = computed(() => {
  const m = props.entry.message
  return m && !m.removed ? props.entry.session.cause : undefined
})

// A state or type the held descriptor lacks reloads it once.
watch(
  () => props.entry,
  (e) => channelSchema.checkNames(e.channel.type, inboxHead(e), inboxMessages(e)),
  { immediate: true }
)

const target = computed(() => {
  const c = props.entry.channel
  return {
    type: c.entity_type,
    address: c.anchor,
    channelType: c.type,
    ...(c.name ? { name: c.name } : {}),
  }
})

function handle(err: unknown, hasForm: boolean): string | undefined {
  return handleChannelRefusal(err, hasForm, {
    refetch: () => void props.refetch(),
    gone: () => void props.refetch(),
    reloadSchema: () => {
      void channelSchema.reload()
      void schema.reloadChannelPresentations()
    },
    toast: (m) => ui.error(m),
  })
}

const { pending, set: setPending } = usePendingKeys((err) => handle(err, false))

const decisionsApi = useSessionDecisions({
  channelType: () => ct.value,
  channelKey: () => JSON.stringify(target.value),
  ch: {
    head,
    messages,
    hasNewer: ref(false),
    actionsGeneration: computed(() => props.entry.session.generation),
    cause,
    refreshHead: () => props.afterPost(),
    refetch: () => props.refetch(),
  },
  messageType,
  post: (type, values, replyTo, expectGeneration) =>
    postMessage(target.value, type, values, replyTo, expectGeneration),
  setPending,
  submitting: ref(false),
  // Form decisions go to the drawer, so the row's composer is never asked.
  composer: {
    editing: () => false,
    discardEdit: () => {},
    endReply: () => {},
    reply: () => {},
    select: () => false,
    showError: (m) => ui.error(m),
  },
  ui: {
    confirm,
    toast: (m) => ui.error(m),
    handle,
    // The inbox moves the focus once it has read itself again.
    focusRow: () => true,
    focusHeader: () => {},
  },
})
const { decisions, decisionSeq, deciding, onDecide, onChoose, resetDecisions } = decisionsApi

// A refetch that drops this row while a confirm is open releases the lock.
onScopeDispose(resetDecisions)

const choices = computed(() =>
  offersChoices(props.entry) ? decisionsApi.choices.value : new Map<number, ChannelDecisionView[]>()
)
const ownChoices = computed(() =>
  decisionSeq.value === undefined ? undefined : choices.value.get(decisionSeq.value)
)
const shownDecisions = computed(() =>
  rowDecisions(decisions.value, choices.value, decisionSeq.value)
)
const blocked = computed(() => shownPending(pending.value, deciding.value))

const presentation = computed(() =>
  presentationView(schema.channelPresentations.get(typeName.value))
)
const enumOptions = computed(() => enumOptionsFor(held.value, schema.customTypes, schema.styles))
const view = computed(() => {
  const m = props.entry.message
  if (!m || !held.value) return undefined
  return messageView(m, m.type ? messageType(m.type) : undefined, {
    tz,
    session: held.value.session,
  })
})

const foldable = computed(() => {
  let n = 0
  for (const v of Object.values(props.entry.message?.props ?? {})) {
    if (typeof v === 'string') n += v.length
  }
  return n > FOLD_CHARS
})
const unfolded = ref(false)
/**
 * A folded message hides its lower part, so its buttons render below it
 * rather than inside it.
 */
const folded = computed(() => !!view.value && foldable.value && !unfolded.value)
/** A message that takes its decisions: shown and not removed. */
const onMessage = computed(() => !!view.value && !view.value.removed)

const newer = computed(() => newerCount(props.entry))
const newerText = computed(() =>
  newer.value > 99
    ? t('inbox.newerMessagesMany')
    : t('inbox.newerMessages', { n: newer.value }, newer.value)
)

function find(key: string): ChannelDecisionView | undefined {
  return decisions.value.find((v) => v.key === key) ?? ownChoices.value?.find((v) => v.key === key)
}

function start(kind: ChannelStart['kind'], v: ChannelDecisionView) {
  emit('answer', {
    kind,
    key: v.key,
    generation: props.entry.session.generation,
    ...(cause.value !== undefined ? { cause: cause.value } : {}),
    ...(kind === 'choose' ? { answerType: v.messageType } : {}),
  })
}

function onDecideKey(key: string) {
  const v = find(key)
  if (v?.opensForm) start('decide', v)
  else void onDecide(key, 'row')
}

function onChooseKey(key: string) {
  const v = ownChoices.value?.find((x) => x.key === key)
  if (v?.opensForm) start('choose', v)
  else void onChoose(key)
}

const { entityHref, onOpenRef, onLogClick } = useChannelLinks(undefined)
</script>

<template>
  <div class="inbox-waiting" data-testid="inbox-waiting-detail" @click="onLogClick">
    <div
      v-if="view"
      class="inbox-waiting__message"
      :class="{ 'inbox-waiting__message--folded': folded }"
    >
      <RlChannelMessage
        :message="view"
        :message-type="view.type ? messageType(view.type) : undefined"
        :presentation="view.type ? presentation?.messages?.[view.type] : undefined"
        :enum-options="enumOptions"
        :entity-href="entityHref"
        :pending="blocked"
        :can-reply="false"
        :decisions="held && onMessage && !folded ? shownDecisions : undefined"
        :choices="held && onMessage && !folded ? ownChoices : undefined"
        @decide="onDecideKey"
        @choose="onChooseKey"
        @open-ref="onOpenRef"
      />
    </div>
    <RlButton v-if="view && foldable" variant="ghost" size="sm" @click="unfolded = !unfolded">
      {{ unfolded ? t('inbox.showLess') : t('inbox.showMore') }}
    </RlButton>
    <template v-if="folded && onMessage && held">
      <RlChannelDecisions
        v-if="ownChoices?.length"
        kind="choice"
        :decisions="ownChoices"
        :pending="blocked"
        @decide="onChooseKey"
      />
      <RlChannelDecisions
        v-if="shownDecisions.length"
        :decisions="shownDecisions"
        :pending="blocked"
        @decide="onDecideKey"
      />
    </template>
    <RlChannelDecisions
      v-if="!onMessage && held && decisions.length"
      :decisions="decisions"
      :pending="blocked"
      @decide="onDecideKey"
    />
    <p v-if="newer > 0" class="inbox-waiting__newer">
      <span>{{ newerText }}</span>
      <RlButton variant="ghost" size="sm" @click="emit('open')">
        {{ t('inbox.openChannel') }}
      </RlButton>
    </p>
  </div>
</template>

<style scoped>
.inbox-waiting {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-xs);
}

.inbox-waiting__message {
  align-self: stretch;
}

.inbox-waiting__message--folded {
  max-height: 16rem;
  overflow: hidden;
  mask-image: linear-gradient(to bottom, black 75%, transparent);
}

.inbox-waiting__newer {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin: 0;
  color: var(--rl-color-text-muted);
  font-size: var(--font-size-sm);
}
</style>
