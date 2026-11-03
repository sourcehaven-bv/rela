import { computed, nextTick, ref, watch, type Ref } from 'vue'
import { i18n, t } from '@/i18n'
import type { useChannel } from '@/composables/useChannel'
import type {
  ChannelMessageTypeSchema,
  ChannelTypeSchema,
  MessagePropInput,
} from '@/types/channels'
import type {
  ChannelComposeView,
  ChannelDecisionView,
  ChannelPropValue,
} from 'rela-components/components/channel/types'
import { fromPropValue } from '@/utils/channelValues'
import { decisionPendingKey } from 'rela-components/components/channel/channelFields'
import { problemType } from './channelErrors'
import type { AppDecisionView } from './presetViews'
import {
  choiceViews,
  decisionCause,
  decisionReplyTo,
  decisionViews,
  rowDecisions as rowDecisionsOf,
} from './channelView'

/** The open channel's state the decisions read. */
export type DecisionChannel = Pick<
  ReturnType<typeof useChannel>,
  'head' | 'messages' | 'hasNewer' | 'actionsGeneration' | 'refreshHead' | 'refetch'
>

/** The composer, as a decision that opens the form drives it. */
export interface DecisionComposer {
  /** A message is being edited. */
  editing(): boolean
  /** Ends the edit, after the user agreed to discard it. */
  discardEdit(): void
  /** Ends the reply being written. */
  endReply(): void
  /** Starts a reply to the message with seq. */
  reply(seq: number): void
  /** Selects the type; false when the composer does not offer it. */
  select(type: string): boolean
  /** Shows a refusal in the composer. */
  showError(message: string): void
}

/** How the panel talks to the user. */
export interface DecisionUI {
  confirm(o: {
    title: string
    message: string
    confirmLabel: string
    danger: boolean
  }): Promise<boolean>
  toast(message: string): void
  /** Answers a refusal the panel's way; returns the message when it belongs in a form. */
  handle(err: unknown, hasForm: boolean): string | undefined
  /** Focuses the row with seq; false when it is not shown. */
  focusRow(seq: number): boolean
  focusHeader(): void
}

/** What the decisions need from the open channel around them. */
export interface DecisionDeps {
  channelType: () => ChannelTypeSchema
  /** Names the open channel; a decision whose channel changed drops its after-steps. */
  channelKey: () => string
  ch: DecisionChannel
  messageType: (name: string) => ChannelMessageTypeSchema | undefined
  /** Posts through the composer's typing-aware path. */
  post: (
    type: string,
    values: Record<string, MessagePropInput>,
    replyTo: number | undefined,
    expectGeneration: number | undefined
  ) => Promise<unknown>
  setPending: (key: string, on: boolean) => void
  /** The composer is sending; a decision waits for it. */
  submitting: Ref<boolean>
  composer: DecisionComposer
  ui: DecisionUI
}

/**
 * Session decisions as buttons (TKT-T9G1AE), for OpenChannel.
 *
 * A click takes the `deciding` lock and reads the session generation
 * before anything is awaited, and posts the type with that generation
 * (`expect_generation`). A click on a state the session has left, even one
 * that offers the same type again, is refused under the store's lock.
 *
 * The lock lives for one click. `resetDecisions()` (a channel switch) releases it
 * at once; the click it belonged to then finishes without touching the new
 * channel: no head refresh, no focus, no form error. Whether decisions
 * show depends only on the held `_actions` matching the head's generation,
 * which every head fetch re-establishes, so the lock cannot stick.
 *
 * The clicked button shows its pending treatment only while the post is in
 * flight, not while a dialog is open.
 *
 * A decision that opens the form records its type and the generation read
 * at the click in `formDecision` (TKT-FPXTQJ). `sendComposer` posts the
 * composer's message of that type with the generation, so a form decision
 * is refused with session_moved like a one-click one. The record ends
 * when the composer switches to another type (`onComposerSelect`), after a
 * successful send, when a message of another type is sent, when a
 * session_moved refusal answers the send, on `resetDecisions()`, and when
 * a head frame brings another generation outside a send; only in that
 * last case is the user told the session moved on.
 *
 * Choices (TKT-03FQPD) are the reply buttons of a question's options. They
 * share the lock and the pending keys with the decisions, so one click
 * blocks both. A choice posts the answer type as a reply to the question,
 * with the head generation read at the click in a channel with a session
 * and none without. Other opens the composer with the answer type and the
 * reply set; in a channel with a session it records a form decision, as a
 * decision that opens the form does.
 *
 * A preset with `compose: true` (TKT-53FN0L) opens the form too. Its form
 * decision also holds `compose`: the preset, fixed for the Send, and the
 * view the composer shows (`composeView`). It is recorded before the
 * composer selects the type, so the composer renders compose mode before
 * it moves the focus. `sendComposer` adds the preset to the posted values
 * when the composer reports the same compose key, so a composer that
 * stopped applying it never has its fields overwritten.
 * `cancelCompose` ends it, besides the rules above. A second click gives a
 * new key, so the composer moves the focus again, and reads the generation
 * again.
 */
export function useSessionDecisions(d: DecisionDeps) {
  const deciding = ref(false)
  /** The click that holds the lock; a reset or a later click moves it on. */
  let click = 0
  let heldKey: string | undefined
  /** The form decision the composer's Send answers, if any. */
  const formDecision = ref<FormDecision>()
  /** Counts compose clicks, so each gives the composer a new key. */
  let composeClicks = 0
  /** The compose decision the composer shows, if any. */
  const composeView = computed(() => formDecision.value?.compose?.view)

  // The head is replaced on every frame; the views are rebuilt only when
  // what they read changes, so the buttons keep their identity.
  const memo = computed<{ ct: ChannelTypeSchema; key: string; value: AppDecisionView[] }>(
    (prev) => {
      const ct = d.channelType()
      const h = d.ch.head.value
      const cs = h?.channel.session
      const key = JSON.stringify([
        cs?.generation,
        cs?.state,
        cs?.stranded,
        cs?.final,
        cs?.transitions,
        h?.channel.state,
        h?._actions.post,
        h?._actions.transitions,
        h?._actions.entries,
        d.ch.actionsGeneration.value,
      ])
      if (prev && prev.ct === ct && prev.key === key) return prev
      const value = h ? decisionViews(ct, h, d.ch.actionsGeneration.value) : []
      return { ct, key, value }
    }
  )
  const decisions = computed(() => memo.value.value)

  /** The message that moved the session into its current state, when held. */
  const decisionSeq = computed(() =>
    decisions.value.length
      ? decisionCause(d.ch.messages.value, d.ch.head.value?.channel.session)
      : undefined
  )

  // The choices also read the held messages. That ref is replaced on every
  // merge, so its identity is part of the key: an answer arriving, an edit
  // or a removal rebuilds them. The Other label is text, so the language is
  // part of the key too.
  type ChoicesMemo = {
    ct: ChannelTypeSchema
    key: string
    messages: unknown
    value: Map<number, ChannelDecisionView[]>
  }
  const choicesMemo = computed<ChoicesMemo>((prev) => {
    const ct = d.channelType()
    const h = d.ch.head.value
    const messages = d.ch.messages.value
    if (!h || !ct.message_types.some((mt) => mt.choices)) {
      return prev && prev.ct === ct && prev.key === ''
        ? prev
        : { ct, key: '', messages, value: new Map() }
    }
    const key = JSON.stringify([
      h.channel.session?.generation,
      h.channel.state,
      h._actions.post,
      d.ch.actionsGeneration.value,
      d.ch.hasNewer.value,
      i18n.global.locale.value,
    ])
    if (prev && prev.ct === ct && prev.key === key && prev.messages === messages) return prev
    const value = choiceViews(ct, h, d.ch.actionsGeneration.value, d.ch.hasNewer.value, messages)
    return { ct, key, messages, value }
  })
  const choices = computed(() => choicesMemo.value.value)

  // The row of the question that moved the session shows its choices, not
  // the decision of the answer type as well; the stepper keeps that one.
  const rowDecisions = computed(() =>
    rowDecisionsOf(decisions.value, choices.value, decisionSeq.value)
  )

  const find = (key: string) => decisions.value.find((x) => x.key === key)

  /** The choice with key, and the seq of its question. */
  function findChoice(key: string): { seq: number; view: ChannelDecisionView } | undefined {
    for (const [seq, views] of choices.value) {
      const view = views.find((v) => v.key === key)
      if (view) return { seq, view }
    }
    return undefined
  }
  const stale = () => d.ui.toast(t('channels.decisionStale'))
  // Without a session, a choice goes stale because someone answered first;
  // there is no session to name.
  const staleChoice = () =>
    d.channelType().session ? stale() : d.ui.toast(t('channels.choiceStale'))

  // A move outside a send makes the open form decision stale. During a
  // send the move is usually the send's own; a move by someone else then
  // is refused by the server with session_moved.
  watch(
    () => d.ch.head.value?.channel.session?.generation,
    (g) => {
      const f = formDecision.value
      if (!f || g === undefined || g === f.generation || d.submitting.value) return
      formDecision.value = undefined
      stale()
    }
  )

  /**
   * Posts the composer's message, with the generation of the form decision
   * when it is of that type. composeKey is the compose key the composer
   * applied (`ChannelPostEvent.compose`); the preset is added only when it
   * matches the record's, so a composer that rendered the given properties
   * as fields never has the user's values overwritten. The error of a
   * refused post is rethrown for the composer to show.
   */
  async function sendComposer(
    type: string,
    values: Record<string, MessagePropInput>,
    replyTo: number | undefined,
    composeKey?: string
  ): Promise<void> {
    const f = formDecision.value
    const generation = f?.messageType === type ? f.generation : undefined
    if (generation === undefined) formDecision.value = undefined
    // With the keys matching, the values hold only the rendered fields, so
    // the preset overrides nothing the user typed.
    const preset =
      f?.messageType === type && composeKey !== undefined && f.compose?.view.key === composeKey
        ? f.compose.preset
        : undefined
    try {
      await d.post(type, preset ? { ...values, ...preset } : values, replyTo, generation)
    } catch (err) {
      if (generation !== undefined && problemType(err) === 'session_moved') {
        formDecision.value = undefined
      }
      throw err
    }
    formDecision.value = undefined
  }

  function hold(key: string | undefined) {
    if (heldKey) d.setPending(heldKey, false)
    heldKey = key
    if (key) d.setPending(key, true)
  }

  /**
   * Selects type in the composer, after ending a reply the type may not
   * carry. False when the composer does not offer it.
   */
  async function select(type: string): Promise<boolean> {
    if (!d.messageType(type)?.reply) d.composer.endReply()
    await nextTick()
    return d.composer.select(type)
  }

  /** The composer switched to type; a form decision of another type ends. */
  function onComposerSelect(type: string) {
    if (formDecision.value && formDecision.value.messageType !== type)
      formDecision.value = undefined
  }

  /** The user dropped the compose decision in the composer. */
  function cancelCompose() {
    formDecision.value = undefined
  }

  /** Releases the lock of a click in flight; for a channel switch. */
  function resetDecisions() {
    click++
    hold(undefined)
    deciding.value = false
    formDecision.value = undefined
  }

  /**
   * Makes a session decision. Everything the post depends on is read
   * before the first await; `live()` tells, after each await, whether this
   * click still owns the lock and its channel is still open.
   */
  async function onDecide(key: string, from: 'header' | 'row') {
    if (deciding.value || d.submitting.value) return
    const view = find(key)
    if (!view) return stale()
    deciding.value = true
    const mine = ++click
    const channel = d.channelKey()
    const generation = d.ch.head.value?.channel.session?.generation ?? 0
    const cause = decisionSeq.value
    const live = () => mine === click && channel === d.channelKey()
    try {
      if (view.opensForm) {
        await openForm(view, generation, live)
        return
      }
      if (view.confirm) {
        const ok = await d.ui.confirm({
          title: view.label,
          message: confirmMessage(view),
          confirmLabel: view.label,
          // The dialog's red follows the outcome, not the button's style.
          danger: !!(view.danger || view.outcomeDanger),
        })
        if (!ok || !live()) return
        // UX only: a state that no longer offers the key is stale; the
        // server refuses a post at another generation anyway. The view
        // posted is the one confirmed, not the one now under the key,
        // which a reload that reorders entries may have changed.
        if (!find(key)) return stale()
      }
      const replyTo = decisionReplyTo(d.ch.messages.value, cause, d.messageType(view.messageType))
      if (!(await postView(key, view, replyTo, generation, live))) return
      if (from === 'row' && cause !== undefined && d.ui.focusRow(cause)) return
      d.ui.focusHeader()
    } finally {
      if (mine === click) {
        hold(undefined)
        deciding.value = false
      }
    }
  }

  /**
   * Posts view and refreshes the head. True when the post landed and the
   * click still owns the lock, so the caller may move the focus.
   */
  async function postView(
    key: string,
    view: ChannelDecisionView,
    replyTo: number | undefined,
    generation: number | undefined,
    live: () => boolean
  ): Promise<boolean> {
    hold(decisionPendingKey(key))
    try {
      await d.post(view.messageType, presetProps(view.props), replyTo, generation)
    } catch (err) {
      if (live()) await refused(err, view, live)
      return false
    }
    if (!live()) return false
    try {
      await d.ch.refreshHead()
    } catch (err) {
      if (live()) d.ui.handle(err, false)
      return false
    }
    await nextTick()
    return live()
  }

  /**
   * Answers a question with one of its options, or opens the composer for
   * Other. Like `onDecide`, it reads the generation before the first await;
   * without a session there is none to send.
   */
  async function onChoose(key: string) {
    if (deciding.value || d.submitting.value) return
    const found = findChoice(key)
    if (!found) return staleChoice()
    const { seq, view } = found
    deciding.value = true
    const mine = ++click
    const channel = d.channelKey()
    const generation = d.ch.head.value?.channel.session?.generation
    const live = () => mine === click && channel === d.channelKey()
    try {
      if (view.opensForm) {
        if ((await openForm(view, generation, live, staleChoice)) && live()) d.composer.reply(seq)
        return
      }
      if (await postView(key, view, seq, generation, live)) d.ui.focusRow(seq)
    } finally {
      if (mine === click) {
        hold(undefined)
        deciding.value = false
      }
    }
  }

  /**
   * A decision that opens the form: asks before it discards an edit, and
   * records the generation read at the click for the form's Send. Without
   * a session (`generation` undefined) there is nothing to record, and a
   * compose view is stale. True
   * when the composer shows the type. onStale tells the user when the
   * composer no longer offers the type.
   */
  async function openForm(
    view: ChannelDecisionView,
    generation: number | undefined,
    live: () => boolean,
    onStale: () => void = stale
  ): Promise<boolean> {
    if (d.composer.editing()) {
      const ok = await d.ui.confirm({
        title: t('channels.decisionDiscardEditTitle'),
        message: t('channels.decisionDiscardEditBody'),
        confirmLabel: t('channels.decisionDiscardEditConfirm'),
        danger: true,
      })
      if (!ok || !live()) return false
      d.composer.discardEdit()
      // Let the composer restore what the edit put aside before compose
      // mode arrives, so the restore cannot take its type away.
      await nextTick()
      if (!live()) return false
    }
    const compose = (view as AppDecisionView).compose ? composeOf(view) : undefined
    if (compose) {
      if (generation === undefined) {
        // A compose Send needs a generation; without one it would post a
        // plain form with no preset.
        onStale()
        return false
      }
      // Recorded before the composer selects the type, so it renders
      // compose mode before it moves the focus.
      if (d.ch.head.value?.channel.session?.generation !== generation) {
        formDecision.value = undefined
        stale()
        return false
      }
      formDecision.value = { messageType: view.messageType, generation, compose }
    }
    if (!(await select(view.messageType))) {
      if (compose && live()) formDecision.value = undefined
      if (live()) onStale()
      return false
    }
    if (!live()) return false
    if (generation === undefined || compose) return true
    if (d.ch.head.value?.channel.session?.generation !== generation) {
      formDecision.value = undefined
      stale()
      return false
    }
    formDecision.value = { messageType: view.messageType, generation }
    return true
  }

  /**
   * The compose record of a preset view: its props in stored form for the
   * Send, and the view the composer shows, with a new key per click.
   */
  function composeOf(view: ChannelDecisionView): ComposeRecord {
    const preset = presetProps(view.props)
    const props = d.messageType(view.messageType)?.properties ?? []
    const given: ChannelComposeView['given'] = {}
    for (const [name, v] of Object.entries(preset)) {
      const p = props.find((x) => x.name === name)
      given[name] = Array.isArray(v) ? [...v] : p ? fromPropValue(p, v) : String(v)
    }
    const key = `${view.key}#${++composeClicks}`
    return {
      preset,
      view: { key, type: view.messageType, label: view.label, toLabel: view.toLabel, given },
    }
  }

  /**
   * Answers a refused decision. session_moved goes through the panel's
   * mapping like any refusal. session_wrong_state means the same to a
   * decision, so it gets the same message. A form refusal (invalid_message)
   * opens the composer with the type and shows the server's message there;
   * during an edit it is a toast.
   */
  async function refused(err: unknown, view: ChannelDecisionView, live: () => boolean) {
    if (problemType(err) === 'session_wrong_state') {
      stale()
      void d.ch.refetch()
      return
    }
    const message = d.ui.handle(err, !d.composer.editing())
    if (message === undefined) return
    if ((await select(view.messageType)) && live()) d.composer.showError(message)
    else if (live()) d.ui.toast(message)
  }

  return {
    choices,
    decisions,
    decisionSeq,
    rowDecisions,
    cancelCompose,
    composeView,
    deciding,
    formDecision,
    onChoose,
    onComposerSelect,
    onDecide,
    resetDecisions,
    sendComposer,
  }
}

/** A compose preset's part of a form decision (TKT-53FN0L). */
interface ComposeRecord {
  /** The preset in stored form, added to the Send. */
  preset: Record<string, MessagePropInput>
  /** What the composer shows. */
  view: ChannelComposeView
}

/** A decision that opened the form: the type and generation its Send carries. */
interface FormDecision {
  messageType: string
  generation: number
  compose?: ComposeRecord
}

/**
 * The body of a decision's confirm dialog: the authored text as written,
 * else a translated text that says the session moves to a state that is
 * not final, or ends in a final one.
 */
function confirmMessage(view: AppDecisionView): string {
  if (view.confirmText) return view.confirmText
  const state = view.toLabel ?? view.label
  return view.movesOnly
    ? t('channels.decisionConfirmMoveBody', { state })
    : t('channels.decisionConfirmBody', { state })
}

/** A preset prop value as a request sends it: a readable ref is its id. */
function presetProps(p: Record<string, ChannelPropValue> | undefined) {
  const out: Record<string, MessagePropInput> = {}
  for (const [name, v] of Object.entries(p ?? {})) {
    if (Array.isArray(v) || typeof v !== 'object') out[name] = v
    else if (!v.restricted) out[name] = v.id
  }
  return out
}
