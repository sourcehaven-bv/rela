<script setup lang="ts">
/**
 * One message in a channel.
 *
 * A message whose type is a single markdown property reads as prose. Any
 * other type is a card: the type's label, then its properties in
 * presentation order (see `resolveChannelFields`), then the action state.
 *
 * Every control follows `message.actions`, the list the server computed for
 * this user and this message. A control it does not name is not rendered,
 * not disabled: the user may not use it, and a greyed button would only
 * advertise that. Delete (own message, no reason) and Remove (moderator) are
 * separate controls because they are separate rights.
 *
 * `decisions` are the session decisions shown on this row, the message
 * that moved the session into its current state; the app passes them to
 * that row only. They render before Reply, as a group described by the
 * card's type label and the author line, and emit `decide`. A notice or a
 * removed message never shows them.
 *
 * `choices` are the reply buttons of a question's options, which the app
 * builds only while the question is open to an answer. They render after
 * the fields, as a group named "Choices", and emit `choose`. While they
 * show, the list property they come from is not rendered as well. A list
 * property renders as a bulleted list, each item as text.
 *
 * A session notice (`message.notice`) renders as one system line: the move,
 * and the time. It has no controls, not even reply: the server sends no
 * actions on a notice, and the message that caused it is the row above,
 * written in the same append.
 *
 * Text is text. Titles, names, reasons and string properties render through
 * interpolation. The one exception is `message.html`: markdown the app has
 * already rendered and sanitized. It is inserted as it is; this component
 * never renders or cleans markdown, so the app's sanitizer is the only one.
 */
import { computed, useId } from 'vue'
import RlButton from '../common/RlButton.vue'
import RlButtonGroup from '../common/RlButtonGroup.vue'
import RlIcon from '../common/RlIcon.vue'
import RlTag from '../common/RlTag.vue'
import RlChannelAuthor from './RlChannelAuthor.vue'
import RlChannelDecisions from './RlChannelDecisions.vue'
import RlRefChip from './RlRefChip.vue'
import {
  CHANNEL_OPERATIONS,
  resolveChannelFields,
  messagePendingKey,
  type ResolvedChannelField,
} from './channelFields'
import type {
  ChannelDecisionView,
  ChannelEnumOption,
  ChannelMessagePresentation,
  ChannelMessageTypeDef,
  ChannelMessageView,
  ChannelPropValue,
  ChannelReadableRef,
  ChannelRef,
} from './types'
import { useMessages, type ChannelTextKey } from '../../composables/useMessages'

const props = withDefaults(
  defineProps<{
    message: ChannelMessageView
    /** The message's type. Without it the properties show by name. */
    messageType?: ChannelMessageTypeDef
    presentation?: ChannelMessagePresentation
    /** Enum labels and colours, keyed by enum type. */
    enumOptions?: Record<string, ChannelEnumOption[]>
    /** The link for a readable ref; without it a chip is a button. */
    entityHref?: (ref: ChannelReadableRef) => string | undefined
    /** Operations in flight, by `messagePendingKey`. */
    pending?: ReadonlySet<string>
    /** Whether the user may post a reply in this channel now. */
    canReply?: boolean
    /** The session decisions this row shows; see the header comment. */
    decisions?: ChannelDecisionView[]
    /** The reply buttons of this question's options; see the header comment. */
    choices?: ChannelDecisionView[]
  }>(),
  { canReply: false },
)

const emit = defineEmits<{
  reply: [seq: number]
  edit: [seq: number]
  delete: [seq: number]
  remove: [seq: number]
  release: [seq: number]
  action: [seq: number, name: string]
  openRef: [ref: ChannelReadableRef, event: MouseEvent]
  jump: [seq: number]
  decide: [key: string]
  choose: [key: string]
}>()

const headId = useId()
const metaId = useId()

const messages = useMessages()
const t = (key: ChannelTextKey) => messages.channelText({ key })

const allowed = computed(() => new Set(props.message.actions ?? []))

const fields = computed<ResolvedChannelField[]>(() => {
  if (props.messageType) return resolveChannelFields(props.messageType, props.presentation)
  // An unknown type (the descriptor is reloading): show what arrived, by name.
  return Object.keys(props.message.props ?? {})
    .sort()
    .map((name) => ({ property: { name, label: name, type: 'string' }, label: name, widget: 'text' }))
})

const choiceList = computed(() => props.choices ?? [])

/** The list property the choices come from, while they show. */
const choicesFrom = computed(() =>
  choiceList.value.length ? props.messageType?.choices?.from : undefined,
)

const shown = computed(() =>
  fields.value.filter(
    (f) => props.message.props?.[f.property.name] !== undefined && f.property.name !== choicesFrom.value,
  ),
)

/** One markdown property and no actions: show it as plain prose, without a card. */
const prose = computed(
  () =>
    !!props.messageType &&
    fields.value.length === 1 &&
    fields.value[0]!.property.type === 'markdown' &&
    props.messageType.actions.length === 0,
)

const actionButtons = computed(() =>
  (props.messageType?.actions ?? []).filter((a) => allowed.value.has(a.name)),
)

const actionLabel = computed(() => {
  const name = props.message.action?.name
  if (!name) return ''
  return props.messageType?.actions.find((a) => a.name === name)?.label ?? name
})

const decisionList = computed(() => props.decisions ?? [])

/**
 * The ids that describe the decision group: the card's type label, if shown,
 * and the author line. `!prose && messageType` must match the template's
 * branch that renders the card head with `headId`; keep the two in step.
 */
const decisionsDescribedBy = computed(() =>
  [!prose.value && props.messageType ? headId : undefined, metaId].filter(Boolean).join(' '),
)

const hasControls = computed(
  () =>
    decisionList.value.length > 0 ||
    props.canReply ||
    actionButtons.value.length > 0 ||
    [...CHANNEL_OPERATIONS].some((op) => allowed.value.has(op)),
)

const noticeText = computed(() => {
  const n = props.message.notice
  if (!n) return ''
  return n.reentry
    ? messages.channelSessionReentered({ state: n.toLabel })
    : messages.channelSessionMoved({ from: n.fromLabel, to: n.toLabel })
})

function isPending(op: string): boolean {
  return props.pending?.has(messagePendingKey(props.message.seq, op)) ?? false
}

function isRef(v: ChannelPropValue): v is ChannelRef {
  return !Array.isArray(v) && typeof v === 'object' && v !== null
}

function enumOption(f: ResolvedChannelField, v: ChannelPropValue): ChannelEnumOption | undefined {
  const opts = f.property.enum ? props.enumOptions?.[f.property.enum] : undefined
  return opts?.find((o) => o.value === v)
}

function plain(f: ResolvedChannelField, v: ChannelPropValue): string {
  const text = props.message.text?.[f.property.name]
  if (text !== undefined) return text
  if (typeof v === 'boolean') return t(v ? 'yes' : 'no')
  if (Array.isArray(v)) return v.join(', ')
  return String(v)
}

function hrefFor(v: ChannelRef): string | undefined {
  return v.restricted ? undefined : props.entityHref?.(v)
}
</script>

<template>
  <div class="rl-channel-message" :data-seq="message.seq" tabindex="-1">
    <p v-if="message.removed" class="rl-channel-message__removed">
      <RlIcon name="erase" :size="14" />
      <span>{{ messages.channelMessageRemoved(message.removed) }}</span>
      <span v-if="message.removed.at" class="rl-channel-message__time">{{ message.removed.at }}</span>
    </p>

    <p v-else-if="message.notice" class="rl-channel-message__notice">
      <RlIcon name="workflow" :size="14" />
      <span>{{ noticeText }}</span>
      <time v-if="message.at" class="rl-channel-message__time" :datetime="message.datetime">{{ message.at }}</time>
    </p>

    <template v-else>
      <div :id="metaId" class="rl-channel-message__meta">
        <RlChannelAuthor v-if="message.author" :author="message.author" />
        <time v-if="message.at" class="rl-channel-message__time" :datetime="message.datetime">
          {{ message.at }}
        </time>
        <span v-if="message.editedAt" class="rl-channel-message__time" :title="message.editedAt">{{ t('edited') }}</span>
      </div>

      <button
        v-if="message.replyTo"
        type="button"
        class="rl-channel-message__quote"
        @click="emit('jump', message.replyTo.seq)"
      >
        <span class="rl-channel-message__quote-who">
          {{ messages.channelReplyTo({ author: message.replyTo.author }) }}
        </span>
        <span v-if="message.replyTo.excerpt" class="rl-channel-message__quote-text">
          {{ message.replyTo.excerpt }}
        </span>
      </button>

      <template v-if="prose">
        <!-- Sanitized by the app before it reaches this component; see the header comment. -->
        <div
          v-if="message.html?.[fields[0]!.property.name] !== undefined"
          class="rl-channel-message__body rl-channel-message__md"
          v-html="message.html[fields[0]!.property.name]"
        />
        <p v-else-if="shown.length" class="rl-channel-message__body rl-channel-message__plain">
          {{ plain(shown[0]!, message.props![shown[0]!.property.name]!) }}
        </p>
      </template>

      <div v-else-if="messageType || shown.length || message.action" class="rl-channel-message__card">
        <div v-if="messageType" :id="headId" class="rl-channel-message__card-head">
          {{ messageType.label }}
        </div>
        <dl v-if="shown.length" class="rl-channel-message__fields">
          <template v-for="f in shown" :key="f.property.name">
            <dt>{{ f.label }}</dt>
            <dd>
              <ul v-if="Array.isArray(message.props![f.property.name])" class="rl-channel-message__list">
                <li v-for="(item, i) in message.props![f.property.name] as string[]" :key="i">{{ item }}</li>
              </ul>
              <RlRefChip
                v-else-if="isRef(message.props![f.property.name]!)"
                :value="message.props![f.property.name] as ChannelRef"
                :href="hrefFor(message.props![f.property.name] as ChannelRef)"
                @open="(r, e) => emit('openRef', r, e)"
              />
              <RlTag
                v-else-if="f.property.type === 'enum'"
                :label="enumOption(f, message.props![f.property.name]!)?.label ?? plain(f, message.props![f.property.name]!)"
                :color="enumOption(f, message.props![f.property.name]!)?.color ?? 'grey'"
              />
              <!-- Sanitized by the app before it reaches this component; see the header comment. -->
              <div
                v-else-if="f.property.type === 'markdown' && message.html?.[f.property.name] !== undefined"
                class="rl-channel-message__md"
                v-html="message.html[f.property.name]"
              />
              <span v-else class="rl-channel-message__plain">{{ plain(f, message.props![f.property.name]!) }}</span>
            </dd>
          </template>
        </dl>

        <div v-if="message.action" class="rl-channel-message__state">
          <RlTag
            :label="
              message.action.status === 'pending'
                ? messages.channelActionPending({ label: actionLabel })
                : messages.channelActionDone({ label: actionLabel })
            "
            :color="message.action.status === 'pending' ? 'amber' : 'green'"
          />
          <template v-if="message.action.promoted">
            <RlIcon name="arrow-right" :size="13" />
            <RlRefChip
              :value="message.action.promoted"
              :href="hrefFor(message.action.promoted)"
              @open="(r, e) => emit('openRef', r, e)"
            />
          </template>
        </div>
      </div>

      <RlChannelDecisions
        v-if="choiceList.length"
        kind="choice"
        class="rl-channel-message__choices"
        :decisions="choiceList"
        :pending="pending"
        :described-by="decisionsDescribedBy"
        @decide="(key) => emit('choose', key)"
      />

      <RlButtonGroup v-if="hasControls" align="start" class="rl-channel-message__controls">
        <RlChannelDecisions
          :decisions="decisionList"
          :pending="pending"
          :described-by="decisionsDescribedBy"
          @decide="(key) => emit('decide', key)"
        />
        <RlButton
          v-for="a in actionButtons"
          :key="a.name"
          :variant="a.promote_to ? 'primary' : 'secondary'"
          size="sm"
          :loading="isPending(a.name)"
          :pending-label="t('working')"
          @click="emit('action', message.seq, a.name)"
        >
          {{ a.label }}
        </RlButton>
        <RlButton
          v-if="allowed.has('release')"
          variant="secondary"
          size="sm"
          :loading="isPending('release')"
          :pending-label="t('releasing')"
          @click="emit('release', message.seq)"
        >
          {{ t('release') }}
        </RlButton>
        <RlButton
          v-if="canReply"
          variant="ghost"
          size="sm"
          @click="emit('reply', message.seq)"
        >
          {{ t('reply') }}
        </RlButton>
        <RlButton v-if="allowed.has('edit')" variant="ghost" size="sm" @click="emit('edit', message.seq)">
          {{ t('edit') }}
        </RlButton>
        <RlButton
          v-if="allowed.has('delete')"
          variant="ghost"
          size="sm"
          tone="danger"
          :loading="isPending('delete')"
          :pending-label="t('deleting')"
          @click="emit('delete', message.seq)"
        >
          {{ t('delete') }}
        </RlButton>
        <RlButton
          v-if="allowed.has('remove')"
          variant="ghost"
          size="sm"
          tone="danger"
          :loading="isPending('remove')"
          :pending-label="t('removing')"
          @click="emit('remove', message.seq)"
        >
          {{ t('remove') }}
        </RlButton>
      </RlButtonGroup>
    </template>
  </div>
</template>

<style scoped>
.rl-channel-message {
  padding: var(--rl-space-3) var(--rl-space-5);
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text);
}

.rl-channel-message:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

.rl-channel-message__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-2);
}

.rl-channel-message__time {
  color: var(--rl-color-text-subtle);
  font-size: var(--rl-font-size-xs);
}

.rl-channel-message__removed {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  margin: 0;
  color: var(--rl-color-text-muted);
  font-style: italic;
}

.rl-channel-message__notice {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-2);
  margin: 0;
  color: var(--rl-color-text-muted);
}

.rl-channel-message__quote {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-width: 560px;
  margin: var(--rl-space-2) 0 0;
  padding: var(--rl-space-1) var(--rl-space-3);
  border: 0;
  border-left: 3px solid var(--rl-color-border-strong);
  background: var(--rl-color-bg-sunken);
  color: var(--rl-color-text-muted);
  font: inherit;
  font-size: var(--rl-font-size-xs);
  text-align: left;
  cursor: pointer;
}

.rl-channel-message__quote:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}

.rl-channel-message__quote-who { font-weight: var(--rl-font-weight-medium); }

.rl-channel-message__quote-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-channel-message__body { margin: var(--rl-space-2) 0 0; }

/* Author-written text: keep their line breaks and break a pasted URL. */
.rl-channel-message__plain {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.rl-channel-message__md {
  line-height: var(--rl-line-height-relaxed);
  overflow-wrap: anywhere;
}

.rl-channel-message__md :deep(p) { margin: 0 0 var(--rl-space-2); }
.rl-channel-message__md :deep(p:last-child) { margin-bottom: 0; }

.rl-channel-message__md :deep(code) {
  padding: 0 var(--rl-space-1);
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg-hover);
  font-family: var(--rl-font-family-mono);
  font-size: 0.9em;
}

/* A job or login the message mentions; the app's renderer builds the span. */
.rl-channel-message__md :deep(.mention) {
  padding: 0 var(--rl-space-1);
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg-selected);
  color: var(--rl-color-info-fg);
  font-weight: var(--rl-font-weight-medium);
  white-space: nowrap;
}

.rl-channel-message__card {
  max-width: 560px;
  margin-top: var(--rl-space-2);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  overflow: hidden;
}

.rl-channel-message__card-head {
  padding: var(--rl-space-2) var(--rl-space-3);
  border-bottom: 1px solid var(--rl-color-border);
  background: var(--rl-color-bg-sunken);
  font-weight: var(--rl-font-weight-semibold);
}

.rl-channel-message__fields {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: var(--rl-space-2) var(--rl-space-4);
  margin: 0;
  padding: var(--rl-space-3);
}

.rl-channel-message__fields dt { color: var(--rl-color-text-muted); }

.rl-channel-message__fields dd {
  min-width: 0;
  margin: 0;
}

.rl-channel-message__state {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-2) var(--rl-space-3);
  border-top: 1px solid var(--rl-color-border);
}

.rl-channel-message__list {
  margin: 0;
  padding-left: var(--rl-space-5);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.rl-channel-message__choices {
  margin-top: var(--rl-space-2);
}

.rl-channel-message__controls {
  flex-wrap: wrap;
  margin-top: var(--rl-space-2);
}
</style>
