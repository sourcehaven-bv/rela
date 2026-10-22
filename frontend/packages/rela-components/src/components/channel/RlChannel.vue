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
 * so instead.
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
 *
 * `typing` shows who else is typing as one line, in its own polite, atomic
 * live region apart from the signals, so it is read out only when the line
 * changes. It names at most two typists and counts the rest. The line is
 * there only when `typingOffered` is set and the channel is open with a
 * composer; it stays mounted while empty, so the live region exists before
 * its first text.
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId, useTemplateRef, watch } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlButtonGroup from '../common/RlButtonGroup.vue'
import RlHeading from '../common/RlHeading.vue'
import RlIcon from '../common/RlIcon.vue'
import RlTag from '../common/RlTag.vue'
import RlChannelAuthor from './RlChannelAuthor.vue'
import RlChannelMessage from './RlChannelMessage.vue'
import { lifecyclePendingKey, resolveChannelFields } from './channelFields'
import type {
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
}>()

const strings = useMessages()
const t = (key: ChannelTextKey) => strings.channelText({ key })
const titleId = useId()

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

function ephemeralText(e: ChannelEphemeralView): string {
  const type = typeByName.value.get(e.type)
  const label = type?.label ?? e.type
  if (e.text !== undefined) return e.text ? `${label} · ${e.text}` : label
  const values = e.props ?? {}
  const names = type
    ? resolveChannelFields(type, props.presentation?.messages?.[type.name]).map((f) => f.property)
    : Object.keys(values).map((name) => ({ name, enum: undefined }))
  const parts: string[] = []
  for (const p of names) {
    const v = values[p.name]
    if (v === undefined) continue
    if (typeof v === 'object') {
      parts.push(v.restricted ? t('restrictedRef') : (v.title ?? v.id))
    } else if (typeof v === 'boolean') {
      parts.push(t(v ? 'yes' : 'no'))
    } else {
      const opts = p.enum ? props.enumOptions?.[p.enum] : undefined
      parts.push(opts?.find((o) => o.value === v)?.label ?? String(v))
    }
  }
  return [label, ...parts].join(' · ')
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

/** The first message whose bottom is below the top of the log, and its offset. */
function firstVisible(el: HTMLElement): { seq: string; offset: number } | null {
  const top = el.getBoundingClientRect().top
  for (const row of el.querySelectorAll<HTMLElement>('[data-seq]')) {
    const r = row.getBoundingClientRect()
    if (r.bottom > top) return { seq: row.dataset.seq!, offset: r.top - top }
  }
  return null
}

const announcement = ref('')

function announce(next: ChannelMessageView[], prev: ChannelMessageView[]) {
  if (prev.length === 0) return
  const last = prev[prev.length - 1]!.seq
  const added = next.filter((m) => m.seq > last && !m.removed)
  if (added.length === 0) return
  const only = added.length === 1 ? added[0]! : undefined
  announcement.value = strings.channelNewMessages({
    count: added.length,
    type: only?.type ? (typeByName.value.get(only.type)?.label ?? only.type) : undefined,
    author: only?.author?.name,
  })
}

watch(
  () => props.messages,
  (next, prev) => {
    announce(next, prev)
    const el = log.value
    if (!el) return
    const follow = stuck.value
    const anchor = follow ? null : firstVisible(el)
    void nextTick(() => {
      if (follow) {
        scrollToBottom()
        return
      }
      if (!anchor) return
      const row = el.querySelector<HTMLElement>(`[data-seq="${anchor.seq}"]`)
      if (!row) return
      const offset = row.getBoundingClientRect().top - el.getBoundingClientRect().top
      el.scrollTop += offset - anchor.offset
    })
  },
  { flush: 'pre' },
)

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

function jumpTo(seq: number) {
  const target = log.value?.querySelector<HTMLElement>(`[data-seq="${seq}"]`)
  if (!target) {
    emit('jump', seq)
    return
  }
  target.scrollIntoView({ block: 'center' })
  target.focus({ preventScroll: true })
}
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
        <RlButton size="sm" :loading="loadingOlder" :pending-label="t('loading')" @click="emit('loadOlder')">
          {{ t('loadOlder') }}
        </RlButton>
      </div>
      <p v-if="messages.length === 0" class="rl-channel__empty">{{ t('noMessages') }}</p>
      <ol v-else ref="list" class="rl-channel__messages">
        <li v-for="m in messages" :key="m.seq">
          <RlChannelMessage
            :message="m"
            :message-type="m.type ? typeByName.get(m.type) : undefined"
            :presentation="m.type ? presentation?.messages?.[m.type] : undefined"
            :enum-options="enumOptions"
            :entity-href="entityHref"
            :pending="pending"
            :can-reply="canReply"
            @reply="(s) => emit('reply', s)"
            @edit="(s) => emit('edit', s)"
            @delete="(s) => emit('delete', s)"
            @remove="(s) => emit('remove', s)"
            @release="(s) => emit('release', s)"
            @action="(s, n) => emit('action', s, n)"
            @open-ref="(r, e) => emit('openRef', r, e)"
            @jump="jumpTo"
          />
        </li>
      </ol>
      <div v-if="hasNewer" class="rl-channel__more">
        <RlButton size="sm" @click="emit('loadNewest')">{{ t('jumpToLatest') }}</RlButton>
      </div>
    </div>
    <p class="rl-visually-hidden" aria-live="polite" aria-atomic="true">{{ announcement }}</p>

    <template v-if="state === 'open'">
      <div class="rl-channel__ephemeral" aria-live="polite">
        <span v-for="e in ephemeral" :key="e.id" class="rl-channel__signal">
          <RlChannelAuthor :author="e.author" compact />
          <span>{{ ephemeralText(e) }}</span>
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
