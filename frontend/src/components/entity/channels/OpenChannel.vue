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
 * Session decisions (TKT-T9G1AE) are buttons, and choices (TKT-03FQPD) sit
 * on a question's row; their logic is useSessionDecisions. Each decision
 * shows once (TKT-A0RTC52): on the message that moved the session into its
 * current state when it is held, with a "Go to decision" button in the
 * stepper, else in the stepper. The composer's type picker leaves out the
 * types those buttons stand for. When the decisions change place, a
 * focused decision button hands the focus to the same decision in the new
 * place, else to "Go to decision", else to the stepper.
 *
 * It emits `gone` when the channel can no longer be shown: a refused read
 * (the reader lost access, or the channel was deleted) or a refusal that
 * names the channel as missing. useChannel has already dropped what it held
 * by then. `changed` asks the host to read the channel list again after a
 * lifecycle operation, since the list is not live while a channel is open.
 *
 * `revealSeq` comes from a message link (TKT-O6RC8Z): useMessageLink jumps
 * to that message once, as a reply quote does, and `revealed` reports it.
 *
 * `start` comes from an inbox row (TKT-V05706X): a decision or choice to
 * start once the channel is shown, such as a form decision the row cannot
 * make. It runs once per `start` object, when the head and its first
 * page are held, the `revealSeq` jump has run, and the held `_actions`
 * match the head's generation. A generation other than the row's means
 * the session moved on: it says so and starts nothing. A choice the held
 * page cannot build (the question is not held) opens its answer type as
 * a reply to `start.cause`. `started` reports the start, and `startSent`
 * the successful send of the form it opened.
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
import { checkFiles, toEditProps, toProps } from '@/utils/channelValues'
import type { MessageFileUpload } from '@/api/channels'
import {
  LIFECYCLE_DELETE,
  type ChannelStart,
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
  lifecyclePendingKey,
  messagePendingKey,
} from 'rela-components/components/channel/channelFields'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlBanner from 'rela-components/components/feedback/RlBanner.vue'
import RlChannel from 'rela-components/components/channel/RlChannel.vue'
import RlChannelComposer from 'rela-components/components/channel/RlChannelComposer.vue'
import RlChannelDecisions from 'rela-components/components/channel/RlChannelDecisions.vue'
import RlChannelSession from 'rela-components/components/channel/RlChannelSession.vue'
import { useMessages } from 'rela-components/composables/useMessages'
import EntityPickerModal from '@/components/forms/EntityPickerModal.vue'
import ChannelTextDialog from './ChannelTextDialog.vue'
import { blockedPending, handleChannelRefusal, type RefusalTarget } from './decisionGlue'
import { useChannelFiles } from './useChannelFiles'
import { useChannelLinks } from './useChannelLinks'
import { useChannelPaging } from './useChannelPaging'
import { useExpiryClock } from './useExpiryClock'
import { useMessageLink } from './useMessageLink'
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
  /** The message a link asks to show; see the component doc. */
  revealSeq?: number
  /** A decision to start once shown; see the component doc. */
  start?: ChannelStart
}>()

const emit = defineEmits<{
  gone: []
  changed: []
  revealed: []
  started: []
  startSent: []
}>()

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
// Previews of the held messages' images; dropped with the channel.
const files = useChannelFiles(target)
watch(ch.messages, (list) => files.sync(list), { immediate: true })
const ctx = computed(() => ({
  tz,
  world: props.world,
  session: props.channelType.session,
  filePreview: files.previewUrl,
}))
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
function handle(err: unknown, target: RefusalTarget): string | undefined {
  return handleChannelRefusal(err, target, {
    refetch: () => void ch.refetch(),
    gone: () => emit('gone'),
    reloadSchema: () => {
      void channelSchema.reload()
      void schema.reloadChannelPresentations()
    },
    toast: (m) => ui.error(m),
  })
}

const { pending, set: setPending, run: runPending } = usePendingKeys((err) => handle(err, 'toast'))

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
const rlMessages = useMessages()
const sessionHeader = useTemplateRef<InstanceType<typeof RlChannelSession>>('sessionHeader')
const channelView = useTemplateRef<InstanceType<typeof RlChannel>>('channelView')

/**
 * The pending keys the library sees: a composer send, or a decision in
 * progress (a dialog open, a post in flight), blocks every decision. Only
 * the posting decision's own key shows it as sending.
 */
const shownPending = computed<ReadonlySet<string>>(() =>
  submitting.value || deciding.value ? blockedPending(pending.value) : pending.value
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
  files.clear()
  replyTo.value = null
  stopEditing()
  clearErrors()
  resetDecisions()
  startedType = undefined
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
  expectGeneration?: number,
  uploads?: readonly MessageFileUpload[]
) {
  try {
    await typing.settle()
    return await ch.post(type, values, replyTo, expectGeneration, uploads)
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
  const picked = checkFiles(mt.properties, event.files)
  if (!converted.ok || !picked.ok) {
    fieldErrors.value = {
      ...(converted.ok ? {} : converted.errors),
      ...(picked.ok ? {} : picked.errors),
    }
    formError.value = picked.ok ? undefined : picked.error
    return
  }
  clearErrors()
  submitting.value = true
  // Read before the send, which ends the form decision.
  const fromStart =
    startedType !== undefined &&
    startedType === event.type &&
    decisionsApi.formDecision.value?.messageType === startedType
  try {
    await decisionsApi.sendComposer(
      event.type,
      converted.props,
      event.replyTo,
      event.compose,
      picked.uploads
    )
    composer.value?.reset(event)
    replyTo.value = null
    if (fromStart) {
      startedType = undefined
      emit('startSent')
    }
  } catch (err) {
    formError.value = handle(err, 'form')
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
    formError.value = handle(err, 'form')
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
    replying: () => replyTo.value ?? undefined,
    select: (type) => composer.value?.select(type) ?? false,
    showError: (m) => (formError.value = m),
  },
  ui: {
    confirm,
    toast: (m) => ui.error(m),
    handle: (err, hasForm) => handle(err, hasForm ? 'form' : 'toast'),
    focusRow: (seq) => channelView.value?.focusRow(seq) ?? false,
    focusHeader: () => sessionHeader.value?.focus(),
  },
  quiet: (type) => !!presentation.value?.messages?.[type]?.display,
  unoffered: (type) => presentation.value?.messages?.[type]?.offer === false,
})
const { decisions, decisionSeq, deciding, onDecide, resetDecisions } = decisionsApi
const placed = decisionsApi.placed
const buttonTypes = decisionsApi.buttonTypes

/** The header's "Go to decision": shown when the decisions sit on the waiting message. */
const showJump = computed(() => decisions.value.length > 0 && decisionSeq.value !== undefined)

/**
 * Focuses the first usable button on the waiting message. The button of a
 * decision in flight stays usable for focus. With none, it shows the
 * message itself.
 */
function goToDecision() {
  const seq = decisionSeq.value
  const view = channelView.value
  if (seq === undefined || !view) return
  if (!view.focusDecisions(seq)) view.reveal(seq)
}

// --- Focus when the decisions change place -------------------------------------
//
// The decisions move between the header and the waiting message when that
// message is loaded or leaves the log. A focused decision button, or "Go
// to decision", hands the focus on. Moved to the row: to "Go to decision",
// so the page does not jump to the row. Moved to the header: to the same
// decision there, else (from "Go to decision") to the first one. Else to
// the header. Every lookup stays inside this channel's root.

const root = useTemplateRef<HTMLElement>('root')
/**
 * What had the focus before the move: a decision button's `data-decision`,
 * `jump` for "Go to decision", undefined for anything else.
 */
let moving: { key: string } | 'jump' | undefined

const PLACES = '.rl-channel-session [data-decision], .rl-channel-message .rl-channel-decisions [data-decision]'
const JUMP = '[data-testid="session-go-to-decision"]'

watch(
  () => placed.value.fallback.length > 0,
  () => {
    const active = document.activeElement
    const own = active instanceof HTMLElement && root.value?.contains(active)
    if (!own) moving = undefined
    else if (active.matches(JUMP)) moving = 'jump'
    else if (active.matches(PLACES) && active.dataset.decision) moving = { key: active.dataset.decision }
    else moving = undefined
  },
  { flush: 'pre' }
)
watch(
  () => placed.value.fallback.length > 0,
  (inHeader) => {
    const from = moving
    moving = undefined
    const el = root.value
    // Nothing to hand on, or the focused button is still there.
    if (from === undefined || !el || el.contains(document.activeElement)) return
    const header = '.rl-channel-session'
    const target = !inHeader
      ? el.querySelector<HTMLElement>(`${header} ${JUMP}`)
      : from === 'jump'
        ? el.querySelector<HTMLElement>(`${header} [data-decision]`)
        : el.querySelector<HTMLElement>(`${header} [data-decision="${CSS.escape(from.key)}"]`)
    if (target) {
      target.focus({ preventScroll: true })
      return
    }
    el.querySelector(header)?.scrollIntoView?.({ block: 'nearest' })
    sessionHeader.value?.focus()
  },
  { flush: 'post' }
)

// --- A decision started from an inbox row ------------------------------------

/** The `start` that ran; another object runs again. */
let ranStart: ChannelStart | undefined
/** The type of the form the start opened; its send emits startSent. */
let startedType: string | undefined
/** Whether the `revealSeq` jump has run. */
const revealDone = ref(false)
watch(
  () => props.revealSeq,
  () => (revealDone.value = false)
)
/**
 * The older pages a jump is reading. The link's jumpTo emits `jump`
 * synchronously when the message is not held, so the count is up before
 * revealDone is set, and the start waits for those pages.
 */
const jumps = ref(0)
async function onJumpTracked(seq: number) {
  jumps.value++
  try {
    await onJump(seq)
  } finally {
    jumps.value--
  }
}

// The started form ends without a send (another type, a stale move, a
// cancel): a later send of the same type is not the start's.
watch(
  () => decisionsApi.formDecision.value,
  (f) => {
    if (!f) startedType = undefined
  }
)

watch(
  [() => props.start, ch.head, ch.messages, ch.actionsGeneration, revealDone, jumps],
  () => void runStart(),
  { immediate: true }
)

async function runStart() {
  const s = props.start
  const h = ch.head.value
  if (!s || s === ranStart || !h) return
  // The first page: a channel with messages holds at least one.
  if (h.channel.last_seq > 0 && ch.messages.value.length === 0) return
  if (props.revealSeq !== undefined && (!revealDone.value || jumps.value > 0)) return
  const g = h.channel.session?.generation
  if (ch.actionsGeneration.value !== g) return
  ranStart = s
  startedType = undefined
  if (g !== s.generation) {
    ui.error(t('channels.decisionStale'))
    return
  }
  emit('started')
  if (s.kind === 'decide') {
    const view = decisions.value.find((v) => v.key === s.key)
    await onDecide(s.key, 'row')
    if (view?.opensForm && decisionsApi.formDecision.value) startedType = view.messageType
    return
  }
  const built = [...decisionsApi.choices.value.values()].flat().find((v) => v.key === s.key)
  if (built) {
    await decisionsApi.onChoose(s.key)
    if (built.opensForm && decisionsApi.formDecision.value) startedType = built.messageType
  } else if (s.answerType && s.cause !== undefined) {
    if (await decisionsApi.onAnswer(s.answerType, s.cause)) startedType = s.answerType
  }
}

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
    removeError.value = handle(err, 'form')
    if (removeError.value === undefined) removing.value = null
  } finally {
    removeBusy.value = false
    setPending(messagePendingKey(seq, 'remove'), false)
  }
}

async function onDownload(seq: number, fileId: string) {
  const file = ch.messages.value.find((m) => m.seq === seq)?.files?.find((f) => f.id === fileId)
  if (!file) return
  await runPending(messagePendingKey(seq, `file:${fileId}`), () => files.download(seq, file))
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

const { loadingOlder, onLoadOlder, onLoadNewest, onJump } = useChannelPaging(
  ch,
  (err) => handle(err, 'toast'),
  (seq) => channelView.value?.reveal(seq)
)

useMessageLink(
  () => props.revealSeq,
  channelView,
  () => {
    revealDone.value = true
    emit('revealed')
  }
)

// --- Links and the ref picker -----------------------------------------------

const { entityHref, onOpenRef, onLogClick } = useChannelLinks(() => props.world)
const picker = useRefPicker()
const pickerOpen = picker.open
const pickerTypes = picker.types
</script>

<template>
  <div ref="root" class="open-channel" data-testid="open-channel" @click="onLogClick">
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
      :decisions="placed.row"
      :decision-seq="decisionSeq"
      :choices="decisionsApi.choices.value"
      :button-types="buttonTypes"
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
      @jump="onJumpTracked"
      @decide="(key) => onDecide(key, 'row')"
      @choose="decisionsApi.onChoose"
      @download="onDownload"
    >
      <template v-if="session.value" #session>
        <RlChannelSession
          ref="sessionHeader"
          :session="session.value"
          :compact="presentation?.session?.display === 'compact'"
        >
          <template v-if="placed.fallback.length" #decisions>
            <RlChannelDecisions
              :decisions="placed.fallback"
              :pending="shownPending"
              @decide="(key) => onDecide(key, 'header')"
            />
          </template>
          <template v-else-if="showJump" #decisions>
            <RlButton
              variant="secondary"
              size="sm"
              data-testid="session-go-to-decision"
              @click="goToDecision"
            >
              {{ rlMessages.channelText({ key: 'sessionGoToDecision' }) }}
            </RlButton>
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
          :compose="decisionsApi.composeView.value"
          :button-types="buttonTypes"
          :errors="fieldErrors"
          :error="formError"
          :time-zone-label="tz"
          @post="onPost"
          @save="onSave"
          @cancel-reply="replyTo = null"
          @cancel-edit="onCancelEdit"
          @typing="typing.onInput"
          @select="decisionsApi.onComposerSelect"
          @cancel-compose="decisionsApi.cancelCompose"
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
