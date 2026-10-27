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
import { formatDate, formatDatetime } from '@/utils/format'
import { renderMessageMarkdown, type MessageRefLink } from '@/utils/markdown'
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
  ChannelSessionSchema,
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
  ChannelEditing,
  ChannelEnumOption,
  ChannelEphemeralView,
  ChannelGroupView,
  ChannelLifecycleDef,
  ChannelMessageView,
  ChannelNoticeView,
  ChannelPresentation,
  ChannelReadableRef,
  ChannelReplyQuote,
  ChannelSessionView,
  ChannelTypingView,
} from 'rela-components/components/channel/types'
import { resolveChannelFields } from 'rela-components/components/channel/channelFields'

/** The longest reply excerpt, in code points. */
export const EXCERPT_MAX = 80

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
  if (a.system) return { name: a.user || 'system', kind: 'system' }
  if (a.principal_type === 'agent' || a.tool === 'mcp') return { name: a.user, kind: 'agent' }
  return { name: a.user, kind: 'person' }
}

/** The lifecycle labels; delete draws in the danger tone. */
export function lifecycleDefs(ct: ChannelTypeSchema): ChannelLifecycleDef[] {
  return ct.lifecycle.map((op) => ({ ...op, danger: op.name === LIFECYCLE_DELETE }))
}

function isReadableRef(v: MessagePropValue | undefined): v is ReadableRef {
  return typeof v === 'object' && v !== null && v.restricted !== true
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
 * prop, collapsed to one line. It is rendered as text, so markdown syntax
 * shows as written.
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
    const v = m.props?.[name]
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
  const html = renderMessageMarkdown(src, { refs, world: ctx.world })
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
 */
export function composerTransitions(
  ct: ChannelTypeSchema,
  head: Pick<ChannelHead, 'channel'> & { _actions: Pick<ChannelActions, 'post' | 'transitions'> }
): ChannelComposerTransition[] {
  const cs = head.channel.session
  if (!ct.session || !cs || cs.stranded) return []
  if (!ct.session.states.some((st) => st.name === cs.state)) return []
  const post = new Set(head._actions.post)
  const out = new Map((cs.transitions ?? []).map((tr) => [tr.message_type, tr]))
  return (head._actions.transitions ?? []).flatMap((name) => {
    const tr = out.get(name)
    if (!tr || !post.has(name)) return []
    const view: ChannelComposerTransition = {
      messageType: name,
      toLabel: stateLabel(ct.session, tr.to),
    }
    if (tr.guarded) view.guarded = true
    return [view]
  })
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
    if (typeof v === 'object') {
      parts.push(v.restricted ? 'Restricted reference' : (v.title ?? v.id))
    } else if (typeof v === 'boolean') {
      parts.push(v ? 'Yes' : 'No')
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
    if (p.type !== 'ref' || typeof v !== 'object' || v === null) continue
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
    const rows = t.channels.map((c) => ({
      key: channelKey(t.type, c.name),
      title: channelTitle(c, label),
      state: c.state,
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
