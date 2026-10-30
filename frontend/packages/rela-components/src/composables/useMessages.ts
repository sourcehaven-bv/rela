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
  // Quiet message types
  | 'showAllMessages'
  | 'onlyHidden'
  | 'onlyHiddenOlder'
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
  | 'sessionWorking'
  | 'sessionStranded'
  | 'sessionLoading'
  | 'sessionPausedClosed'
  | 'sessionPausedArchived'
  | 'sessionShowAll'
  | 'sessionHideAll'
  // Session decisions
  | 'decisions'
  // Reply buttons of a question's options
  | 'choices'

/**
 * The fixed captions of every component outside the channel set: control
 * names, default placeholders and labels, and screen-reader text. Same
 * reasoning as `ChannelTextKey`: each could be a prop, but an app would then
 * pass the same translated word into every instance.
 *
 * Where a component also takes the caption as a prop, the prop wins; the text
 * here is only its default.
 */
export type ChromeTextKey =
  // Sidebar and theme picker
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
  | 'emptyValue'
  | 'helpDocs'
  // Shared verbs and states
  | 'add'
  | 'cancel'
  | 'save'
  | 'edit'
  | 'delete'
  | 'back'
  | 'expand'
  | 'previous'
  | 'next'
  | 'help'
  | 'more'
  | 'loading'
  | 'working'
  | 'saving'
  | 'saved'
  | 'couldNotSave'
  | 'search'
  | 'clearSearch'
  | 'clearSelection'
  | 'select'
  | 'selectOptions'
  | 'from'
  | 'to'
  | 'name'
  | 'unassigned'
  | 'assignedTo'
  | 'noMatchingPeople'
  | 'due'
  | 'today'
  | 'keyThen'
  | 'keyOr'
  // Page and panel chrome
  | 'openNavigation'
  | 'moreOptions'
  | 'star'
  | 'like'
  | 'share'
  | 'copyLink'
  | 'attachFile'
  | 'closePanel'
  | 'closeDialog'
  | 'resizeSidebar'
  | 'resizePanel'
  | 'view'
  | 'documentActions'
  | 'taskActions'
  | 'pagination'
  | 'previousPage'
  | 'nextPage'
  | 'notifications'
  | 'dismissMessage'
  | 'dismissNotification'
  | 'commandPalette'
  | 'commandPlaceholder'
  | 'noMatchingCommands'
  // Boards and tables
  | 'addSection'
  | 'draggableCard'
  | 'moveRowHint'
  | 'subtasks'
  // Calendar
  | 'continuesAfter'
  | 'continuesBeforeAndAfter'
  | 'continuedFromBefore'
  // Comments
  | 'commentNoun'
  | 'comments'
  | 'commentAction'
  | 'addComment'
  | 'addCommentLabel'
  | 'addCommentPlaceholder'
  | 'replyPlaceholder'
  | 'reply'
  | 'commentBody'
  | 'commentAbout'
  | 'editComment'
  | 'postingComment'
  | 'toPost'
  | 'checking'
  | 'allComments'
  | 'noComments'
  | 'startThread'
  | 'closeComments'
  | 'detached'
  | 'detachedComment'
  | 'resolve'
  | 'reopen'
  // Task detail and attachments
  | 'description'
  | 'attachments'
  | 'subtasksHeading'
  | 'showEmptyFields'
  | 'download'
  | 'fileKindSheet'
  | 'fileKindImage'
  | 'fileKindOther'
  // Editors and code
  | 'formatting'
  | 'table'
  | 'writeSomething'
  | 'docTitlePlaceholder'
  | 'docBodyPlaceholder'
  | 'code'
  | 'copyCode'
  | 'copied'

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
  emptyValue: 'Empty',
  helpDocs: 'Help docs',
  add: 'Add',
  cancel: 'Cancel',
  save: 'Save',
  edit: 'Edit',
  delete: 'Delete',
  back: 'Back',
  expand: 'Expand',
  previous: 'Previous',
  next: 'Next',
  help: 'Help',
  more: 'more',
  loading: 'Loading',
  working: 'Working',
  saving: 'Saving',
  saved: 'Saved',
  couldNotSave: 'Could not save',
  search: 'Search',
  clearSearch: 'Clear search',
  clearSelection: 'Clear selection',
  select: 'Select',
  selectOptions: 'Select options',
  from: 'From',
  to: 'To',
  name: 'Name',
  unassigned: 'Unassigned',
  assignedTo: 'Assigned to',
  noMatchingPeople: 'No matching people',
  due: 'Due',
  today: 'Today',
  keyThen: 'then',
  keyOr: 'or',
  openNavigation: 'Open navigation',
  moreOptions: 'More options',
  star: 'Star',
  like: 'Like',
  share: 'Share',
  copyLink: 'Copy link',
  attachFile: 'Attach file',
  closePanel: 'Close panel',
  closeDialog: 'Close dialog',
  resizeSidebar: 'Resize sidebar',
  resizePanel: 'Resize panel',
  view: 'View',
  documentActions: 'Document actions',
  taskActions: 'Task actions',
  pagination: 'Pagination',
  previousPage: 'Previous page',
  nextPage: 'Next page',
  notifications: 'Notifications',
  dismissMessage: 'Dismiss message',
  dismissNotification: 'Dismiss notification',
  commandPalette: 'Command palette',
  commandPlaceholder: 'Type a command or search',
  noMatchingCommands: 'No matching commands',
  addSection: 'Add section',
  draggableCard: 'Draggable card',
  moveRowHint: 'Use the up and down arrow keys to move this row',
  subtasks: 'subtasks',
  continuesAfter: 'Continues on following days',
  continuesBeforeAndAfter: 'Continues before and after this day',
  continuedFromBefore: 'Continued from earlier days',
  commentNoun: 'comment',
  comments: 'comments',
  commentAction: 'Comment',
  addComment: 'Add comment',
  addCommentLabel: 'Add a comment',
  addCommentPlaceholder: 'Add a comment...',
  replyPlaceholder: 'Reply...',
  reply: 'Reply',
  commentBody: 'Comment body',
  commentAbout: 'What this comment is about',
  editComment: 'Edit comment',
  postingComment: 'Posting comment',
  toPost: 'to post',
  checking: 'Checking...',
  allComments: 'All comments',
  noComments: 'No comments yet',
  startThread: 'Start the thread below.',
  closeComments: 'Close comments',
  detached: 'detached',
  detachedComment: 'What this comment points at no longer exists',
  resolve: 'Resolve',
  reopen: 'Reopen',
  description: 'Description',
  attachments: 'Attachements',
  subtasksHeading: 'Subtasks',
  showEmptyFields: 'Show empty fields',
  download: 'Download',
  fileKindSheet: 'SHEET',
  fileKindImage: 'IMAGE',
  fileKindOther: 'FILE',
  formatting: 'Formatting',
  table: 'Table',
  writeSomething: 'Write something...',
  docTitlePlaceholder: 'Give your doc a title...',
  docBodyPlaceholder: 'Click here to start writing',
  code: 'Code',
  copyCode: 'Copy code',
  copied: 'Copied',
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
  showAllMessages: 'Show all messages',
  onlyHidden: 'Only hidden messages here. Use Show all messages to see them.',
  onlyHiddenOlder: 'Only hidden messages so far. Load older messages or use Show all messages to see them.',
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
  sessionWorking: 'Working…',
  sessionStranded: 'Stranded',
  sessionLoading: 'Loading the states of this session…',
  sessionPausedClosed: 'Paused while the channel is closed',
  sessionPausedArchived: 'Paused while the channel is archived',
  sessionShowAll: 'Show all states',
  sessionHideAll: 'Hide the states',
  decisions: 'Decisions',
  choices: 'Choices',
}

/**
 * How long a busy entry has been in its state, rounded down to the largest
 * whole unit: under a minute, minutes, hours or days. A translation words it.
 */
export interface BusyDuration {
  unit: 'lessThanMinute' | 'minutes' | 'hours' | 'days'
  count: number
}

/** The `BusyDuration` of `ms` milliseconds. A negative span reads as under a minute. */
export function busyDuration(ms: number): BusyDuration {
  const minutes = Math.floor(ms / 60_000)
  if (minutes < 1) return { unit: 'lessThanMinute', count: 0 }
  if (minutes < 60) return { unit: 'minutes', count: minutes }
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return { unit: 'hours', count: hours }
  return { unit: 'days', count: Math.floor(hours / 24) }
}

/** One entry of the busy indicator's accessible name. */
export interface BusyNameEntry {
  /** What is busy, such as a channel type label: `Agent run`. */
  type?: string
  /** The state it is busy in: `Planning`. */
  label: string
  duration?: BusyDuration
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
  /** Announced when a move is refused; `reason` is the caller's, already localised. */
  sortRefused: (args: { title: string; position: number; count: number; reason: string }) => string

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
  /**
   * The fold line of a run of collapsed messages: `3 messages: Claim,
   * Progress`. `labels` are the type labels in first-seen order.
   */
  channelFold: (args: { count: number; labels: string[] }) => string
  /**
   * Announced when loading older messages added only messages that are
   * not shown: `4 older messages are hidden. Use Show all messages to see
   * them.`
   */
  channelOlderHidden: (args: { count: number }) => string
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
   * The composer's note while a message type that may move the session to
   * one of several states is selected: `Sending this may move the session
   * to Working or Planning.`
   */
  channelSessionBranchHint: (args: { tos: string[] }) => string
  /**
   * Screen-reader text read after the label of a composer message type
   * that moves the session: ` (moves the session)`.
   */
  channelSessionOptionHint: () => string
  /**
   * Screen-reader text read after the label of a session decision button:
   * ` (moves the session to Working)`, ` (moves the session)` when the
   * target is not one state, or ` (opens the form)` for a button that opens
   * the composer instead of posting.
   */
  channelDecisionHint: (args: { to?: string; form?: boolean }) => string
  /**
   * Screen-reader text read after the label of a choice button, which
   * posts that option as a reply: ` (posts this answer as a reply)`.
   */
  channelChoiceHint: () => string
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
  /** Every other component's fixed captions; see `ChromeTextKey`. */
  chromeText: (args: { key: ChromeTextKey }) => string
  /** The name of a channel row's unread badge: `3 unread`. */
  channelUnreadCount: (args: { count: number }) => string

  /** A button's pending label, built from its caption: `Delete in progress`. */
  inProgress: (args: { label: string }) => string
  /** A sidebar group's menu control: `Projects options`. */
  groupOptions: (args: { label: string }) => string
  /** A calendar day's overflow control: `+3 more`. */
  moreCount: (args: { count: number }) => string
  /** A file field's drop zone. */
  dropFiles: (args: { multiple: boolean }) => string

  /** A table row's checkbox: `Select Fix login`. */
  selectItem: (args: { title: string }) => string
  /** A table section's select-all checkbox: `Select all in Done`. */
  selectAllIn: (args: { section: string }) => string
  /** A table row's drag handle: `Move Fix login`. */
  moveRow: (args: { title: string }) => string
  /** A table section's show-more button: `Show 20 more`. */
  showMoreRows: (args: { count: number }) => string
  /** Follows `showMoreRows` in a quieter style: `of 45 remaining`. */
  ofRemaining: (args: { remaining: number }) => string

  /** The comments panel's unresolved tally: `2 open`. */
  openCount: (args: { count: number }) => string
  /** The comments panel's tally of comments with no field: `1 not on a field`. */
  notOnField: (args: { count: number }) => string
  /** The heading of a field's comment popover: `Comments on Title`. */
  commentsOn: (args: { anchor: string }) => string
  /** The heading of a block's comment popover: `On this paragraph`. */
  onThisBlock: (args: { kind: string }) => string

  /**
   * An editor toolbar button's name. `label` is the command's own English
   * label, which the English set returns unchanged; a translation looks the
   * command up by `id` and falls back to `label` for a command it does not
   * know, such as one an app added.
   */
  editorCommand: (args: { id: string; label: string }) => string
  /** The tooltip of a toolbar button that cannot run where the cursor is. */
  commandUnavailable: (args: { label: string }) => string

  /**
   * The busy indicator's visible text after the first entry's state label:
   * the time in the state and how many more entries there are, ` · 3h +1`.
   * It starts with a space, or is empty, because it follows the label
   * directly. The label is rendered apart, so a long one can be cut off
   * while this stays visible.
   */
  busyTail: (args: { duration?: BusyDuration; more: number }) => string
  /**
   * The busy indicator's accessible name, naming every entry:
   * `Agent run: Planning, for 3 hours`.
   */
  busyName: (args: { entries: BusyNameEntry[] }) => string

  /** The inbox section of sessions that wait on the user. */
  inboxWaitingHeading: () => string
  /** The inbox section of unread mentions and replies. */
  inboxMentionsHeading: () => string
  /** The inbox section of channels with unread messages. */
  inboxUnreadHeading: () => string
  /** The empty state of the waiting section. */
  inboxWaitingEmpty: () => string
  /** The empty state of the mentions section. */
  inboxMentionsEmpty: () => string
  /** The empty state of the unread section. */
  inboxUnreadEmpty: () => string
  /** A mention row: `Bob mentioned you`. */
  inboxMentioned: (args: { author: string }) => string
  /** A reply row: `Bob replied to you`. */
  inboxReplied: (args: { author: string }) => string
  /** The button that marks a row read. */
  inboxMarkRead: () => string
  /** The note under a cut-off waiting list. `total` is preformatted, such as `20000+`. */
  inboxShowing: (args: { shown: number; total: string }) => string
  /** The notice shown when the server could not load every channel setting. */
  inboxDegraded: () => string
}

/** `a`, `a or b`, `a, b or c`. */
function orList(items: string[]): string {
  if (items.length <= 1) return items.join('')
  return `${items.slice(0, -1).join(', ')} or ${items[items.length - 1]}`
}

/** `3h`, the short form of a busy duration. */
function shortDuration(d: BusyDuration, hourUnit = 'h'): string {
  switch (d.unit) {
    case 'lessThanMinute':
      return '<1m'
    case 'minutes':
      return `${d.count}m`
    case 'hours':
      return `${d.count}${hourUnit}`
    default:
      return `${d.count}d`
  }
}

/** `for 3 hours`. */
function longDuration(d: BusyDuration): string {
  const n = (one: string, many: string) => `for ${d.count} ${d.count === 1 ? one : many}`
  switch (d.unit) {
    case 'lessThanMinute':
      return 'for less than a minute'
    case 'minutes':
      return n('minute', 'minutes')
    case 'hours':
      return n('hour', 'hours')
    default:
      return n('day', 'days')
  }
}

/**
 * ` · 3h +1`, the text after a busy label. Shared with the Dutch set,
 * which passes its own hour unit.
 */
export function busyTailWith(
  { duration, more }: { duration?: BusyDuration; more: number },
  hourUnit = 'h',
): string {
  return `${duration ? ` · ${shortDuration(duration, hourUnit)}` : ''}${more > 0 ? ` +${more}` : ''}`
}

/** `Agent run: Planning, for 3 hours`, with `long` wording the duration. */
export function busyNameWith(entries: BusyNameEntry[], long: (d: BusyDuration) => string): string {
  return entries
    .map((e) => `${e.type ? `${e.type}: ` : ''}${e.label}${e.duration ? `, ${long(e.duration)}` : ''}`)
    .join('; ')
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
  sortRefused: ({ title, position, count, reason }) =>
    `${title} cannot move there. ${reason} Put back at ${position} of ${count}.`,

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
  channelFold: ({ count, labels }) => `${count} ${count === 1 ? 'message' : 'messages'}: ${labels.join(', ')}`,
  channelOlderHidden: ({ count }) =>
    `${count} older ${count === 1 ? 'message is' : 'messages are'} hidden. Use Show all messages to see them.`,
  channelSessionMoved: ({ from, to }) => `Session moved from ${from} to ${to}`,
  channelSessionReentered: ({ state }) => `Session entered ${state} again`,
  channelSessionWaiting: ({ labels }) => `Waiting for ${orList(labels)}`,
  channelSessionStrandedText: ({ state }) =>
    `${state} is no longer a state of this session. Messages do not move it.`,
  channelSessionHint: ({ to, guarded }) =>
    `Sending this moves the session to ${to}${guarded ? ' if its condition holds' : ''}.`,
  channelSessionBranchHint: ({ tos }) => `Sending this may move the session to ${orList(tos)}.`,
  channelSessionOptionHint: () => ' (moves the session)',
  channelDecisionHint: ({ to, form }) =>
    form ? ' (opens the form)' : to ? ` (moves the session to ${to})` : ' (moves the session)',
  channelChoiceHint: () => ' (posts this answer as a reply)',
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

  inProgress: ({ label }) => `${label} in progress`,
  groupOptions: ({ label }) => `${label} options`,
  moreCount: ({ count }) => `+${count} more`,
  dropFiles: ({ multiple }) => `Drop ${multiple ? 'files' : 'a file'} here, or click to choose`,

  selectItem: ({ title }) => `Select ${title}`,
  selectAllIn: ({ section }) => `Select all in ${section}`,
  moveRow: ({ title }) => `Move ${title}`,
  showMoreRows: ({ count }) => `Show ${count} more`,
  ofRemaining: ({ remaining }) => `of ${remaining} remaining`,

  openCount: ({ count }) => `${count} open`,
  notOnField: ({ count }) => `${count} not on a field`,
  commentsOn: ({ anchor }) => `Comments on ${anchor}`,
  onThisBlock: ({ kind }) => `On this ${kind}`,

  editorCommand: ({ label }) => label,
  commandUnavailable: ({ label }) => `${label} (not available here)`,

  busyTail: (args) => busyTailWith(args),
  busyName: ({ entries }) => busyNameWith(entries, longDuration),

  inboxWaitingHeading: () => 'Waiting on you',
  inboxMentionsHeading: () => 'Mentions and replies',
  inboxUnreadHeading: () => 'Unread channels',
  inboxWaitingEmpty: () => 'Nothing waits on you.',
  inboxMentionsEmpty: () => 'No unread mentions or replies.',
  inboxUnreadEmpty: () => 'All channels are read.',
  inboxMentioned: ({ author }) => `${author} mentioned you`,
  inboxReplied: ({ author }) => `${author} replied to you`,
  inboxMarkRead: () => 'Mark read',
  inboxShowing: ({ shown, total }) => `Showing ${shown} of ${total}`,
  inboxDegraded: () =>
    'Some channel settings could not be loaded. The list may be incomplete.',
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
