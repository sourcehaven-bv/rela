/**
 * Folds the held messages of a channel into the rows the log renders, for
 * message types the presentation marks `collapsed` or `hidden`
 * (`ChannelMessagePresentation.display`). A pure function: no DOM, no
 * reactivity.
 *
 * Terms:
 *
 * - A quiet type is a message type with a display value.
 * - A quiet notice is a session notice into a `working` state
 *   (`notice.toStatus`). A notice into a state that waits, a final state or
 *   a state the app does not know is not quiet.
 * - A run is a sequence of adjacent messages of quiet types with no other
 *   row between them. A hidden message continues a run. A removed message
 *   or a quiet notice between two members of a run joins it, unless it is
 *   kept. Any other notice, a kept row or any other message ends a run. A
 *   quiet notice with no open run, or after the last member, is a row of its
 *   own, so the state change that follows a shown message stays in view.
 * - A kept row is a message of a quiet type that is shown in full anyway:
 *   its seq is in `keep` (the decision cause, a question with choices, a
 *   reply target the reader asked for, or a row kept earlier), or its
 *   `actions` hold a message-type action.
 *
 * The collapsed and removed messages and the notices of a run become one
 * fold row; its hidden messages become no row. A run without collapsed
 * messages becomes no row, except that its removed messages and notices
 * stay rows of their own.
 */
import { CHANNEL_OPERATIONS } from './channelFields'
import type { ChannelMessageDisplay, ChannelMessageView } from './types'

/** One message, shown in full. */
export interface ChannelMessageRow {
  kind: 'message'
  message: ChannelMessageView
}

/** The collapsed messages of one run, behind a fold line. */
export interface ChannelFoldRow {
  kind: 'fold'
  /**
   * The Vue key, from the run's first seq. A live append changes only the
   * end of a run, so the fold and its expanded rows survive it.
   */
  key: string
  /** The seq of the run's first member, hidden ones included. */
  first: number
  /** The seq of the run's last member: the fold's scroll anchor. */
  last: number
  /** Every member of the run, hidden ones included; expanding adds them all. */
  seqs: number[]
  /** The messages the fold shows when expanded: collapsed, removed and notices. */
  messages: ChannelMessageView[]
  /** How many of `messages` are not session notices. */
  count: number
  /** How many of `messages` are session notices: the session steps. */
  moves: number
  /** The type labels of the collapsed messages, in first-seen order. */
  labels: string[]
  expanded: boolean
}

export type ChannelRow = ChannelMessageRow | ChannelFoldRow

export interface FoldOptions {
  /** The display of a message type; undefined shows it in full. */
  displayOf: (type: string) => ChannelMessageDisplay | undefined
  /** The label of a message type, for the fold line. */
  labelOf: (type: string) => string
  /** Every message is one row, as without quiet types. */
  showAll: boolean
  /** Seqs that are kept rows. */
  keep: ReadonlySet<number>
  /** Seqs of expanded runs; a run is expanded when any of its seqs is here. */
  expanded: ReadonlySet<number>
}

/** Whether the message offers an action of its type, such as a promote. */
export function hasTypeAction(m: ChannelMessageView): boolean {
  return m.actions?.some((a) => !CHANNEL_OPERATIONS.has(a)) ?? false
}

/** The display of m in the log with the toggle off: undefined is a full row. */
function quietDisplay(m: ChannelMessageView, opts: FoldOptions): ChannelMessageDisplay | undefined {
  if (m.notice || !m.type || m.removed) return undefined
  if (opts.keep.has(m.seq) || hasTypeAction(m)) return undefined
  return opts.displayOf(m.type)
}

/** Whether m is a quiet notice that is not kept: it may join a run. */
function quietNotice(m: ChannelMessageView, opts: FoldOptions): boolean {
  return !m.removed && m.notice?.toStatus === 'working' && !opts.keep.has(m.seq)
}

/** Turns the held messages, oldest first, into the rows to render. */
export function foldMessages(messages: readonly ChannelMessageView[], opts: FoldOptions): ChannelRow[] {
  if (opts.showAll) return messages.map((message) => ({ kind: 'message', message }))
  const rows: ChannelRow[] = []
  let run: { members: ChannelMessageView[]; collapsed: number } | undefined
  /** Removed messages and quiet notices after the run, waiting to see whether it goes on. */
  let pending: ChannelMessageView[] = []

  const closeRun = () => {
    if (run && run.collapsed > 0) rows.push(foldRow(run.members, opts))
    // Without a fold line, the run's removed messages and notices stay rows of their own.
    else
      for (const message of run?.members ?? [])
        if (message.removed || message.notice) rows.push({ kind: 'message', message })
    run = undefined
    for (const message of pending) rows.push({ kind: 'message', message })
    pending = []
  }

  for (const m of messages) {
    if (run && ((m.removed && !m.notice && !opts.keep.has(m.seq)) || quietNotice(m, opts))) {
      pending.push(m)
      continue
    }
    const display = quietDisplay(m, opts)
    if (!display) {
      closeRun()
      rows.push({ kind: 'message', message: m })
      continue
    }
    run ??= { members: [], collapsed: 0 }
    run.members.push(...pending, m)
    pending = []
    if (display === 'collapsed') run.collapsed++
  }
  closeRun()
  return rows
}

function foldRow(members: ChannelMessageView[], opts: FoldOptions): ChannelFoldRow {
  const shown = members.filter(
    (m) => m.removed || m.notice || (m.type && opts.displayOf(m.type) === 'collapsed'),
  )
  const moves = shown.filter((m) => m.notice).length
  const labels: string[] = []
  for (const m of shown) {
    if (!m.type || m.notice) continue
    const label = opts.labelOf(m.type)
    if (!labels.includes(label)) labels.push(label)
  }
  const seqs = members.map((m) => m.seq)
  const first = seqs[0]!
  return {
    kind: 'fold',
    key: `fold-${first}`,
    first,
    last: seqs[seqs.length - 1]!,
    seqs,
    messages: shown,
    count: shown.length - moves,
    moves,
    labels,
    expanded: seqs.some((s) => opts.expanded.has(s)),
  }
}

/** The first seq a row covers. */
function startOf(row: ChannelRow): number {
  return row.kind === 'message' ? row.message.seq : row.first
}

/**
 * The rendered row that holds seq: its message row, the fold whose run
 * contains it, or, for a seq that renders nothing, the first row after it.
 * Undefined when no row is at or after seq.
 */
export function rowFor(rows: readonly ChannelRow[], seq: number): ChannelRow | undefined {
  for (const row of rows) {
    if (row.kind === 'fold' && row.first <= seq && seq <= row.last) return row
    if (startOf(row) >= seq) return row
  }
  return undefined
}

/**
 * How the message with seq renders: `full` as its own row or inside an
 * expanded fold, `folded` behind a collapsed fold line, undefined when it
 * renders nothing.
 */
export function shownAs(rows: readonly ChannelRow[], seq: number): 'full' | 'folded' | undefined {
  const row = rowFor(rows, seq)
  if (!row) return undefined
  if (row.kind === 'message') return row.message.seq === seq ? 'full' : undefined
  if (!row.messages.some((m) => m.seq === seq)) return undefined
  return row.expanded ? 'full' : 'folded'
}
