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
 * The fixed captions of the channel components. They have no value in them,
 * but there are too many to give each its own prop, so they come through here
 * by key: a translation is one lookup table.
 */
export type ChannelTextKey =
  // Composer
  | 'newMessage'
  | 'editMessage'
  | 'messageType'
  | 'cancelReply'
  | 'required'
  | 'toSend'
  | 'toSave'
  | 'send'
  | 'sending'
  | 'save'
  | 'saving'
  | 'cancel'
  | 'typeGone'
  // Composer ref field
  | 'choose'
  | 'choosing'
  | 'change'
  | 'replace'
  | 'clear'
  // Channel
  | 'closed'
  | 'archived'
  | 'working'
  | 'messages'
  | 'loadOlder'
  | 'loading'
  | 'noMessages'
  | 'jumpToLatest'
  | 'closedNotice'
  | 'archivedNotice'
  // Message
  | 'yes'
  | 'no'
  | 'edited'
  | 'reply'
  | 'edit'
  | 'delete'
  | 'deleting'
  | 'remove'
  | 'removing'
  | 'release'
  | 'releasing'
  // Entity channel list
  | 'showChannels'
  | 'filterOpen'
  | 'filterAll'
  | 'newChannel'
  | 'noChannels'
  // Author badges and ref chips
  | 'system'
  | 'agent'
  | 'restrictedRef'
  | 'restrictedRefTitle'
  // Session stepper and composer marks
  | 'session'
  | 'sessionEnded'
  | 'sessionCompleted'
  | 'sessionCancelled'
  | 'sessionFailed'
  | 'sessionInputRequired'
  | 'sessionWaitingForYou'
  | 'sessionStranded'
  | 'sessionLoading'
  | 'sessionPausedClosed'
  | 'sessionPausedArchived'
  | 'sessionShowAll'
  | 'sessionHideAll'

/**
 * The fixed captions of the app chrome: the sidebar's own controls and the
 * theme picker. Same reasoning as `ChannelTextKey`: each could be a prop, but
 * an app would then pass the same translated word into every instance.
 */
export type ChromeTextKey =
  | 'sidebar'
  | 'mainNavigation'
  | 'collapseSidebar'
  | 'closeNavigation'
  | 'filterNavigation'
  | 'filter'
  | 'otherGroup'
  | 'colourTheme'
  | 'themeSystem'
  | 'themeLight'
  | 'themeDark'

const CHROME_TEXT: Record<ChromeTextKey, string> = {
  sidebar: 'Sidebar',
  mainNavigation: 'Main',
  collapseSidebar: 'Collapse sidebar',
  closeNavigation: 'Close navigation',
  filterNavigation: 'Filter navigation',
  filter: 'Filter',
  otherGroup: 'Other',
  colourTheme: 'Colour theme',
  themeSystem: 'System',
  themeLight: 'Light',
  themeDark: 'Dark',
}

const CHANNEL_TEXT: Record<ChannelTextKey, string> = {
  newMessage: 'New message',
  editMessage: 'Edit message',
  messageType: 'Message type',
  cancelReply: 'Cancel reply',
  required: 'Required.',
  toSend: 'to send',
  toSave: 'to save',
  send: 'Send',
  sending: 'Sending…',
  save: 'Save',
  saving: 'Saving…',
  cancel: 'Cancel',
  typeGone: 'This message type no longer exists, so the message cannot be edited.',
  choose: 'Choose…',
  choosing: 'Choosing…',
  change: 'Change…',
  replace: 'Replace…',
  clear: 'Clear',
  closed: 'Closed',
  archived: 'Archived',
  working: 'Working…',
  messages: 'Messages',
  loadOlder: 'Load older messages',
  loading: 'Loading…',
  noMessages: 'No messages yet.',
  jumpToLatest: 'Jump to latest',
  closedNotice: 'This channel is closed. It can be read but takes no new messages.',
  archivedNotice: 'This channel is archived. It can be read but takes no new messages.',
  yes: 'Yes',
  no: 'No',
  edited: '(edited)',
  reply: 'Reply',
  edit: 'Edit',
  delete: 'Delete',
  deleting: 'Deleting…',
  remove: 'Remove…',
  removing: 'Removing…',
  release: 'Release',
  releasing: 'Releasing…',
  showChannels: 'Show channels',
  filterOpen: 'Open',
  filterAll: 'All',
  newChannel: 'New channel',
  noChannels: 'No channels yet.',
  system: 'system',
  agent: 'agent',
  restrictedRef: 'Restricted reference',
  restrictedRefTitle: 'You do not have access to this entity',
  session: 'Session',
  sessionEnded: 'Ended',
  sessionCompleted: 'Completed',
  sessionCancelled: 'Cancelled',
  sessionFailed: 'Failed',
  sessionInputRequired: 'Input required',
  sessionWaitingForYou: 'Waiting for you',
  sessionStranded: 'Stranded',
  sessionLoading: 'Loading the states of this session…',
  sessionPausedClosed: 'Paused while the channel is closed',
  sessionPausedArchived: 'Paused while the channel is archived',
  sessionShowAll: 'Show all states',
  sessionHideAll: 'Hide the states',
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
  /** A column heading's collapse control: `Collapse Done`. */
  collapseSection: (args: { title: string }) => string
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

  /**
   * The placeholder of a removed channel message. `by` and `reason` are set
   * only for a user the server shows them to.
   */
  channelMessageRemoved: (args: {
    kind: 'author' | 'moderator'
    by?: string
    reason?: string
  }) => string
  /** The quote above a reply: `Reply to Alice`. */
  channelReplyTo: (args: { author?: string }) => string
  /** The composer's note while a reply is being written: `Replying to Alice`. */
  channelReplyingTo: (args: { author?: string }) => string
  /** A message whose action waits to complete: `Promote: pending`. */
  channelActionPending: (args: { label: string }) => string
  /** A message whose action ran: `Promote: done`. */
  channelActionDone: (args: { label: string }) => string
  /** A composer field over its `max`: `At most 140 characters.` */
  channelTooLong: (args: { max: number }) => string
  /**
   * Announced when messages arrive at the end of the log. `type` and `author`
   * are set when exactly one arrived. Session notices are not counted; see
   * `channelNewNotices`.
   */
  channelNewMessages: (args: { count: number; type?: string; author?: string }) => string
  /**
   * Announced when session notices arrive at the end of the log. `notices`
   * holds the text of each, from `channelSessionMoved` or
   * `channelSessionReentered`. `messages` is the `channelNewMessages` text
   * for the other messages that arrived with them, if any.
   */
  channelNewNotices: (args: { notices: string[]; messages?: string }) => string
  /** A session notice: `Session moved from Detected to Contained`. */
  channelSessionMoved: (args: { from: string; to: string }) => string
  /** A session notice for a move into the same state: `Session entered Contained again`. */
  channelSessionReentered: (args: { state: string }) => string
  /** Whom a session waits for: `Waiting for the incident lead`. */
  channelSessionWaiting: (args: { labels: string[] }) => string
  /** Why a stranded session does not move, naming its stored state. */
  channelSessionStrandedText: (args: { state: string }) => string
  /** The composer's note while a message type that moves the session is selected. */
  channelSessionHint: (args: { to: string; guarded?: boolean }) => string
  /**
   * Screen-reader text read after the label of a composer message type
   * that moves the session: ` (moves the session)`.
   */
  channelSessionOptionHint: () => string
  /**
   * The one-line summary of a narrow session stepper. `label` is the
   * current state's label, or its stored name when stranded or loading.
   */
  channelSessionSummary: (args: {
    status: 'normal' | 'final' | 'paused' | 'stranded' | 'loading'
    label: string
    /** For a final state: `Completed`, `Cancelled`, `Failed` or `Ended`. */
    outcome?: string
  }) => string
  /**
   * The screen-reader text of a stepper state that is a next state, an
   * end state, or both: `Contained, next`.
   */
  channelSessionStep: (args: { label: string; next: boolean; final: boolean }) => string
  /**
   * Who else is typing in a channel: `Ann is typing…`, `Ann and Bo are
   * typing…`, `Ann, Bo and 1 other are typing…`. `names` holds at most two
   * names; `others` counts the typists beyond them.
   */
  channelTyping: (args: { names: string[]; others: number }) => string
  /** The channel components' fixed captions; see `ChannelTextKey`. */
  channelText: (args: { key: ChannelTextKey }) => string
  /** The sidebar's and theme picker's fixed captions; see `ChromeTextKey`. */
  chromeText: (args: { key: ChromeTextKey }) => string
  /** The name of a channel row's unread badge: `3 unread`. */
  channelUnreadCount: (args: { count: number }) => string
}

/** `a`, `a or b`, `a, b or c`. */
function orList(items: string[]): string {
  if (items.length <= 1) return items.join('')
  return `${items.slice(0, -1).join(', ')} or ${items[items.length - 1]}`
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
  collapseSection: ({ title }) => `Collapse ${title}`,
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

  channelMessageRemoved: ({ kind, by, reason }) =>
    (kind === 'author' ? 'Deleted by its author' : 'Removed by a moderator') +
    (by ? ` (${by})` : '') +
    (reason ? `: ${reason}` : ''),
  channelReplyTo: ({ author }) => (author ? `Reply to ${author}` : 'Reply to an earlier message'),
  channelReplyingTo: ({ author }) =>
    author ? `Replying to ${author}` : 'Replying to an earlier message',
  channelActionPending: ({ label }) => `${label}: pending`,
  channelActionDone: ({ label }) => `${label}: done`,
  channelTooLong: ({ max }) => `At most ${max} characters.`,
  channelNewMessages: ({ count, type, author }) =>
    count === 1 && type ? `${type}${author ? ` from ${author}` : ''}` : `${count} new messages`,
  channelNewNotices: ({ notices, messages }) => `${[...(messages ? [messages] : []), ...notices].join('. ')}.`,
  channelSessionMoved: ({ from, to }) => `Session moved from ${from} to ${to}`,
  channelSessionReentered: ({ state }) => `Session entered ${state} again`,
  channelSessionWaiting: ({ labels }) => `Waiting for ${orList(labels)}`,
  channelSessionStrandedText: ({ state }) =>
    `${state} is no longer a state of this session. Messages do not move it.`,
  channelSessionHint: ({ to, guarded }) =>
    `Sending this moves the session to ${to}${guarded ? ' if its condition holds' : ''}.`,
  channelSessionOptionHint: () => ' (moves the session)',
  channelSessionSummary: ({ status, label, outcome }) => {
    switch (status) {
      case 'stranded':
        return `Stranded · ${label}`
      case 'final':
        return `${outcome ?? 'Ended'} · ${label}`
      case 'paused':
        return `${label} · Paused`
      case 'loading':
        return `Session · ${label} · Loading`
      default:
        return `Session · ${label}`
    }
  },
  channelSessionStep: ({ label, next, final }) =>
    [label, ...(next ? ['next'] : []), ...(final ? ['end state'] : [])].join(', '),
  channelTyping: ({ names, others }) => {
    if (others > 0) {
      return `${names.join(', ')} and ${others} ${others === 1 ? 'other' : 'others'} are typing…`
    }
    if (names.length === 1) return `${names[0]} is typing…`
    return `${names.join(' and ')} are typing…`
  },
  channelText: ({ key }) => CHANNEL_TEXT[key],
  chromeText: ({ key }) => CHROME_TEXT[key],
  channelUnreadCount: ({ count }) => `${count} unread`,
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
