/**
 * Maps the channel wire types (types/channels.ts) onto the presentational
 * shapes the rela-components channel components take (TKT-91BK6F R10).
 *
 * Everything the library leaves to the app happens here: times are
 * formatted in the browser's zone (the zone the composer converts in, see
 * utils/channelValues.ts), markdown is rendered by the strict
 * renderMessageMarkdown, enum labels and colours come from the schema, and
 * the ephemeral line is built as text. The functions are pure, so the
 * panel's tests and this module's tests pin the same mapping.
 */
import { t } from '@/i18n'
import { formatDate, formatDatetime } from '@/utils/format'
import {
  renderMessageMarkdown,
  type MessageMentionLink,
  type MessageRefLink,
} from '@/utils/markdown'
import { fromPropValue } from '@/utils/channelValues'
import { tagColor } from '@/configure/names'
import type { ChannelPresentation as AppChannelPresentation, CustomType } from '@/types'
import type {
  Channel,
  ChannelActions,
  ChannelAuthor,
  ChannelHead,
  ChannelMessageTypeSchema,
  ChannelPropertySchema,
  ChannelSession,
  ChannelSessionSchema,
  ChannelSessionTransition,
  ChannelTypeList,
  ChannelTypeSchema,
  EphemeralMessage,
  Message,
  MessagePropValue,
  ReadableRef,
  TypingEntry,
} from '@/types/channels'
import { LIFECYCLE_DELETE, isSessionNotice } from '@/types/channels'
import type {
  ChannelAuthorView,
  ChannelComposerTransition,
  ChannelDecisionView,
  ChannelEditing,
  ChannelEnumOption,
  ChannelEphemeralView,
  ChannelGroupView,
  ChannelLifecycleDef,
  ChannelMessageDisplay,
  ChannelMessageView,
  ChannelNoticeView,
  ChannelPresentation,
  ChannelReadableRef,
  ChannelReplyQuote,
  ChannelRowView,
  ChannelSessionView,
  ChannelTypingView,
} from 'rela-components/components/channel/types'
import { resolveChannelFields } from 'rela-components/components/channel/channelFields'

/** The longest reply excerpt, in code points. */
export const EXCERPT_MAX = 80

/**
 * Whether d is a display the library knows. A value from a newer server is
 * dropped, so its messages show in full rather than silently hidden.
 */
function isDisplay(d: unknown): d is ChannelMessageDisplay {
  return d === 'collapsed' || d === 'hidden'
}

/**
 * The data-entry.yaml presentation as the library takes it. A form field
 * may omit `property`; a channel field names one (the server refuses any
 * other at load), so a field without one is left out rather than trusted.
 */
export function presentationView(
  p: AppChannelPresentation | undefined
): ChannelPresentation | undefined {
  if (!p) return undefined
  const messages: NonNullable<ChannelPresentation['messages']> = {}
  for (const [type, mp] of Object.entries(p.messages ?? {})) {
    messages[type] = {
      fields: mp.fields?.flatMap((f) => (f.property ? [{ ...f, property: f.property }] : [])),
      ...(isDisplay(mp.display) ? { display: mp.display } : {}),
    }
  }
  return { messages }
}

/** What every mapping needs from the page. */
export interface ChannelViewContext {
  /** The IANA zone times are shown in. */
  tz: string
  /** The page world, carried on in-app entity links. */
  world?: string
  /** The channel type's session, for the state labels in session notices. */
  session?: ChannelSessionSchema
}

/**
 * A person, an agent or the server. An agent is a principal whose type the
 * server verified as `agent`, or a post made through the MCP tools.
 */
export function authorView(a: ChannelAuthor): ChannelAuthorView {
  if (a.system) return { name: a.user || t('channels.system'), kind: 'system' }
  if (a.principal_type === 'agent' || a.tool === 'mcp') return { name: a.user, kind: 'agent' }
  return { name: a.user, kind: 'person' }
}

/** The lifecycle labels; delete draws in the danger tone. */
export function lifecycleDefs(ct: ChannelTypeSchema): ChannelLifecycleDef[] {
  return ct.lifecycle.map((op) => ({ ...op, danger: op.name === LIFECYCLE_DELETE }))
}

function isReadableRef(v: MessagePropValue | undefined): v is ReadableRef {
  return !Array.isArray(v) && typeof v === 'object' && v !== null && v.restricted !== true
}

/** The readable refs among a message's props, keyed by property name. */
function readableRefs(props: Message['props']): Record<string, ReadableRef> {
  const out: Record<string, ReadableRef> = {}
  for (const [name, v] of Object.entries(props ?? {})) if (isReadableRef(v)) out[name] = v
  return out
}

function when(value: string | undefined, tz: string): string | undefined {
  if (!value) return undefined
  return formatDatetime(value, tz) ?? value
}

/**
 * A short plain-text excerpt of a message: its first string or markdown
 * prop, collapsed to one line, where a list reads as its items joined by
 * commas. It is rendered as text, so markdown syntax shows as written.
 */
export function excerptOf(
  m: Message,
  mt: ChannelMessageTypeSchema | undefined
): string | undefined {
  if (m.removed) return undefined
  const names = mt
    ? mt.properties.filter((p) => p.type === 'string' || p.type === 'markdown').map((p) => p.name)
    : Object.keys(m.props ?? {})
  for (const name of names) {
    const raw = m.props?.[name]
    const v = Array.isArray(raw) ? raw.join(', ') : raw
    if (typeof v !== 'string' || !v.trim()) continue
    const line = v.replace(/\s+/g, ' ').trim()
    const chars = [...line]
    return chars.length > EXCERPT_MAX ? `${chars.slice(0, EXCERPT_MAX - 1).join('')}…` : line
  }
  return undefined
}

/** The quote a reply shows: the original's author and excerpt when it is held. */
export function replyQuote(
  seq: number,
  original: Message | undefined,
  mt: ChannelMessageTypeSchema | undefined
): ChannelReplyQuote {
  return {
    seq,
    author: original && !original.removed ? original.author?.user : undefined,
    excerpt: original ? excerptOf(original, mt) : undefined,
  }
}

/** Renders a markdown prop. `memo` holds the HTML by `seq:rev:prop` (R19). */
export type MarkdownMemo = Map<string, string>

function renderProp(
  m: Message,
  name: string,
  src: string,
  ctx: ChannelViewContext,
  memo: MarkdownMemo | undefined
): string {
  const key = `${m.seq}:${m.rev}:${name}:${ctx.world ?? ''}`
  const held = memo?.get(key)
  if (held !== undefined) return held
  const refs: MessageRefLink[] = Object.values(readableRefs(m.props)).map((r) => ({
    id: r.id,
    type: r.type,
    title: r.title,
  }))
  const mentions: MessageMentionLink[] = (m.mentions ?? []).map((x) => ({
    text: x.text,
    kind: x.kind,
    ref: x.ref ? { id: x.ref.id, type: x.ref.type, title: x.ref.title } : undefined,
  }))
  const html = renderMessageMarkdown(src, { refs, mentions, world: ctx.world })
  memo?.set(key, html)
  return html
}

/** A prop formatted by the app, or undefined to let the library show it. */
function formatted(p: ChannelPropertySchema, v: MessagePropValue, tz: string): string | undefined {
  if (typeof v !== 'string') return undefined
  if (p.type === 'datetime') return formatDatetime(v, tz) ?? v
  if (p.type === 'date') return formatDate(v) ?? v
  return undefined
}

/** One wire message as the library draws it. */
export function messageView(
  m: Message,
  mt: ChannelMessageTypeSchema | undefined,
  ctx: ChannelViewContext,
  memo?: MarkdownMemo
): ChannelMessageView {
  const view: ChannelMessageView = {
    seq: m.seq,
    rev: m.rev,
    type: m.type,
    author: m.author ? authorView(m.author) : undefined,
    at: when(m.at, ctx.tz),
    datetime: m.at,
    editedAt: when(m.edited_at, ctx.tz),
    actions: m._actions,
  }
  if (m.removed) {
    view.removed = {
      kind: m.removed.kind,
      by: m.removed.by?.user,
      reason: m.removed.reason,
      at: when(m.removed.at, ctx.tz),
    }
    return view
  }
  view.props = m.props
  const notice = noticeView(m, ctx.session)
  if (notice) {
    view.notice = notice
    return view
  }
  if (m.action) {
    view.action = { name: m.action.name, status: m.action.status, promoted: m.action.promoted }
  }
  if (!mt || !m.props) return view
  const html: Record<string, string> = {}
  const text: Record<string, string> = {}
  for (const p of mt.properties) {
    const v = m.props[p.name]
    if (v === undefined) continue
    if (p.type === 'markdown' && typeof v === 'string')
      html[p.name] = renderProp(m, p.name, v, ctx, memo)
    const t = formatted(p, v, ctx.tz)
    if (t !== undefined) text[p.name] = t
  }
  if (Object.keys(html).length) view.html = html
  if (Object.keys(text).length) view.text = text
  return view
}

/** The label of a session state, or its name when the descriptor has no such state. */
function stateLabel(session: ChannelSessionSchema | undefined, name: string): string {
  return session?.states.find((st) => st.name === name)?.label ?? name
}

/**
 * The notice view of a session notice, or undefined for any other message.
 * A state the descriptor no longer has shows by its name. A notice names
 * no waiting roles: it is history, and the current configuration cannot
 * say whom the session waited for then. The stepper shows the current wait.
 */
export function noticeView(
  m: Message,
  session: ChannelSessionSchema | undefined
): ChannelNoticeView | undefined {
  if (!isSessionNotice(m)) return undefined
  const from = typeof m.props?.from === 'string' ? m.props.from : ''
  const to = typeof m.props?.to === 'string' ? m.props.to : ''
  return {
    fromLabel: stateLabel(session, from),
    toLabel: stateLabel(session, to),
    reentry: from === to,
  }
}

/**
 * The stepper view of a channel's session, or undefined when the type or
 * the channel has none. The next states come from the head's transitions.
 *
 * A head state the held descriptor does not know is usually one
 * realtime.yaml has just added, and the reload that brings it is on its
 * way: the view is loading, with no current state and no next states. It
 * is stranded only when the server says so, or when `answered` says a
 * fresh descriptor still lacks the state. Status, waiting roles and
 * waiting_on_me come from the head as they are.
 */
export function sessionView(
  ct: ChannelTypeSchema,
  channel: Channel,
  answered: (state: string) => boolean = () => false
): ChannelSessionView | undefined {
  const s = ct.session
  const cs = channel.session
  if (!s || !cs) return undefined
  const known = s.states.some((st) => st.name === cs.state)
  const stranded = !!cs.stranded || (!known && answered(cs.state))
  const loading = !stranded && !known
  const view: ChannelSessionView = {
    label: s.label,
    states: s.states.map((st) => ({ name: st.name, label: st.label, final: st.final })),
    currentLabel: known && !stranded ? stateLabel(s, cs.state) : cs.state,
    next: known && !stranded ? [...new Set((cs.transitions ?? []).map((tr) => tr.to))] : [],
  }
  if (stranded) view.stranded = true
  else if (loading) view.loading = true
  else view.current = cs.state
  if (known && !stranded && cs.final) view.final = true
  if (channel.state !== 'open') view.paused = channel.state
  if (cs.status) view.status = cs.status
  if (cs.waiting_for?.length)
    view.waitingFor = cs.waiting_for.map((w) => ({ label: w.label || w.role }))
  if (cs.waiting_on_me) view.waitingOnMe = true
  return view
}

/**
 * The message types whose post would move the session, for the composer:
 * those in the caller's `_actions.transitions` that are also transitions
 * out of the head's current state and postable now. The head session
 * follows every frame while the actions follow a head refetch, so a type
 * the session has already left is dropped here. A head state the held
 * descriptor does not know marks nothing: the stepper is loading then.
 *
 * A type may have several transitions (one per entry of its list). They
 * are grouped into one view: `toLabels` holds each target once, in the
 * server's order. `guarded` is set when every transition is guarded, and
 * `mayMove` when only some are: the move then depends on the poster or
 * the message, which the view cannot tell apart.
 */
export function composerTransitions(
  ct: ChannelTypeSchema,
  head: Pick<ChannelHead, 'channel'> & { _actions: Pick<ChannelActions, 'post' | 'transitions'> }
): ChannelComposerTransition[] {
  const cs = head.channel.session
  if (!ct.session || !cs || cs.stranded) return []
  if (!ct.session.states.some((st) => st.name === cs.state)) return []
  const post = new Set(head._actions.post)
  const out = new Map<string, ChannelSessionTransition[]>()
  for (const tr of cs.transitions ?? []) {
    const list = out.get(tr.message_type)
    if (list) list.push(tr)
    else out.set(tr.message_type, [tr])
  }
  return [...new Set(head._actions.transitions ?? [])].flatMap((name) => {
    const list = out.get(name)
    if (!list || !post.has(name)) return []
    const view: ChannelComposerTransition = {
      messageType: name,
      toLabels: [...new Set(list.map((tr) => tr.to))].map((to) => stateLabel(ct.session, to)),
    }
    const guarded = list.filter((tr) => tr.guarded).length
    if (guarded === list.length) view.guarded = true
    else if (guarded > 0) view.mayMove = true
    return [view]
  })
}

/** The head fields the decisions read. */
export type DecisionHead = Pick<ChannelHead, 'channel'> & {
  _actions: Pick<ChannelActions, 'post' | 'transitions'>
}

/**
 * The session decisions the caller may make now, as buttons: one per
 * message type in the caller's `_actions.transitions`, in that order (the
 * server's, by name).
 *
 * There are none unless the channel is open, the session is neither final
 * nor stranded, its state is in the descriptor, and the held `_actions`
 * were fetched at the head's current generation (`actionsGeneration`): a
 * message page moves the head's session ahead of the actions, and stale
 * actions would offer a decision of the state the session has left. A type
 * is kept when it is also in `_actions.post`, is a `log` type of the
 * descriptor, and has an edge out of the current state that a person's
 * post fires.
 *
 * A decision opens the form when an edge is guarded, its edges lead to
 * several states, or the type has a required property. `toLabel` is set
 * when every edge leads to one state. A one-click decision asks first
 * (`confirm`) when its state is final, and is `danger` when that state
 * ends as cancelled or failed. A later offer, choice or authored style
 * plugs in here.
 */
export function decisionViews(
  ct: ChannelTypeSchema,
  head: DecisionHead,
  actionsGeneration: number | undefined
): ChannelDecisionView[] {
  const s = ct.session
  const cs = head.channel.session
  if (head.channel.state !== 'open' || !s || !cs || cs.final || cs.stranded) return []
  if (!s.states.some((st) => st.name === cs.state)) return []
  if (actionsGeneration !== cs.generation) return []
  const post = new Set(head._actions.post)
  const edges = new Map<string, ChannelSessionTransition[]>()
  for (const tr of cs.transitions ?? []) {
    if (tr.by === 'trigger') continue
    const list = edges.get(tr.message_type)
    if (list) list.push(tr)
    else edges.set(tr.message_type, [tr])
  }
  const byName = new Map(ct.message_types.map((mt) => [mt.name, mt]))
  const stateOf = new Map(s.states.map((st) => [st.name, st]))
  return [...new Set(head._actions.transitions ?? [])].flatMap((name) => {
    const list = edges.get(name)
    const mt = byName.get(name)
    if (!list || !mt || mt.persist !== 'log' || !post.has(name)) return []
    const targets = [...new Set(list.map((tr) => tr.to))]
    const finals = targets.map((to) => stateOf.get(to))
    const view: ChannelDecisionView = { key: name, messageType: name, label: mt.label || name }
    const [only, ...more] = targets
    if (only !== undefined && !more.length) view.toLabel = stateLabel(s, only)
    if (
      targets.length > 1 ||
      list.some((tr) => tr.guarded) ||
      mt.properties.some((p) => p.required)
    ) {
      view.opensForm = true
    }
    // Only a one-click decision asks; a form decision asks nothing, since
    // its own Send is the confirmation. A one-click decision has one target.
    if (!view.opensForm && view.toLabel !== undefined && finals.every((st) => st?.final)) {
      view.confirm = true
      if (finals.every((st) => st?.status === 'cancelled' || st?.status === 'failed')) {
        view.danger = true
      }
    }
    return [view]
  })
}

/**
 * The seq of the message that moved the session into its current state,
 * when the held page has it: the `cause` of the session notice of the
 * current generation, if that message is held and not removed. Undefined
 * at generation 0, for a timer's move (its notice has no cause), and when
 * the notice or the message is not held. One pass over the held messages,
 * newest first; no fetch.
 */
export function decisionCause(
  messages: readonly Message[],
  session: Pick<ChannelSession, 'generation'> | undefined
): number | undefined {
  if (!session || !session.generation) return undefined
  let cause: number | undefined
  for (let i = messages.length - 1; i >= 0; i--) {
    const m = messages[i]
    if (!m) continue
    if (cause === undefined) {
      if (!isSessionNotice(m) || m.props?.generation !== session.generation) continue
      const c = m.props?.cause
      if (typeof c !== 'number') return undefined
      cause = c
      continue
    }
    if (m.seq < cause) return undefined
    if (m.seq === cause) return m.removed ? undefined : cause
  }
  return undefined
}

/** The head fields the choices read. */
export type ChoiceHead = Pick<ChannelHead, 'channel'> & { _actions: Pick<ChannelActions, 'post'> }

/**
 * The reply buttons of the questions open to an answer, keyed by the
 * question's seq. A question is a held message whose type has `choices`,
 * with a non-empty list in its `from` property. It gets one button per
 * option, which posts the answer type with the option in `into`, and with
 * `other` a button that opens the composer.
 *
 * There are none while newer messages exist than the held ones
 * (`hasNewer`), when the channel is not open, or when the caller may not
 * post the answer type. A question must not be removed, must be top-level
 * (a reply to a reply would address the thread's author, not the asker),
 * and must have no held reply of the answer type: once an answer is held,
 * its buttons go. A removed answer does not count, so a moderator who
 * removes a wrong answer brings the buttons back.
 *
 * In a channel with a session, only the question that moved the session
 * into its current state is open (`decisionCause`), and only while the
 * held `_actions` were fetched at the head's generation. That proof is
 * what makes the click's `expect_generation` the question's own: a click
 * after the session moved on is refused with session_moved.
 */
export function choiceViews(
  ct: ChannelTypeSchema,
  head: ChoiceHead,
  actionsGeneration: number | undefined,
  hasNewer: boolean,
  messages: readonly Message[]
): Map<number, ChannelDecisionView[]> {
  const out = new Map<number, ChannelDecisionView[]>()
  if (hasNewer || head.channel.state !== 'open') return out
  const byName = new Map(ct.message_types.map((mt) => [mt.name, mt]))
  const post = new Set(head._actions.post)
  let candidates: readonly Message[] = messages
  if (ct.session) {
    const cs = head.channel.session
    if (!cs || actionsGeneration !== cs.generation) return out
    const cause = decisionCause(messages, cs)
    const m = cause === undefined ? undefined : messages.find((x) => x.seq === cause)
    candidates = m ? [m] : []
  }
  const answered = new Set<string>()
  for (const m of messages) {
    if (m.reply_to && m.type && !m.removed) answered.add(`${m.type}:${m.reply_to}`)
  }
  for (const m of candidates) {
    const c = m.type ? byName.get(m.type)?.choices : undefined
    if (!c || m.removed || m.reply_to || !post.has(c.answer)) continue
    if (answered.has(`${c.answer}:${m.seq}`)) continue
    const options = m.props?.[c.from]
    if (!Array.isArray(options) || !options.length) continue
    const views: ChannelDecisionView[] = options.map((option, i) => ({
      key: `choice:${m.seq}:${i}`,
      messageType: c.answer,
      label: option,
      props: { [c.into]: option },
    }))
    if (c.other) {
      views.push({
        key: `choice:${m.seq}:other`,
        messageType: c.answer,
        label: t('channels.choiceOther'),
        opensForm: true,
      })
    }
    out.set(m.seq, views)
  }
  return out
}

/**
 * The decisions the row of `decisionSeq` shows. When that message is also
 * a question with buttons, the decisions of its answer type are left out:
 * its choices post that type already. The stepper keeps them.
 */
export function rowDecisions(
  decisions: ChannelDecisionView[],
  choices: ReadonlyMap<number, ChannelDecisionView[]>,
  decisionSeq: number | undefined
): ChannelDecisionView[] {
  const own = decisionSeq === undefined ? undefined : choices.get(decisionSeq)
  if (!own?.length) return decisions
  const answers = new Set(own.map((c) => c.messageType))
  return decisions.filter((d) => !answers.has(d.messageType))
}

/**
 * The `reply_to` of a decision: the deciding message, when it is known, is
 * itself top-level, and the decision's type may reply. Undefined otherwise.
 */
export function decisionReplyTo(
  messages: readonly Message[],
  cause: number | undefined,
  mt: Pick<ChannelMessageTypeSchema, 'reply'> | undefined
): number | undefined {
  if (cause === undefined || !mt?.reply) return undefined
  const m = messages.find((x) => x.seq === cause)
  return m && !m.reply_to ? cause : undefined
}

/**
 * Drops the memo entries of revisions no longer held: an edited message's
 * old rev and messages the held-list cap let go. The memo then holds at
 * most one entry per held message and markdown prop (and page world).
 */
export function pruneMemo(memo: MarkdownMemo, messages: readonly Message[]) {
  const held = new Set(messages.map((m) => `${m.seq}:${m.rev}`))
  for (const key of memo.keys()) {
    const [seq, rev] = key.split(':', 2)
    if (!held.has(`${seq}:${rev}`)) memo.delete(key)
  }
}

/** Maps a page of messages, quoting each reply from the held list. */
export function messageViews(
  messages: readonly Message[],
  messageType: (name: string) => ChannelMessageTypeSchema | undefined,
  ctx: ChannelViewContext,
  memo?: MarkdownMemo
): ChannelMessageView[] {
  if (memo) pruneMemo(memo, messages)
  const bySeq = new Map(messages.map((m) => [m.seq, m]))
  return messages.map((m) => {
    const original = m.reply_to ? bySeq.get(m.reply_to) : undefined
    const view = messageView(m, m.type ? messageType(m.type) : undefined, ctx, memo)
    if (m.reply_to) {
      view.replyTo = replyQuote(
        m.reply_to,
        original,
        original?.type ? messageType(original.type) : undefined
      )
    }
    return view
  })
}

/**
 * The text after the type label on an ephemeral line, in presentation
 * order: times in the user's zone, enum labels, readable refs by title.
 */
export function ephemeralText(
  e: EphemeralMessage,
  mt: ChannelMessageTypeSchema | undefined,
  presentation: ChannelPresentation | undefined,
  enumOptions: Record<string, ChannelEnumOption[]>,
  tz: string
): string {
  if (!mt) return ''
  const parts: string[] = []
  for (const f of resolveChannelFields(mt, presentation?.messages?.[mt.name])) {
    const p = f.property
    const v = e.props?.[p.name]
    if (v === undefined) continue
    if (Array.isArray(v)) {
      parts.push(v.join(', '))
    } else if (typeof v === 'object') {
      parts.push(v.restricted ? t('channels.restrictedRef') : (v.title ?? v.id))
    } else if (typeof v === 'boolean') {
      parts.push(v ? t('common.yes') : t('common.no'))
    } else {
      const opt = p.enum ? enumOptions[p.enum]?.find((o) => o.value === v) : undefined
      parts.push(opt?.label ?? formatted(p as ChannelPropertySchema, v, tz) ?? String(v))
    }
  }
  return parts.join(' · ')
}

/** The signals still live at `now`, as the library draws them. */
export function ephemeralViews(
  signals: readonly EphemeralMessage[],
  now: number,
  messageType: (name: string) => ChannelMessageTypeSchema | undefined,
  presentation: ChannelPresentation | undefined,
  enumOptions: Record<string, ChannelEnumOption[]>,
  tz: string
): ChannelEphemeralView[] {
  return signals
    .filter((e) => Date.parse(e.expires_at) > now)
    .map((e) => ({
      id: e.id,
      type: e.type,
      author: authorView(e.author),
      props: e.props,
      text: ephemeralText(e, messageType(e.type), presentation, enumOptions, tz),
    }))
}

/**
 * Who else is typing, as the library draws it: exactly the head's
 * `typing_now`, in its order. There is no expiry here; the server expires
 * an entry and sends a new head.
 *
 * The server holds one entry per user and organization, and the wire does
 * not carry the organization. So the key is the user and `since`, with a
 * count appended when two entries still share it (one user typing from two
 * organizations in the same instant), so keys never repeat.
 */
export function typingViews(entries: readonly TypingEntry[] | undefined): ChannelTypingView[] {
  const seen = new Map<string, number>()
  return (entries ?? []).map((e) => {
    const base = `${e.author.user}\u0000${e.since}`
    const n = seen.get(base) ?? 0
    seen.set(base, n + 1)
    return { key: n === 0 ? base : `${base}\u0000${n}`, author: authorView(e.author) }
  })
}

/** The soonest `expires_at` after `now`, or undefined when none is left. */
export function nextExpiry(signals: readonly EphemeralMessage[], now: number): number | undefined {
  let soonest: number | undefined
  for (const e of signals) {
    const at = Date.parse(e.expires_at)
    if (at > now && (soonest === undefined || at < soonest)) soonest = at
  }
  return soonest
}

/**
 * The composer prefill for editing m. Restricted refs are named in
 * `restricted`, so the composer shows the placeholder and the edit leaves
 * them to the server (see toEditProps).
 */
export function editingFor(m: Message, mt: ChannelMessageTypeSchema): ChannelEditing {
  const values: ChannelEditing['values'] = {}
  const refs: Record<string, ChannelReadableRef> = {}
  const restricted: string[] = []
  for (const p of mt.properties) {
    const v = m.props?.[p.name]
    values[p.name] = fromPropValue(p, v)
    if (p.type !== 'ref' || typeof v !== 'object' || v === null || Array.isArray(v)) continue
    if (v.restricted) restricted.push(p.name)
    else refs[p.name] = { id: v.id, type: v.type, title: v.title }
  }
  return { seq: m.seq, type: mt.name, values, refs, restricted }
}

/** Enum labels and colours for every enum property of a channel type, by enum type. */
export function enumOptionsFor(
  ct: ChannelTypeSchema | undefined,
  customTypes: ReadonlyMap<string, CustomType>,
  styles: Readonly<Record<string, Record<string, string>>>
): Record<string, ChannelEnumOption[]> {
  const out: Record<string, ChannelEnumOption[]> = {}
  for (const mt of ct?.message_types ?? []) {
    for (const p of mt.properties) {
      if (p.type !== 'enum' || !p.enum || out[p.enum]) continue
      const labels = customTypes.get(p.enum)?.labels ?? {}
      const style = styles[p.enum] ?? {}
      out[p.enum] = (p.values ?? []).map((value) => ({
        value,
        label: labels[value] ?? value,
        // A style class is `badge-<hue>`, `kit-<hue>-<shade>` or
        // `custom-<name>`; a chip draws the hue's tag.
        color: style[value]
          ? tagColor(style[value].replace(/^badge-/, '').replace(/^kit-([a-z]+)-\d+$/, '$1'))
          : undefined,
      }))
    }
  }
  return out
}

/** The list key of a channel; a `one` type has no name. */
export function channelKey(type: string, name?: string): string {
  return `${type}/${name ?? ''}`
}

function channelTitle(c: Channel, label: string): string {
  return c.title || c.name || label
}

/**
 * The channel list as the library draws it. A `one` type whose channel was
 * never written gets a row of its own, so the first message can be posted:
 * the server opens that channel on the first post.
 */
export function channelGroups(
  types: readonly ChannelTypeList[],
  schemaType: (name: string) => ChannelTypeSchema | undefined
): ChannelGroupView[] {
  return types.map((t) => {
    const ct = schemaType(t.type)
    const label = t.label || ct?.label || t.type
    const rows: ChannelRowView[] = t.channels.map((c) => ({
      key: channelKey(t.type, c.name),
      title: channelTitle(c, label),
      state: c.state,
      ...(c.read?.unread ? { unread: c.read.unread } : {}),
    }))
    if (t.cardinality === 'one' && rows.length === 0) {
      rows.push({ key: channelKey(t.type), title: label, state: 'open' as const })
    }
    return {
      type: t.type,
      label,
      cardinality: t.cardinality,
      canCreate: t.create,
      createLabel: ct?.create_label,
      channels: rows,
    }
  })
}

/** The channel type and name a list key names. */
export function parseChannelKey(key: string): { channelType: string; name?: string } {
  const at = key.indexOf('/')
  const channelType = at < 0 ? key : key.slice(0, at)
  const name = at < 0 ? '' : key.slice(at + 1)
  return name ? { channelType, name } : { channelType }
}
