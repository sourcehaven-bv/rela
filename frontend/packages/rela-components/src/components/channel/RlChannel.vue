<script setup lang="ts">
/**
 * One open channel: its header and lifecycle controls, the message log, the
 * ephemeral signals, and a slot for the composer.
 *
 * Everything the user may do comes in as data: `allowedLifecycle` is the
 * channel's `_actions.lifecycle`, `postable` its `_actions.post`, and each
 * message carries its own `actions`. A control the user may not use is not
 * shown. The composer is the caller's to mount in the `composer` slot, which
 * renders only while the channel is open; a closed or archived channel says
 * so instead. The `session` slot sits between the header and the log in
 * every channel state, since a closed channel still shows where its session
 * stopped; the app mounts `RlChannelSession` there.
 *
 * The log opens at the newest message and follows new ones, and messages
 * that grow, only while the reader is at the bottom, so reading back is never
 * yanked away. Any other change (older messages loaded above, the oldest
 * trimmed at the app's cap, an edit above the viewport) keeps the first
 * visible message where it was. A log first shown after mounting (a hidden
 * tab) still opens at the bottom.
 *
 * `messages` must be replaced rather than mutated in place: the scroll
 * handling runs when the array changes identity.
 *
 * The log itself is not a live region, so loading a page of older messages
 * or reloading the whole log is not read out. Messages that arrive at the
 * end are announced through a separate, visually hidden polite region.
 * Session notices among them are read after the other messages, by their
 * own text, so a move is announced once and the stepper need not be a live
 * region.
 *
 * `decisions` are the session decisions; the row whose seq is
 * `decisionSeq` (the message that moved the session into its current
 * state) shows them and emits `decide` with `from: 'row'`. No other row
 * does. `focusRow(seq)` puts focus on a row without scrolling, for after a
 * decision removes the button that had it. `jumpTo(seq)` is the reply
 * quote's jump, also used by a message link.
 *
 * `choices` are the reply buttons of open questions, keyed by the
 * question's seq; each row shows its own and emits `choose` with the key.
 *
 * `typing` shows who else is typing as one line, in its own polite, atomic
 * live region apart from the signals, so it is read out only when the line
 * changes. It names at most two typists and counts the rest. The line is
 * there only when `typingOffered` is set and the channel is open with a
 * composer; it stays mounted while empty, so the live region exists before
 * its first text.
 *
 * Message types whose presentation sets `display` are quiet: `collapsed`
 * runs fold into one line that expands in place, `hidden` messages are left
 * out (see `channelFold.ts`). A row the reader must act on or asked for is
 * kept in full for the rest of the mount. The header then offers a toggle
 * that shows every message, and only messages shown in full are announced;
 * a new quiet message that becomes kept later is announced then.
 * `reveal(seq)` shows, scrolls to and focuses a held message, expanding or
 * keeping it as needed.
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId, useTemplateRef, watch } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlButtonGroup from '../common/RlButtonGroup.vue'
import RlHeading from '../common/RlHeading.vue'
import RlIcon from '../common/RlIcon.vue'
import RlTag from '../common/RlTag.vue'
import RlChannelAuthor from './RlChannelAuthor.vue'
import RlChannelFold from './RlChannelFold.vue'
import RlChannelMessage from './RlChannelMessage.vue'
import { ephemeralText, lifecyclePendingKey } from './channelFields'
import { foldMessages, hasTypeAction, rowFor, shownAs, type ChannelFoldRow } from './channelFold'
import { findAnchor, firstVisible, measured, type Anchor } from './channelAnchor'
import type {
  ChannelDecisionView,
  ChannelEnumOption,
  ChannelEphemeralView,
  ChannelLifecycleDef,
  ChannelMessageTypeDef,
  ChannelMessageView,
  ChannelPresentation,
  ChannelReadableRef,
  ChannelStateView,
  ChannelTypingView,
} from './types'
import { useMessages, type ChannelTextKey } from '../../composables/useMessages'

const props = withDefaults(
  defineProps<{
    title: string
    state: ChannelStateView
    /** Oldest first. Replace the array on every change; see the header comment. */
    messages: ChannelMessageView[]
    /** Every message type of the channel type. */
    messageTypes: ChannelMessageTypeDef[]
    /** The message types the user may post now. */
    postable?: string[]
    /** Every lifecycle operation of the channel type, for the labels. */
    lifecycle?: ChannelLifecycleDef[]
    /** The lifecycle operations the user may run now. */
    allowedLifecycle?: string[]
    ephemeral?: ChannelEphemeralView[]
    /** Who else is typing, in the order to name them. */
    typing?: ChannelTypingView[]
    /** The channel type offers typing, so the typing line is shown. */
    typingOffered?: boolean
    presentation?: ChannelPresentation
    enumOptions?: Record<string, ChannelEnumOption[]>
    /**
     * The link for a readable ref: an in-app path built from `ref.id`, never
     * text an author wrote. See `RlRefChip`.
     */
    entityHref?: (ref: ChannelReadableRef) => string | undefined
    /** Operations in flight, by `messagePendingKey` and `lifecyclePendingKey`. */
    pending?: ReadonlySet<string>
    /** Older messages exist than the oldest shown. */
    hasOlder?: boolean
    /** Newer messages exist than the newest shown (after reading far back). */
    hasNewer?: boolean
    loadingOlder?: boolean
    /** The level of the title, so the channel fits the host page's outline. */
    headingLevel?: 2 | 3 | 4 | 5 | 6
    /** The session decisions, shown on the row with seq `decisionSeq`. */
    decisions?: ChannelDecisionView[]
    /** The seq of the message that moved the session; absent shows them on no row. */
    decisionSeq?: number
    /** The reply buttons of open questions, by the question's seq. */
    choices?: ReadonlyMap<number, ChannelDecisionView[]>
  }>(),
  {
    postable: () => [],
    lifecycle: () => [],
    allowedLifecycle: () => [],
    ephemeral: () => [],
    typing: () => [],
    typingOffered: false,
    presentation: undefined,
    enumOptions: undefined,
    entityHref: undefined,
    pending: undefined,
    hasOlder: false,
    hasNewer: false,
    loadingOlder: false,
    headingLevel: 3,
    decisions: undefined,
    decisionSeq: undefined,
    choices: undefined,
  },
)

const emit = defineEmits<{
  lifecycle: [op: string]
  reply: [seq: number]
  edit: [seq: number]
  delete: [seq: number]
  remove: [seq: number]
  release: [seq: number]
  action: [seq: number, name: string]
  openRef: [ref: ChannelReadableRef, event: MouseEvent]
  loadOlder: []
  loadNewest: []
  /** A reply quote points at a message that is not loaded. */
  jump: [seq: number]
  decide: [key: string, from: 'row']
  choose: [key: string]
}>()

const strings = useMessages()
const t = (key: ChannelTextKey) => strings.channelText({ key })
const titleId = useId()
const foldId = useId()

const log = useTemplateRef<HTMLElement>('log')
const list = useTemplateRef<HTMLElement>('list')

const typeByName = computed(() => new Map(props.messageTypes.map((t) => [t.name, t])))

/** The typing line, or empty when nobody else is typing. */
const typingText = computed(() => {
  if (props.typing.length === 0) return ''
  const names = props.typing.slice(0, 2).map((e) => e.author.name)
  return strings.channelTyping({ names, others: props.typing.length - names.length })
})

const lifecycleButtons = computed(() => {
  const allowed = new Set(props.allowedLifecycle)
  return props.lifecycle.filter((op) => allowed.has(op.name))
})

const canReply = computed(
  () =>
    props.state === 'open' &&
    props.messageTypes.some((t) => t.reply && props.postable.includes(t.name)),
)

/* Quiet message types ------------------------------------------------- */

const showAll = ref(false)
/** Seqs of expanded runs. */
const expanded = ref<ReadonlySet<number>>(new Set())
/** Seqs kept in full during this mount; only grows. */
const revealed = ref<ReadonlySet<number>>(new Set())

const displayOf = (type: string) => props.presentation?.messages?.[type]?.display
const hasQuiet = computed(() => Object.values(props.presentation?.messages ?? {}).some((p) => p?.display))
const quietOn = computed(() => hasQuiet.value && !showAll.value)

/** The rows kept right now: the decision cause, open questions, rows with a type action. */
const keptNow = computed(() => {
  const s = new Set<number>(props.choices?.keys() ?? [])
  if (props.decisionSeq !== undefined) s.add(props.decisionSeq)
  for (const m of props.messages) if (hasTypeAction(m)) s.add(m.seq)
  return s
})

/** The text of the polite region that announces new messages. */
const announcement = ref('')

/**
 * Seqs of new messages that were not announced only because they were quiet.
 * The choices and the decision cause come with the head, after the messages,
 * so such a row may become kept later; it is announced then.
 */
const silenced = new Set<number>()

// With every message shown, the reader has seen them; nothing is late.
watch(showAll, () => silenced.clear())

watch(
  keptNow,
  (now) => {
    const add = [...now].filter((seq) => !revealed.value.has(seq))
    if (add.length) revealed.value = new Set([...revealed.value, ...add])
    const late = props.messages.filter((m) => now.has(m.seq) && silenced.delete(m.seq))
    if (late.length) announcement.value = newMessagesText(late)
  },
  { immediate: true },
)

const rows = computed(() =>
  foldMessages(props.messages, {
    displayOf,
    labelOf: (type) => typeByName.value.get(type)?.label ?? type,
    showAll: !quietOn.value,
    keep: new Set([...revealed.value, ...keptNow.value]),
    expanded: expanded.value,
  }),
)

const signals = computed(() =>
  quietOn.value ? props.ephemeral.filter((e) => !displayOf(e.type)) : props.ephemeral,
)

const signalText = (e: ChannelEphemeralView) =>
  ephemeralText(e, typeByName.value, props.presentation, props.enumOptions, t)

/** The props of one message row, the same in the log and in an expanded run. */
function messageBind(m: ChannelMessageView) {
  return {
    message: m,
    messageType: m.type ? typeByName.value.get(m.type) : undefined,
    presentation: m.type ? props.presentation?.messages?.[m.type] : undefined,
    enumOptions: props.enumOptions,
    entityHref: props.entityHref,
    pending: props.pending,
    canReply: canReply.value,
    decisions: props.decisions && m.seq === props.decisionSeq ? props.decisions : undefined,
    choices: props.choices?.get(m.seq),
  }
}

const messageOn = {
  decide: (key: string) => emit('decide', key, 'row'),
  choose: (key: string) => emit('choose', key),
  reply: (s: number) => emit('reply', s),
  edit: (s: number) => emit('edit', s),
  delete: (s: number) => emit('delete', s),
  remove: (s: number) => emit('remove', s),
  release: (s: number) => emit('release', s),
  action: (s: number, n: string) => emit('action', s, n),
  openRef: (r: ChannelReadableRef, e: MouseEvent) => emit('openRef', r, e),
  jump: (s: number) => jumpTo(s),
}

/* Scroll handling ------------------------------------------------------- */

/** Whether the reader is at the bottom; kept up to date on every scroll. */
const stuck = ref(true)

function atBottom(el: HTMLElement): boolean {
  return el.scrollHeight - el.scrollTop - el.clientHeight < 24
}

function scrollToBottom() {
  if (log.value) log.value.scrollTop = log.value.scrollHeight
}

function onScroll() {
  // A log with no height (hidden) cannot be scrolled; keep what it was.
  if (log.value && log.value.clientHeight > 0) stuck.value = atBottom(log.value)
}

/**
 * Keeps the reader's view across a change: at the bottom the log stays at
 * the bottom, otherwise `given` or the first visible row keeps its offset.
 * Call it before the change; it restores after the next render.
 */
function keepView(given?: Anchor) {
  const el = log.value
  if (!el) return
  const follow = stuck.value
  const anchor = follow ? null : (given ?? firstVisible(el))
  void nextTick(() => {
    if (follow) {
      scrollToBottom()
      return
    }
    if (!anchor) return
    const row = findAnchor(el, anchor, rows.value)
    if (!row) return
    el.scrollTop += measured(row).getBoundingClientRect().top - el.getBoundingClientRect().top - anchor.offset
  })
}

/** The announcement of new messages that are not session notices. */
function newMessagesText(added: ChannelMessageView[]): string {
  const only = added.length === 1 ? added[0]! : undefined
  return strings.channelNewMessages({
    count: added.length,
    type: only?.type ? (typeByName.value.get(only.type)?.label ?? only.type) : undefined,
    author: only?.author?.name,
  })
}

function announce(next: ChannelMessageView[], prev: ChannelMessageView[]) {
  if (prev.length === 0) return
  const quiet = quietOn.value
  const older = quiet && !jumping ? next.filter((m) => m.seq < prev[0]!.seq) : []
  if (older.length && older.every((m) => !shownAs(rows.value, m.seq))) {
    announcement.value = strings.channelOlderHidden({ count: older.length })
    return
  }
  const last = prev[prev.length - 1]!.seq
  const fresh = next.filter((m) => m.seq > last && !m.removed)
  const added = quiet ? fresh.filter((m) => shownAs(rows.value, m.seq) === 'full') : fresh
  const oldest = next[0]?.seq ?? Infinity
  for (const seq of silenced) if (seq < oldest) silenced.delete(seq)
  if (quiet) for (const m of fresh) if (!added.includes(m)) silenced.add(m.seq)
  if (added.length === 0) return
  const others = added.filter((m) => !m.notice)
  const notices = added.flatMap((m) =>
    m.notice
      ? [
          m.notice.reentry
            ? strings.channelSessionReentered({ state: m.notice.toLabel })
            : strings.channelSessionMoved({ from: m.notice.fromLabel, to: m.notice.toLabel }),
        ]
      : [],
  )
  const messages = others.length > 0 ? newMessagesText(others) : undefined
  announcement.value = notices.length ? strings.channelNewNotices({ notices, messages }) : messages!
}

watch(
  () => props.messages,
  (next, prev) => {
    announce(next, prev)
    keepView()
  },
  { flush: 'pre' },
)

/**
 * The anchor for the next row change: a fold line the reader toggled, or
 * `skip` when `reveal` scrolls by itself.
 */
let rowChangeAnchor: Anchor | 'skip' | undefined

watch([showAll, expanded, revealed, () => props.presentation], () => {
  const given = rowChangeAnchor
  rowChangeAnchor = undefined
  if (given !== 'skip') keepView(given)
})

function toggleFold(row: ChannelFoldRow) {
  const el = log.value
  const button = el?.querySelector<HTMLElement>(`[data-fold-seq="${row.last}"]`)
  stuck.value = false
  if (el && button) {
    const offset = measured(button).getBoundingClientRect().top - el.getBoundingClientRect().top
    rowChangeAnchor = { seq: row.last, first: row.first, fold: true, offset }
  }
  const next = new Set(expanded.value)
  for (const s of row.seqs) {
    if (row.expanded) next.delete(s)
    else next.add(s)
  }
  expanded.value = next
}

/**
 * Follows the bottom when the log gets its first height (a hidden tab shown)
 * or a message grows (an edit, an action state), while the reader is there.
 */
let observer: ResizeObserver | undefined

onMounted(() => {
  scrollToBottom()
  if (typeof ResizeObserver === 'undefined') return
  observer = new ResizeObserver(() => {
    if (stuck.value) scrollToBottom()
  })
  if (log.value) observer.observe(log.value)
  if (list.value) observer.observe(list.value)
})

watch(list, (el, old) => {
  if (old) observer?.unobserve(old)
  if (el) observer?.observe(el)
})

onBeforeUnmount(() => observer?.disconnect())

/**
 * Shows the held message with seq, scrolls to it and focuses it: a hidden
 * or folded message is kept in full, or its run expanded. False when the
 * message is not held.
 */
function reveal(seq: number): boolean {
  if (!props.messages.some((m) => m.seq === seq)) return false
  jumping = false
  const row = rowFor(rows.value, seq)
  const shown = shownAs(rows.value, seq)
  if (shown !== 'full') {
    rowChangeAnchor = 'skip'
    stuck.value = false
    if (shown === 'folded' && row?.kind === 'fold') expanded.value = new Set([...expanded.value, ...row.seqs])
    else revealed.value = new Set([...revealed.value, seq])
  }
  void nextTick(() => {
    const target = log.value?.querySelector<HTMLElement>(`[data-seq="${seq}"]`)
    target?.scrollIntoView({ block: 'center' })
    target?.focus({ preventScroll: true })
  })
  return true
}

/**
 * Set while the app loads older pages for a reply quote's jump, so a page of
 * hidden messages is not announced just before `reveal` shows one of them.
 * Cleared by `reveal` and by a click on Load older.
 */
let jumping = false

/**
 * Reveals the held message with seq, or asks the app for older pages with
 * `jump` and stays quiet about them until `reveal` shows the message.
 */
function jumpTo(seq: number) {
  if (reveal(seq)) return
  jumping = true
  emit('jump', seq)
}

function loadOlder() {
  jumping = false
  emit('loadOlder')
}

defineExpose({
  reveal,
  jumpTo,
  /** Moves focus to the row with seq, without scrolling; false when it is not shown. */
  focusRow(seq: number): boolean {
    const target = log.value?.querySelector<HTMLElement>(`[data-seq="${seq}"]`)
    if (!target) return false
    target.focus({ preventScroll: true })
    return true
  },
})
</script>

<template>
  <section class="rl-channel" :aria-labelledby="titleId">
    <header class="rl-channel__head">
      <div class="rl-channel__title">
        <RlHeading :id="titleId" :level="headingLevel" size="lg" weight="semibold" truncate>
          {{ title }}
        </RlHeading>
        <RlTag v-if="state !== 'open'" :label="t(state)" color="grey" />
        <slot name="header" />
      </div>
      <RlButton v-if="hasQuiet" size="sm" variant="ghost" :aria-pressed="showAll" @click="showAll = !showAll">
        {{ t('showAllMessages') }}
      </RlButton>
      <RlButtonGroup v-if="lifecycleButtons.length" class="rl-channel__lifecycle">
        <RlButton
          v-for="op in lifecycleButtons"
          :key="op.name"
          size="sm"
          variant="secondary"
          :tone="op.danger ? 'danger' : 'default'"
          :loading="pending?.has(lifecyclePendingKey(op.name))"
          :pending-label="t('working')"
          @click="emit('lifecycle', op.name)"
        >
          {{ op.label }}
        </RlButton>
      </RlButtonGroup>
    </header>

    <slot name="session" />

    <div
      ref="log"
      class="rl-channel__log"
      role="log"
      aria-live="off"
      :aria-label="t('messages')"
      :aria-busy="loadingOlder || undefined"
      @scroll.passive="onScroll"
    >
      <div v-if="hasOlder" class="rl-channel__more">
        <RlButton size="sm" :loading="loadingOlder" :pending-label="t('loading')" @click="loadOlder">
          {{ t('loadOlder') }}
        </RlButton>
      </div>
      <p v-if="messages.length === 0" class="rl-channel__empty">{{ t('noMessages') }}</p>
      <p v-else-if="rows.length === 0" class="rl-channel__empty">
        {{ t(hasOlder ? 'onlyHiddenOlder' : 'onlyHidden') }}
      </p>
      <ol v-else ref="list" class="rl-channel__messages">
        <template v-for="row in rows" :key="row.kind === 'fold' ? row.key : row.message.seq">
          <li v-if="row.kind === 'message'">
            <RlChannelMessage v-bind="messageBind(row.message)" v-on="messageOn" />
          </li>
          <li v-else :data-fold-seq="row.last" :data-fold-first="row.first">
            <RlChannelFold
              :count="row.messages.length"
              :labels="row.labels"
              :expanded="row.expanded"
              :controls="`${foldId}-${row.first}`"
              @toggle="toggleFold(row)"
            />
            <ol v-if="row.expanded" :id="`${foldId}-${row.first}`" class="rl-channel__messages">
              <li v-for="m in row.messages" :key="m.seq">
                <RlChannelMessage v-bind="messageBind(m)" v-on="messageOn" />
              </li>
            </ol>
          </li>
        </template>
      </ol>
      <div v-if="hasNewer" class="rl-channel__more">
        <RlButton size="sm" @click="emit('loadNewest')">{{ t('jumpToLatest') }}</RlButton>
      </div>
    </div>
    <p class="rl-visually-hidden" aria-live="polite" aria-atomic="true">{{ announcement }}</p>

    <template v-if="state === 'open'">
      <div class="rl-channel__ephemeral" aria-live="polite">
        <span v-for="e in signals" :key="e.id" class="rl-channel__signal">
          <RlChannelAuthor :author="e.author" compact />
          <span>{{ signalText(e) }}</span>
        </span>
      </div>
      <p
        v-if="typingOffered && $slots.composer"
        class="rl-channel__typing"
        aria-live="polite"
        aria-atomic="true"
      >
        {{ typingText }}
      </p>
      <div v-if="$slots.composer" class="rl-channel__composer">
        <slot name="composer" :can-reply="canReply" />
      </div>
    </template>
    <p v-else class="rl-channel__readonly">
      <RlIcon :name="state === 'archived' ? 'archive' : 'lock'" :size="14" />
      {{ t(state === 'archived' ? 'archivedNotice' : 'closedNotice') }}
    </p>
  </section>
</template>

<style scoped>
.rl-channel {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  height: 100%;
  background: var(--rl-color-bg);
  color: var(--rl-color-text);
}

.rl-channel__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--rl-space-3);
  padding: var(--rl-space-3) var(--rl-space-5);
  border-bottom: 1px solid var(--rl-color-border);
}

.rl-channel__title {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  min-width: 0;
}

.rl-channel__title > .rl-heading { min-width: 0; }


.rl-channel__log {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--rl-space-2) 0;
}

.rl-channel__messages {
  margin: 0;
  padding: 0;
  list-style: none;
}

.rl-channel__more {
  display: flex;
  justify-content: center;
  padding: var(--rl-space-2);
}

.rl-channel__empty {
  margin: 0;
  padding: var(--rl-space-6) var(--rl-space-5);
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-sm);
  text-align: center;
}

.rl-channel__ephemeral {
  display: flex;
  flex-wrap: wrap;
  gap: 0 var(--rl-space-4);
  min-height: 24px;
  padding: var(--rl-space-1) var(--rl-space-5);
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
}

.rl-channel__typing {
  min-height: 1lh;
  margin: 0;
  padding: 0 var(--rl-space-5);
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-xs);
  font-style: italic;
}

.rl-channel__signal {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
  min-width: 0;
}

.rl-channel__composer { padding: 0 var(--rl-space-5) var(--rl-space-4); }

.rl-channel__readonly {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  margin: var(--rl-space-2) var(--rl-space-5) var(--rl-space-4);
  padding: var(--rl-space-3);
  border: 1px dashed var(--rl-color-border-strong);
  border-radius: var(--rl-radius-lg);
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-sm);
}
</style>
