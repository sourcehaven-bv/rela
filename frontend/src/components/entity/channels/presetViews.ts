/**
 * The preset buttons of session transition entries (TKT-S9TLJJ), for
 * decisionViews in channelView.ts. A file of its own because channelView.ts
 * is over the max-lines warning.
 */
import type {
  ChannelButtonOptions,
  ChannelSessionStateSchema,
  ChannelSessionTransition,
} from '@/types/channels'
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

/**
 * A decision view with what only the app's confirm dialog reads. Each
 * field is set only when an authored option (TKT-TXZZZ2) causes it.
 */
export interface AppDecisionView extends ChannelDecisionView {
  /** The authored question, shown as written in the dialog. */
  confirmText?: string
  /** An authored confirm into a state that is not final: the dialog says the session moves. */
  movesOnly?: true
  /** An authored style dropped the derived danger; the dialog stays red. */
  outcomeDanger?: true
  /**
   * A preset button with `compose: true`: it opens the composer with its
   * props fixed (TKT-53FN0L). Set with opensForm.
   */
  compose?: true
}

/**
 * Applies a button's authored options over the derived look. On a
 * one-click button (oneClick), derived is the result of endTone and each
 * option overrides only its own part: `confirm` true or a text asks into
 * any target, false never asks; `style` danger draws it red, primary marks
 * it primary and drops a derived danger, default drops a derived danger.
 * On a button that opens the composer only `style` applies, since Send is
 * the confirmation. Without options it adds derived and nothing else, so
 * it never writes a key whose value is false or undefined.
 */
export function applyOptions(
  view: AppDecisionView,
  options: ChannelButtonOptions | undefined,
  oneClick: boolean,
  derived: Pick<ChannelDecisionView, 'confirm' | 'danger'>
): AppDecisionView {
  const out: AppDecisionView = { ...view }
  let confirm = oneClick && !!derived.confirm
  let danger = oneClick && !!derived.danger
  if (oneClick && options?.confirm !== undefined) confirm = options.confirm
  const style = options?.style
  const dropped = danger && (style === 'primary' || style === 'default')
  if (style === 'danger') danger = true
  else if (dropped) danger = false
  if (style === 'primary') out.primary = true
  if (confirm) {
    out.confirm = true
    if (options?.confirm && options.confirm_text) out.confirmText = options.confirm_text
    if (options?.confirm && !derived.confirm) out.movesOnly = true
    if (dropped) out.outcomeDanger = true
  }
  if (danger) out.danger = true
  return out
}

/** The preset views of one message type, and what is left for its type button. */
export interface PresetSplit {
  /** One view per listed entry with a button, in entry order. */
  views: AppDecisionView[]
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
  const views: AppDecisionView[] = []
  const rest: ChannelSessionTransition[] = []
  for (const tr of listed) {
    if (!tr.button) {
      rest.push(tr)
      continue
    }
    // A preset is one-click, or opens the composer with its props fixed
    // when it has compose. Its row holds its options with the type's
    // decision_button merged in; on a compose preset only style applies.
    const view: AppDecisionView = {
      key: `preset:${name}:${entryOf(tr)}`,
      messageType: name,
      label: tr.button.label,
      props: { ...tr.button.props },
      toLabel: stateOf.get(tr.to)?.label ?? tr.to,
    }
    if (tr.button.compose) {
      views.push(applyOptions({ ...view, opensForm: true, compose: true }, tr.button, false, {}))
      continue
    }
    views.push(applyOptions(view, tr.button, true, endTone([tr.to], stateOf)))
  }
  const guardedBefore = rest.some((r) => rows.some((tr) => tr.guarded && entryOf(tr) < entryOf(r)))
  return { views, rest, guardedBefore }
}
