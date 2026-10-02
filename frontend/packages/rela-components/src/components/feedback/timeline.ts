import type { IconName } from '../common/icons'
import type { StatusColor } from '../../types'

/**
 * One thing that happened.
 *
 * Deliberately not a union of known event kinds. A typed graph's event
 * vocabulary is the app's: rela records relation changes and property edits,
 * another deployment records deploys and approvals, and a library that enumerated
 * either would be wrong for the other and would need a release to add a kind.
 *
 * So an entry arrives already decided: the app chose the icon, the tone and the
 * wording, and this component lays them out.
 */
export interface TimelineEntry {
  id: string
  /**
   * What happened, as a sentence fragment the app already composed: `moved this
   * to In review`, `added the control ISO-27001.A.9`.
   *
   * A string rather than parts, because the word order of "who did what to
   * which thing" differs by language and the app is the only party that knows
   * which of those it is showing. Use the `entry` slot for anything that needs
   * markup, such as a link to the thing that changed.
   */
  summary?: string
  /** Who did it. Omitted for something the system did. */
  actor?: string
  /** The actor's photo, where there is one. */
  actorAvatarUrl?: string
  /**
   * When, already formatted: `2 hours ago`, `18 Sep 14:02`.
   *
   * Formatted by the app for the same reason the calendar takes pre-formatted
   * times: choosing between a relative and an absolute form needs a display
   * timezone and a locale, and both belong to the app.
   */
  timestamp?: string
  /**
   * The machine-readable instant, for the `<time datetime>` attribute. Optional,
   * and worth passing: it is what lets a screen reader or a crawler read the
   * real date behind "2 hours ago", which is otherwise unrecoverable.
   */
  datetime?: string
  /** The glyph on the rail. Falls back to a plain dot. */
  icon?: IconName
  /**
   * The rail marker's colour, for an event whose kind carries meaning: red for a
   * deletion, green for an approval. Grey by default, because most events are
   * simply things that happened.
   */
  tone?: StatusColor
  /**
   * Longer content under the summary: a comment's body, a diff, a reason. Kept
   * as a plain string; use the `detail` slot for anything richer.
   */
  detail?: string
  /**
   * Whether this entry is the current state rather than history — the newest
   * event, or the one a deep link points at. Marked, and reported with
   * `aria-current`.
   */
  current?: boolean
}

/**
 * A run of entries under one heading, usually a day.
 *
 * Grouping is the app's, not the timeline's: bucketing by day needs a display
 * timezone, which is the same boundary `calendarGrid.ts` draws. A flat list is
 * one group with no label.
 */
export interface TimelineGroup {
  id: string
  /** The heading, such as `Today` or `18 September`. Omit for an unlabelled run. */
  label?: string
  entries: TimelineEntry[]
}
