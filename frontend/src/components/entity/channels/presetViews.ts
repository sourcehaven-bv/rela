/**
 * The preset buttons of session transition entries (TKT-S9TLJJ), for
 * decisionViews in channelView.ts. A file of its own because channelView.ts
 * is over the max-lines warning.
 */
import type { ChannelSessionStateSchema, ChannelSessionTransition } from '@/types/channels'
import type { ChannelDecisionView } from 'rela-components/components/channel/types'

/** The states of the session descriptor, by name. */
export type StateOf = ReadonlyMap<string, ChannelSessionStateSchema>

/**
 * How a one-click post to targets is drawn: it asks first (`confirm`) when
 * every target is final, and is `danger` when every target ends cancelled
 * or failed. The rule of the type button and of a preset button.
 */
export function endTone(
  targets: readonly string[],
  stateOf: StateOf
): Pick<ChannelDecisionView, 'confirm' | 'danger'> {
  const finals = targets.map((to) => stateOf.get(to))
  if (!finals.length || !finals.every((st) => st?.final)) return {}
  if (finals.every((st) => st?.status === 'cancelled' || st?.status === 'failed')) {
    return { confirm: true, danger: true }
  }
  return { confirm: true }
}

/** The preset views of one message type, and what is left for its type button. */
export interface PresetSplit {
  /** One view per listed entry with a button, in entry order. */
  views: ChannelDecisionView[]
  /** The listed entries without a button; the type button stands for them. */
  rest: ChannelSessionTransition[]
  /**
   * A person-fired entry before one in rest is guarded. A one-click post
   * could be taken by that entry, so the type button opens the form.
   */
  guardedBefore: boolean
}

const entryOf = (tr: ChannelSessionTransition) => tr.entry ?? 0

/**
 * Splits the person-fired rows of message type name. entries are the
 * entries of the type in the caller's `_actions.entries`; undefined when
 * the server sent none, and then every row is left for the type button and
 * no preset shows, since only the server knows whose click an entry takes.
 * A listed entry without a served row shows nothing.
 */
export function presetViews(
  name: string,
  rows: readonly ChannelSessionTransition[],
  entries: ReadonlySet<number> | undefined,
  stateOf: StateOf
): PresetSplit {
  if (!entries) return { views: [], rest: [...rows], guardedBefore: false }
  const listed = rows
    .filter((tr) => entries.has(entryOf(tr)))
    .sort((a, b) => entryOf(a) - entryOf(b))
  const views: ChannelDecisionView[] = []
  const rest: ChannelSessionTransition[] = []
  for (const tr of listed) {
    if (!tr.button) {
      rest.push(tr)
      continue
    }
    views.push({
      key: `preset:${name}:${entryOf(tr)}`,
      messageType: name,
      label: tr.button.label,
      props: { ...tr.button.props },
      toLabel: stateOf.get(tr.to)?.label ?? tr.to,
      ...endTone([tr.to], stateOf),
    })
  }
  const guardedBefore = rest.some((r) => rows.some((tr) => tr.guarded && entryOf(tr) < entryOf(r)))
  return { views, rest, guardedBefore }
}
