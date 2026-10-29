<script setup lang="ts">
/**
 * One open channel on an entity page (TKT-91BK6F): useChannel's state drawn
 * by the library's RlChannel and RlChannelComposer.
 *
 * The wire-to-view mapping is channelView.ts and the error handling is
 * channelErrors.ts; this component holds the session state between them:
 * which message is being replied to or edited, which operations are in
 * flight, the remove dialog and the ref picker. The ephemeral clock is
 * useExpiryClock, the paging useChannelPaging and the picker plumbing
 * useRefPicker. useTyping sends the composer's typing events; the head's
 * typing_now is shown as it is, with no clock, since the server expires
 * entries and frames the head then.
 * A channel type with a session gets the stepper in RlChannel's session
 * slot, and the composer marks the types whose post would move it.
 *
 * Session decisions (TKT-T9G1AE) are buttons in the stepper and on the
 * message that moved the session into its current state, and choices
 * (TKT-03FQPD) on a question's row; their logic is useSessionDecisions.
 *
 * It emits `gone` when the channel can no longer be shown: a refused read
 * (the reader lost access, or the channel was deleted) or a refusal that
 * names the channel as missing. useChannel has already dropped what it held
 * by then. `changed` asks the host to read the channel list again after a
 * lifecycle operation, since the list is not live while a channel is open.
 */
import { computed, ref, useTemplateRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useChannel } from '@/composables/useChannel'
import { useMarkRead } from '@/composables/useMarkRead'
import { useTyping } from '@/composables/useTyping'
import { useConfirm } from '@/composables/useConfirm'
import { useChannelSchemaStore } from '@/stores/channelSchema'
import { useSchemaStore, useUIStore } from '@/stores'
import { shouldDropHeldContent } from '@/api/errors'
import { browserTimeZone } from '@/utils/format'
import { toEditProps, toProps } from '@/utils/channelValues'
import {
  LIFECYCLE_DELETE,
  type ChannelTypeSchema,
  type Message,
  type MessagePropInput,
} from '@/types/channels'
import type {
  ChannelComposerTransition,
  ChannelEditEvent,
  ChannelEditing,
  ChannelPostEvent,
  ChannelSessionView,
} from 'rela-components/components/channel/types'
import {
  DECISIONS_BLOCKED_KEY,
  lifecyclePendingKey,
  messagePendingKey,
} from 'rela-components/components/channel/channelFields'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlBanner from 'rela-components/components/feedback/RlBanner.vue'
import RlChannel from 'rela-components/components/channel/RlChannel.vue'
import RlChannelComposer from 'rela-components/components/channel/RlChannelComposer.vue'
import RlChannelDecisions from 'rela-components/components/channel/RlChannelDecisions.vue'
import RlChannelSession from 'rela-components/components/channel/RlChannelSession.vue'
import EntityPickerModal from '@/components/forms/EntityPickerModal.vue'
import ChannelTextDialog from './ChannelTextDialog.vue'
import { classifyChannelError } from './channelErrors'
import { useChannelLinks } from './useChannelLinks'
import { useChannelPaging } from './useChannelPaging'
import { useExpiryClock } from './useExpiryClock'
import { usePendingKeys } from './usePendingKeys'
import { useRefPicker } from './useRefPicker'
import { useSessionDecisions } from './useSessionDecisions'
import {
  composerTransitions,
  editingFor,
  enumOptionsFor,
  ephemeralViews,
  lifecycleDefs,
  messageViews,
  presentationView,
  replyQuote,
  sessionView,
  typingViews,
  type MarkdownMemo,
} from './channelView'

const props = defineProps<{
  entityType: string
  /** The entity's address, face included on a faced type. */
  address: string
  channelType: ChannelTypeSchema
  /** Absent for a `one` channel. */
  name?: string
  /**
   * The page world, carried on entity links only. Channel routes take no
   * `?world=`: the server refuses a non-default world there, and the
   * address names the face.
   */
  world?: string
}>()

const emit = defineEmits<{ gone: []; changed: [] }>()

const { t } = useI18n()
const ui = useUIStore()
const schema = useSchemaStore()
const channelSchema = useChannelSchemaStore()
const { confirm } = useConfirm()

const tz = browserTimeZone()

const target = computed(() => ({
  type: props.entityType,
  address: props.address,
  channelType: props.channelType.name,
  ...(props.name ? { name: props.name } : {}),
}))

const hasEphemeral = computed(() =>
  props.channelType.message_types.some((mt) => mt.persist === 'ephemeral')
)

// typing_now lives in the head only, as signals do, so a typing type
// refetches the head on every frame too.
const ch = useChannel(target, {
  ephemeral: () => hasEphemeral.value || props.channelType.typing,
  onSeen: (head, messages) => channelSchema.checkNames(props.channelType.name, head, messages),
})

// The open channel marks itself read while the tab is visible. Nothing of
// it is shown.
useMarkRead({
  target,
  head: ch.head,
  messages: ch.messages,
  hasNewer: ch.hasNewer,
  takeRead: ch.takeRead,
})

const messageType = (name: string) => channelSchema.messageType(props.channelType.name, name)

// The rendered HTML per message revision (R19), dropped with the channel.
let memo: MarkdownMemo = new Map()

const presentation = computed(() =>
  presentationView(schema.channelPresentations.get(props.channelType.name))
)
const enumOptions = computed(() =>
  enumOptionsFor(props.channelType, schema.customTypes, schema.styles)
)
const ctx = computed(() => ({ tz, world: props.world, session: props.channelType.session }))
const views = computed(() => messageViews(ch.messages.value, messageType, ctx.value, memo))

// Ephemeral signals show until their expires_at; the head brings new ones.
const now = useExpiryClock(() => ch.head.value?.ephemeral)
const ephemeral = computed(() =>
  ephemeralViews(
    ch.head.value?.ephemeral ?? [],
    now.value,
    messageType,
    presentation.value,
    enumOptions.value,
    tz
  )
)

const typing = useTyping(target, () => ch.head.value?._actions.typing ?? false)
const typingNow = computed(() => typingViews(ch.head.value?.typing_now))
// The head is replaced on every frame. The stepper and the composer's
// transitions are rebuilt only when what they read changes: the descriptor,
// the session's generation and state, the channel state and the actions.
// Returning the previous entry keeps the same value, so nothing downstream
// re-renders or re-runs its watchers.
type Keyed<T> = { ct: ChannelTypeSchema; key: string; value: T }
const session = computed<Keyed<ChannelSessionView | undefined>>((prev) => {
  const h = ch.head.value
  const cs = h?.channel.session
  const answered = cs ? channelSchema.stateAnswered(props.channelType.name, cs.state) : false
  const key = JSON.stringify([
    !!h,
    cs?.generation,
    cs?.state,
    cs?.stranded,
    h?.channel.state,
    answered,
  ])
  if (prev && prev.ct === props.channelType && prev.key === key) return prev
  const value = h ? sessionView(props.channelType, h.channel, () => answered) : undefined
  return { ct: props.channelType, key, value }
})
const transitions = computed<Keyed<ChannelComposerTransition[]>>((prev) => {
  const h = ch.head.value
  const cs = h?.channel.session
  const key = JSON.stringify([
    cs?.generation,
    cs?.state,
    cs?.stranded,
    h?._actions.post,
    h?._actions.transitions,
  ])
  if (prev && prev.ct === props.channelType && prev.key === key) return prev
  const value = h ? composerTransitions(props.channelType, h) : []
  return { ct: props.channelType, key, value }
})

const title = computed(() => ch.head.value?.channel.title || props.name || props.channelType.label)

// A refused read means the channel is no longer there for this reader.
watch(ch.error, (err) => {
  if (err && !ch.head.value && shouldDropHeldContent(err)) emit('gone')
})

// --- Errors and operations in flight ---------------------------------------

const formError = ref<string>()
const fieldErrors = ref<Record<string, string>>({})

/** Answers a refusal; returns the message when it belongs in the form. */
function handle(err: unknown, hasForm: boolean): string | undefined {
  const out = classifyChannelError(err)
  if (out.then === 'refetch') void ch.refetch()
  else if (out.then === 'list') emit('gone')
  else if (out.then === 'schema') {
    void channelSchema.reload()
    void schema.reloadChannelPresentations()
  }
  if (out.place === 'form' && hasForm) return out.message
  ui.error(out.message)
  return undefined
}

const { pending, set: setPending, run: runPending } = usePendingKeys((err) => handle(err, false))

// --- Composer ----------------------------------------------------------------

const composer = useTemplateRef<InstanceType<typeof RlChannelComposer>>('composer')
const replyTo = ref<number | null>(null)
const editing = ref<ChannelEditing>()
// The message's stored props when the edit started. An edit replaces every
// prop (R7), and the held list may drop the message before the save (a
// reload, the 500 cap), so the save is built from this copy, never from
// the list at save time.
let editProps: { seq: number; props: Message['props'] } | undefined
const submitting = ref(false)
const sessionHeader = useTemplateRef<InstanceType<typeof RlChannelSession>>('sessionHeader')
const channelView = useTemplateRef<InstanceType<typeof RlChannel>>('channelView')

/**
 * The pending keys the library sees: a composer send, or a decision in
 * progress (a dialog open, a post in flight), blocks every decision. Only
 * the posting decision's own key shows it as sending.
 */
const shownPending = computed<ReadonlySet<string>>(() =>
  submitting.value || deciding.value
    ? new Set([...pending.value, DECISIONS_BLOCKED_KEY])
    : pending.value
)

const replyQuoteView = computed(() => {
  const seq = replyTo.value
  if (seq === null) return undefined
  const original = ch.messages.value.find((m) => m.seq === seq)
  return replyQuote(seq, original, original?.type ? messageType(original.type) : undefined)
})

function clearErrors() {
  formError.value = undefined
  fieldErrors.value = {}
}

function stopEditing() {
  editing.value = undefined
  editProps = undefined
}

// Another channel starts clean: no reply, edit or error carries over. The
// key is a string, so a descriptor reload (a new channelType object for the
// same channel) does not count as another channel.
const channelKey = () =>
  [props.entityType, props.address, props.channelType.name, props.name ?? ''].join('\u0000')
watch(channelKey, () => {
  memo = new Map()
  replyTo.value = null
  stopEditing()
  clearErrors()
  resetDecisions()
})

/**
 * Posts a message. A log post ends the typing entry on the server; a
 * typing call still in flight must land first, or it would bring the
 * entry back.
 */
async function postTyped(
  type: string,
  values: Record<string, MessagePropInput>,
  replyTo?: number,
  expectGeneration?: number
) {
  try {
    await typing.settle()
    return await ch.post(type, values, replyTo, expectGeneration)
  } finally {
    typing.reset()
  }
}

async function onPost(event: ChannelPostEvent) {
  const mt = messageType(event.type)
  if (!mt || submitting.value || deciding.value) return
  const rendered = new Set(event.rendered)
  const converted = toProps(
    mt.properties.filter((p) => rendered.has(p.name)),
    event.values
  )
  if (!converted.ok) {
    fieldErrors.value = converted.errors
    return
  }
  clearErrors()
  submitting.value = true
  try {
    await decisionsApi.sendComposer(event.type, converted.props, event.replyTo)
    composer.value?.reset(event)
    replyTo.value = null
  } catch (err) {
    formError.value = handle(err, true)
  } finally {
    submitting.value = false
  }
}

async function onSave(event: ChannelEditEvent) {
  const mt = messageType(event.type)
  if (!mt || submitting.value) return
  // Props the composer did not render are carried as stored, so an edit
  // never erases a field the user could not see (AC 11). Without the stored
  // copy the save would send a partial body, so it is refused instead.
  if (editProps?.seq !== event.seq) {
    formError.value = t('channels.editLost')
    return
  }
  const converted = toEditProps(
    mt.properties,
    new Set(event.rendered),
    event.values,
    editProps.props
  )
  if (!converted.ok) {
    fieldErrors.value = converted.errors
    return
  }
  clearErrors()
  submitting.value = true
  try {
    await ch.edit(event.seq, converted.props)
    stopEditing()
  } catch (err) {
    formError.value = handle(err, true)
  } finally {
    submitting.value = false
  }
}

function onReply(seq: number) {
  stopEditing()
  replyTo.value = seq
  composer.value?.focus()
}

function onEdit(seq: number) {
  const m = ch.messages.value.find((x) => x.seq === seq)
  const mt = m?.type ? messageType(m.type) : undefined
  if (!m || !mt) return
  replyTo.value = null
  clearErrors()
  editing.value = editingFor(m, mt)
  editProps = { seq, props: m.props }
  composer.value?.focus()
}

function onCancelEdit() {
  stopEditing()
  clearErrors()
}

// --- Session decisions ---------------------------------------------------------

const decisionsApi = useSessionDecisions({
  channelType: () => props.channelType,
  channelKey,
  ch,
  messageType,
  post: postTyped,
  setPending,
  submitting,
  composer: {
    editing: () => !!editing.value,
    discardEdit: onCancelEdit,
    endReply: () => (replyTo.value = null),
    reply: onReply,
    select: (type) => composer.value?.select(type) ?? false,
    showError: (m) => (formError.value = m),
  },
  ui: {
    confirm,
    toast: (m) => ui.error(m),
    handle,
    focusRow: (seq) => channelView.value?.focusRow(seq) ?? false,
    focusHeader: () => sessionHeader.value?.focus(),
  },
})
const { decisions, decisionSeq, deciding, onDecide, resetDecisions } = decisionsApi

// --- Message and channel operations -----------------------------------------

async function onDelete(seq: number) {
  const ok = await confirm({
    title: t('channels.deleteMessageTitle'),
    message: t('channels.deleteMessageBody'),
    confirmLabel: t('common.delete'),
    danger: true,
  })
  if (ok) await runPending(messagePendingKey(seq, 'delete'), () => ch.remove(seq))
}

const removing = ref<number | null>(null)
const removeBusy = ref(false)
const removeError = ref<string>()

function onRemove(seq: number) {
  removeError.value = undefined
  removing.value = seq
}

async function submitRemove(reason: string) {
  const seq = removing.value
  if (seq === null) return
  removeBusy.value = true
  setPending(messagePendingKey(seq, 'remove'), true)
  try {
    await ch.remove(seq, reason || undefined)
    removing.value = null
  } catch (err) {
    removeError.value = handle(err, true)
    if (removeError.value === undefined) removing.value = null
  } finally {
    removeBusy.value = false
    setPending(messagePendingKey(seq, 'remove'), false)
  }
}

async function onRelease(seq: number) {
  await runPending(messagePendingKey(seq, 'release'), () => ch.release(seq))
}

async function onAction(seq: number, name: string) {
  const res = await runPending(messagePendingKey(seq, name), () => ch.runAction(seq, name))
  const warnings = res?.warnings ?? []
  if (warnings.length) ui.warning(warnings.map((w) => w.detail).join(' '))
}

async function onLifecycle(op: string) {
  const def = props.channelType.lifecycle.find((l) => l.name === op)
  if (op === LIFECYCLE_DELETE) {
    const ok = await confirm({
      title: def?.label
        ? t('channels.lifecycleTitle', { label: def.label })
        : t('channels.deleteChannelTitle'),
      message: t('channels.deleteChannelBody'),
      confirmLabel: def?.label ?? t('common.delete'),
      danger: true,
    })
    if (!ok) return
  }
  const channel = await runPending(lifecyclePendingKey(op), () => ch.lifecycle(op))
  // undefined: refused or already running. `gone` reads the list again
  // too, so a delete emits only that.
  if (channel === null) emit('gone')
  else if (channel !== undefined) emit('changed')
}

const { loadingOlder, onLoadOlder, onLoadNewest, onJump } = useChannelPaging(ch, (err) =>
  handle(err, false)
)

// --- Links and the ref picker -----------------------------------------------

const { entityHref, onOpenRef, onLogClick } = useChannelLinks(() => props.world)
const picker = useRefPicker()
const pickerOpen = picker.open
const pickerTypes = picker.types
</script>

<template>
  <div class="open-channel" data-testid="open-channel" @click="onLogClick">
    <RlBanner
      v-if="ch.error.value && !ch.head.value"
      tone="danger"
      :title="t('channels.loadFailed')"
    >
      <RlButton size="sm" @click="ch.refetch()">{{ t('common.retry') }}</RlButton>
    </RlBanner>
    <RlChannel
      v-else-if="ch.head.value"
      ref="channelView"
      :title="title"
      :state="ch.head.value.channel.state"
      :messages="views"
      :message-types="channelType.message_types"
      :postable="ch.head.value._actions.post"
      :lifecycle="lifecycleDefs(channelType)"
      :allowed-lifecycle="ch.head.value._actions.lifecycle"
      :ephemeral="ephemeral"
      :typing="typingNow"
      :typing-offered="channelType.typing"
      :presentation="presentation"
      :enum-options="enumOptions"
      :entity-href="entityHref"
      :pending="shownPending"
      :decisions="decisionsApi.rowDecisions.value"
      :decision-seq="decisionSeq"
      :choices="decisionsApi.choices.value"
      :has-older="ch.hasOlder.value"
      :has-newer="ch.hasNewer.value"
      :loading-older="loadingOlder"
      @lifecycle="onLifecycle"
      @reply="onReply"
      @edit="onEdit"
      @delete="onDelete"
      @remove="onRemove"
      @release="onRelease"
      @action="onAction"
      @open-ref="onOpenRef"
      @load-older="onLoadOlder"
      @load-newest="onLoadNewest"
      @jump="onJump"
      @decide="(key) => onDecide(key, 'row')"
      @choose="decisionsApi.onChoose"
    >
      <template v-if="session.value" #session>
        <RlChannelSession ref="sessionHeader" :session="session.value">
          <template v-if="decisions.length" #decisions>
            <RlChannelDecisions
              :decisions="decisions"
              :pending="shownPending"
              @decide="(key) => onDecide(key, 'header')"
            />
          </template>
        </RlChannelSession>
      </template>
      <template #composer>
        <RlChannelComposer
          ref="composer"
          :message-types="channelType.message_types"
          :postable="ch.head.value._actions.post"
          :transitions="transitions.value"
          :presentation="presentation"
          :enum-options="enumOptions"
          :pick-ref="picker.pickRef"
          :reply-to="replyQuoteView"
          :editing="editing"
          :submitting="submitting"
          :blocked="deciding"
          :errors="fieldErrors"
          :error="formError"
          :time-zone-label="tz"
          @post="onPost"
          @save="onSave"
          @cancel-reply="replyTo = null"
          @cancel-edit="onCancelEdit"
          @typing="typing.onInput"
          @select="decisionsApi.onComposerSelect"
        />
      </template>
    </RlChannel>
    <p v-else class="open-channel__loading" role="status">{{ t('channels.loading') }}</p>

    <ChannelTextDialog
      v-if="removing !== null"
      :title="t('channels.removeTitle')"
      :description="t('channels.removeDescription')"
      :label="t('channels.reason')"
      :required="channelType.remove_reason_required"
      multiline
      :confirm-label="t('common.remove')"
      :pending-label="t('channels.removing')"
      danger
      :busy="removeBusy"
      :error="removeError"
      @close="removing = null"
      @submit="submitRemove"
    />
    <EntityPickerModal
      :open="pickerOpen"
      :types="pickerTypes"
      :label="t('channels.chooseEntity')"
      @select="picker.onPicked"
      @close="picker.onClose"
    />
  </div>
</template>
