/**
 * The library's own strings, and the seam an app translates them through.
 *
 * Most text in this library is already the app's: a label, a placeholder, an
 * empty state and a button's caption all arrive as props, so they are in
 * whatever language the app passed. What is left are the strings a component
 * composes itself, and they divide into two kinds:
 *
 * - Text with a value in it, such as `3 items selected`. A prop cannot carry
 *   this, because the count is not known until render and the word order around
 *   it differs by language.
 * - Screen-reader announcements, such as what a keyboard drag is doing. These
 *   have no prop because they have no visible element to hang one on.
 *
 * The second kind is why this exists. An English label in a German app is
 * reported on the first screenshot; an English announcement inside an
 * `aria-live` region is heard only by the users least likely to be asked, and
 * looks perfect to everyone reviewing it. So the fix has to cover the strings
 * nobody sees, not just the ones that are easy to spot.
 *
 * ## How an app uses it
 *
 * Provide a partial set at the root, once:
 *
 * ```ts
 * import { provideMessages } from 'rela-components'
 *
 * provideMessages({
 *   selectedCount: ({ count, noun }) => `${count} ${noun} geselecteerd`,
 *   cardGrabbed: ({ title }) => `${title}, opgepakt. ...`,
 * })
 * ```
 *
 * Anything not overridden keeps its English default, so an app translates the
 * strings it cares about and adds the rest later without a version where the
 * interface is half-empty.
 *
 * ## Why functions rather than templates with placeholders
 *
 * A `'{count} items selected'` string forces every consumer to pick an
 * interpolation syntax, and it cannot express agreement: `1 item` against `2
 * items` is the easy case, and Polish has three plural forms. A function takes
 * the values and returns the finished string, so a translator with a plural rule
 * or a different word order can express it, and there is nothing to parse.
 *
 * It also means an app already holding an i18n library can delegate:
 * `selectedCount: (args) => t('table.selected', args)`.
 *
 * ## Why inject rather than module state
 *
 * A module-level record would be one language per page, which is usually true
 * but not always — a translator's side-by-side view, or an admin screen pinned
 * to English inside a localised app. Provide/inject also keeps components pure
 * in the sense `check:purity` means: the strings arrive through the component
 * tree rather than being read from ambient state, so a component renders the
 * same way given the same inputs.
 */
import { inject, provide, type InjectionKey } from 'vue'

/** A noun and its plural, for the strings that count something. */
export interface CountArgs {
  count: number
  /** The singular noun, as the consumer named it: `task`, `row`, `comment`. */
  noun: string
  /** The plural, which a caller supplies when it is not `noun` plus `s`. */
  pluralNoun?: string
}

/**
 * Every string this library composes rather than receiving.
 *
 * Each entry is a function so a translation can reorder, inflect and pluralise
 * freely. Keep them short: these are interface strings, not documentation.
 */
export interface Messages {
  /** The bulk action bar's tally: `3 tasks selected`. */
  selectedCount: (args: CountArgs) => string

  /** A search or filter's result tally: `4 of 120`. */
  resultCount: (args: { shown: number; total: number }) => string

  /** Announced when a card is picked up for a keyboard move. */
  cardGrabbed: (args: { title: string }) => string
  /** Announced for a card at rest, describing how to pick it up. */
  cardAtRest: (args: { title: string; section?: string }) => string
  /** Announced while a grabbed card is over a target column. */
  cardOverTarget: (args: { title: string; target: string }) => string
  /** Announced while a grabbed card is over a target cell in a swimlane board. */
  cardOverSwimlaneTarget: (args: { title: string; target: string; lane: string }) => string

  /** A sortable row's handle, at rest: `Reorder Title, 2 of 5`. */
  sortHandle: (args: { title: string; position: number; count: number }) => string
  /** Announced when a sortable row is picked up or moved while held. */
  sortGrabbed: (args: { title: string; position: number; count: number }) => string
  /** Announced when a held row is put down. */
  sortDropped: (args: { title: string; position: number; count: number }) => string
  /** Announced when a held row is put back where it started. */
  sortCancelled: (args: { title: string; position: number; count: number }) => string

  /** The kind of a change in a change list, read before its label: `Added`. */
  changeKind: (args: { kind: 'added' | 'changed' | 'removed' }) => string
  /** Read before a change's old value, which is otherwise only struck through: `from`. */
  changeBefore: () => string
  /** Read before a change's new value: `to`. */
  changeAfter: () => string

  /** A collapsed column's expand control: `Expand Done`. */
  expandSection: (args: { title: string }) => string
  /** A section's add control, named for the section: `Add task to Done`. */
  addToSection: (args: { addLabel: string; section: string; lane?: string }) => string

  /** The comment affordance on an uncommented block: `Comment on this paragraph`. */
  commentOnBlock: (args: { kind: string }) => string
  /** The same once a block has comments: `2 comments on this paragraph`. */
  commentCountOnBlock: (args: CountArgs & { kind: string }) => string
  /** The comment affordance against a named anchor. */
  commentOnAnchor: (args: { anchor: string }) => string
  /** The same once that anchor has comments. */
  commentCountOnAnchor: (args: CountArgs & { anchor: string }) => string
  /** How a section anchor is named when the app gave no label. */
  sectionAnchor: (args: { reference: string }) => string

  /** A pane resizer's value, for a screen reader: `280 pixels`. */
  resizerValue: (args: { pixels: number }) => string

  /** A file field's remove control: `Remove report.pdf`. */
  removeFile: (args: { name: string }) => string
  /** A file's size, already rounded to a unit. */
  fileSize: (args: { bytes: number }) => string

  /** The sentence appended to a date field's hint naming its time zone. */
  timeZoneNote: (args: { zone: string }) => string

  /** A filter's empty result, naming the query. */
  noMatches: (args: { query: string }) => string

  /** A disclosure or nav item's toggle: `Expand Projects` / `Collapse Projects`. */
  toggleDisclosure: (args: { label: string; open: boolean }) => string

  /** An inline-edit field's control: `Title, edit`. */
  editField: (args: { label: string }) => string

  /** A pagination control: `Page 3`. */
  pageNumber: (args: { page: number }) => string

  /** A filter chip's remove control: `Remove filter Status: Open`. */
  removeFilter: (args: { label: string; value?: string }) => string
}

/**
 * English, and the fallback for anything an app does not override.
 *
 * These are the strings that were written inline in the components before this
 * file existed, moved here unchanged: the wording is already what the
 * interaction tests and the stories assert, so centralising it is not the place
 * to also revise it.
 */
export const DEFAULT_MESSAGES: Messages = {
  selectedCount: ({ count, noun, pluralNoun }) =>
    `${count} ${count === 1 ? noun : (pluralNoun ?? `${noun}s`)} selected`,

  resultCount: ({ shown, total }) => `${shown} of ${total}`,

  cardGrabbed: ({ title }) =>
    `${title}, picked up. Use the arrow keys to choose a column, Enter to drop, Escape to cancel.`,
  cardAtRest: ({ title, section }) =>
    `${title}${section ? `, in ${section}` : ''}. Press Enter to pick up.`,
  cardOverTarget: ({ title, target }) =>
    `${title}, over ${target}. Press Enter to drop, Escape to cancel.`,
  cardOverSwimlaneTarget: ({ title, target, lane }) =>
    `${title}, over ${target} in ${lane}. Press Enter to drop, Escape to cancel.`,

  sortHandle: ({ title, position, count }) => `Reorder ${title}, ${position} of ${count}`,
  sortGrabbed: ({ title, position, count }) =>
    `${title}, picked up, ${position} of ${count}. Use the up and down arrows to move it, Enter to drop, Escape to cancel.`,
  sortDropped: ({ title, position, count }) => `${title}, dropped at ${position} of ${count}.`,
  sortCancelled: ({ title, position, count }) => `${title}, put back at ${position} of ${count}.`,

  changeKind: ({ kind }) => ({ added: 'Added', changed: 'Changed', removed: 'Removed' })[kind],
  changeBefore: () => 'from',
  changeAfter: () => 'to',

  expandSection: ({ title }) => `Expand ${title}`,
  addToSection: ({ addLabel, section, lane }) =>
    `${addLabel} to ${section}${lane ? `, ${lane}` : ''}`,

  commentOnBlock: ({ kind }) => `Comment on this ${kind}`,
  commentCountOnBlock: ({ count, noun, pluralNoun, kind }) =>
    `${count} ${count === 1 ? noun : (pluralNoun ?? `${noun}s`)} on this ${kind}`,
  commentOnAnchor: ({ anchor }) => `Comment on ${anchor}`,
  commentCountOnAnchor: ({ count, noun, pluralNoun, anchor }) =>
    `${count} ${count === 1 ? noun : (pluralNoun ?? `${noun}s`)} on ${anchor}`,
  sectionAnchor: ({ reference }) => `Section: ${reference}`,

  resizerValue: ({ pixels }) => `${pixels} pixels`,

  removeFile: ({ name }) => `Remove ${name}`,
  fileSize: ({ bytes }) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`
    return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  },

  timeZoneNote: ({ zone }) => `Times are ${zone}.`,

  noMatches: ({ query }) => `Nothing matches “${query}”.`,

  toggleDisclosure: ({ label, open }) => `${open ? 'Collapse' : 'Expand'} ${label}`,

  editField: ({ label }) => `${label}, edit`,

  pageNumber: ({ page }) => `Page ${page}`,

  removeFilter: ({ label, value }) => `Remove filter ${label}${value ? `: ${value}` : ''}`,
}

const MessagesKey: InjectionKey<Messages> = Symbol('rl-messages')

/**
 * Override some or all of the library's strings for everything below this
 * component. Call it once at the app root.
 *
 * Merged over the defaults rather than replacing them, so a partial set is
 * valid and a string added by a later version of this library has a default
 * instead of rendering as blank.
 */
export function provideMessages(messages: Partial<Messages>): void {
  provide(MessagesKey, { ...DEFAULT_MESSAGES, ...messages })
}

/**
 * The strings, for a component that composes one. Falls back to English when no
 * app provided a set, so every component works with nothing wired.
 */
export function useMessages(): Messages {
  return inject(MessagesKey, DEFAULT_MESSAGES)
}
