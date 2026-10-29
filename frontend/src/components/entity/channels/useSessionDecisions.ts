import { computed, nextTick, ref, watch, type Ref } from 'vue'
import { t } from '@/i18n'
import type { useChannel } from '@/composables/useChannel'
import type {
  ChannelMessageTypeSchema,
  ChannelTypeSchema,
  MessagePropInput,
} from '@/types/channels'
import type {
  ChannelDecisionView,
  ChannelPropValue,
} from 'rela-components/components/channel/types'
import { decisionPendingKey } from 'rela-components/components/channel/channelFields'
import { problemType } from './channelErrors'
import { decisionCause, decisionReplyTo, decisionViews } from './channelView'

/** The open channel's state the decisions read. */
export type DecisionChannel = Pick<
  ReturnType<typeof useChannel>,
  'head' | 'messages' | 'actionsGeneration' | 'refreshHead' | 'refetch'
>

/** The composer, as a decision that opens the form drives it. */
export interface DecisionComposer {
  /** A message is being edited. */
  editing(): boolean
  /** Ends the edit, after the user agreed to discard it. */
  discardEdit(): void
  /** Ends the reply being written. */
  endReply(): void
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
 */
export function useSessionDecisions(d: DecisionDeps) {
  const deciding = ref(false)
  /** The click that holds the lock; a reset or a later click moves it on. */
  let click = 0
  let heldKey: string | undefined
  /** The form decision the composer's Send answers, if any. */
  const formDecision = ref<{ messageType: string; generation: number }>()

  // The head is replaced on every frame; the views are rebuilt only when
  // what they read changes, so the buttons keep their identity.
  const memo = computed<{ ct: ChannelTypeSchema; key: string; value: ChannelDecisionView[] }>(
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

  const find = (key: string) => decisions.value.find((x) => x.key === key)
  const stale = () => d.ui.toast(t('channels.decisionStale'))

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
   * when it is of that type. The error of a refused post is rethrown for
   * the composer to show.
   */
  async function sendComposer(
    type: string,
    values: Record<string, MessagePropInput>,
    replyTo: number | undefined
  ): Promise<void> {
    const f = formDecision.value
    const generation = f?.messageType === type ? f.generation : undefined
    if (generation === undefined) formDecision.value = undefined
    try {
      await d.post(type, values, replyTo, generation)
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
    const view0 = find(key)
    if (!view0) return stale()
    deciding.value = true
    const mine = ++click
    const channel = d.channelKey()
    const generation = d.ch.head.value?.channel.session?.generation ?? 0
    const cause = decisionSeq.value
    const live = () => mine === click && channel === d.channelKey()
    try {
      if (view0.opensForm) return await openForm(view0, generation, live)
      let view = view0
      if (view.confirm) {
        const ok = await d.ui.confirm({
          title: view.label,
          message: t('channels.decisionConfirmBody', { state: view.toLabel ?? view.label }),
          confirmLabel: view.label,
          danger: !!view.danger,
        })
        if (!ok || !live()) return
        // UX only: the server refuses a post at another generation anyway.
        const again = find(key)
        if (!again) return stale()
        view = again
      }
      const replyTo = decisionReplyTo(d.ch.messages.value, cause, d.messageType(view.messageType))
      hold(decisionPendingKey(key))
      try {
        await d.post(view.messageType, presetProps(view.props), replyTo, generation)
      } catch (err) {
        if (live()) await refused(err, view, live)
        return
      }
      if (!live()) return
      try {
        await d.ch.refreshHead()
      } catch (err) {
        if (live()) d.ui.handle(err, false)
        return
      }
      await nextTick()
      if (!live()) return
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
   * A decision that opens the form: asks before it discards an edit, and
   * records the generation read at the click for the form's Send.
   */
  async function openForm(view: ChannelDecisionView, generation: number, live: () => boolean) {
    if (d.composer.editing()) {
      const ok = await d.ui.confirm({
        title: t('channels.decisionDiscardEditTitle'),
        message: t('channels.decisionDiscardEditBody'),
        confirmLabel: t('channels.decisionDiscardEditConfirm'),
        danger: true,
      })
      if (!ok || !live()) return
      d.composer.discardEdit()
    }
    if (!(await select(view.messageType))) {
      if (live()) stale()
      return
    }
    if (!live()) return
    if (d.ch.head.value?.channel.session?.generation !== generation) {
      formDecision.value = undefined
      stale()
      return
    }
    formDecision.value = { messageType: view.messageType, generation }
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
    decisions,
    decisionSeq,
    deciding,
    formDecision,
    onComposerSelect,
    onDecide,
    resetDecisions,
    sendComposer,
  }
}

/** A preset prop value as a request sends it: a readable ref is its id. */
function presetProps(p: Record<string, ChannelPropValue> | undefined) {
  const out: Record<string, MessagePropInput> = {}
  for (const [name, v] of Object.entries(p ?? {})) {
    if (typeof v !== 'object') out[name] = v
    else if (!v.restricted) out[name] = v.id
  }
  return out
}
