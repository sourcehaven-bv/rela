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
 * The log opens at the newest message and follows new ones only while the
 * reader is already at the bottom, so reading back is never yanked away.
 * Loading older messages keeps the message under the reader in place. New
 * messages are announced through the log role, which is a polite live
 * region.
 */
import { computed, nextTick, onMounted, useTemplateRef, watch } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlButtonGroup from '../common/RlButtonGroup.vue'
import RlIcon from '../common/RlIcon.vue'
import RlTag from '../common/RlTag.vue'
import RlChannelAuthor from './RlChannelAuthor.vue'
import RlChannelMessage from './RlChannelMessage.vue'
import { lifecyclePendingKey } from './channelFields'
import type {
  ChannelEnumOption,
  ChannelEphemeralView,
  ChannelLifecycleDef,
  ChannelMessageTypeDef,
  ChannelMessageView,
  ChannelPresentation,
  ChannelPropValue,
  ChannelReadableRef,
  ChannelState,
} from './types'

const props = withDefaults(
  defineProps<{
    title: string
    state: ChannelState
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
    presentation?: ChannelPresentation
    enumOptions?: Record<string, ChannelEnumOption[]>
    entityHref?: (ref: ChannelReadableRef) => string | undefined
    /** Operations in flight, by `messagePendingKey` and `lifecyclePendingKey`. */
    pending?: ReadonlySet<string>
    /** Older messages exist than the oldest shown. */
    hasOlder?: boolean
    /** Newer messages exist than the newest shown (after reading far back). */
    hasNewer?: boolean
    loadingOlder?: boolean
  }>(),
  {
    postable: () => [],
    lifecycle: () => [],
    allowedLifecycle: () => [],
    ephemeral: () => [],
    hasOlder: false,
    hasNewer: false,
    loadingOlder: false,
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

const log = useTemplateRef<HTMLElement>('log')

const typeByName = computed(() => new Map(props.messageTypes.map((t) => [t.name, t])))

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
  const label = typeByName.value.get(e.type)?.label ?? e.type
  const parts = Object.values(e.props ?? {})
    .filter((v): v is Exclude<ChannelPropValue, object> => typeof v !== 'object')
    .map(String)
  return [label, ...parts].join(' · ')
}

function atBottom(el: HTMLElement): boolean {
  return el.scrollHeight - el.scrollTop - el.clientHeight < 24
}

function scrollToBottom() {
  if (log.value) log.value.scrollTop = log.value.scrollHeight
}

onMounted(scrollToBottom)

watch(
  () => props.messages,
  (next, prev) => {
    const el = log.value
    if (!el) return
    const before = { height: el.scrollHeight, top: el.scrollTop, bottom: atBottom(el) }
    const prepended = prev.length > 0 && next.length > 0 && next[0]!.seq < prev[0]!.seq
    const appended =
      next.length > 0 && (prev.length === 0 || next[next.length - 1]!.seq > prev[prev.length - 1]!.seq)
    void nextTick(() => {
      if (prepended) el.scrollTop = before.top + (el.scrollHeight - before.height)
      else if (appended && before.bottom) scrollToBottom()
    })
  },
  { flush: 'pre' },
)

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
  <section class="rl-channel" :aria-label="title">
    <header class="rl-channel__head">
      <div class="rl-channel__title">
        <h2>{{ title }}</h2>
        <RlTag v-if="state !== 'open'" :label="state === 'closed' ? 'Closed' : 'Archived'" color="grey" />
        <slot name="header" />
      </div>
      <RlButtonGroup v-if="lifecycleButtons.length" class="rl-channel__lifecycle">
        <RlButton
          v-for="op in lifecycleButtons"
          :key="op.name"
          size="sm"
          variant="secondary"
          :tone="op.name === 'delete' ? 'danger' : 'default'"
          :loading="pending?.has(lifecyclePendingKey(op.name))"
          pending-label="Working…"
          @click="emit('lifecycle', op.name)"
        >
          {{ op.label }}
        </RlButton>
      </RlButtonGroup>
    </header>

    <div ref="log" class="rl-channel__log" role="log" aria-label="Messages" :aria-busy="loadingOlder || undefined">
      <div v-if="hasOlder" class="rl-channel__more">
        <RlButton size="sm" :loading="loadingOlder" pending-label="Loading…" @click="emit('loadOlder')">
          Load older messages
        </RlButton>
      </div>
      <p v-if="messages.length === 0" class="rl-channel__empty">No messages yet.</p>
      <ol v-else class="rl-channel__messages">
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
        <RlButton size="sm" @click="emit('loadNewest')">Jump to latest</RlButton>
      </div>
    </div>

    <template v-if="state === 'open'">
      <div class="rl-channel__ephemeral" aria-live="polite">
        <span v-for="e in ephemeral" :key="e.id" class="rl-channel__signal">
          <RlChannelAuthor :author="e.author" compact />
          <span>{{ ephemeralText(e) }}</span>
        </span>
      </div>
      <div v-if="$slots.composer" class="rl-channel__composer">
        <slot name="composer" :can-reply="canReply" />
      </div>
    </template>
    <p v-else class="rl-channel__readonly">
      <RlIcon :name="state === 'archived' ? 'archive' : 'lock'" :size="14" />
      {{
        state === 'archived'
          ? 'This channel is archived. It can be read but takes no new messages.'
          : 'This channel is closed. It can be read but takes no new messages.'
      }}
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

.rl-channel__title h2 {
  margin: 0;
  overflow: hidden;
  font-size: var(--rl-font-size-lg);
  font-weight: var(--rl-font-weight-semibold);
  text-overflow: ellipsis;
  white-space: nowrap;
}

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
