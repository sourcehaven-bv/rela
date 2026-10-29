/**
 * The shapes the channel components share.
 *
 * A channel is a log of typed messages hanging off an entity. These are the
 * presentational shapes only: the app reads the channel API, decides what the
 * user may do, formats times and renders markdown, and hands the results in.
 * The library does no fetching, no date handling and no markdown rendering.
 *
 * Several shapes match the app's wire types field for field (the property,
 * message type and ref shapes), so the app can pass those straight through.
 * The rest (message, author, ephemeral line, list row) are views the app
 * builds from the wire.
 */
import type { Component } from 'vue'
import type { TagColor } from '../../types'

/**
 * A message property's type, as realtime.yaml declares it. Named apart from
 * the app's wire type of the same shape, so a consumer can import both.
 */
export type ChannelPropertyKind =
  | 'string'
  | 'markdown'
  | 'integer'
  | 'boolean'
  | 'date'
  | 'datetime'
  | 'ref'
  | 'enum'

/** One property of a message type. */
export interface ChannelPropertyDef {
  name: string
  /** Shown as it is; the server sends the name when the config sets no label. */
  label: string
  type: ChannelPropertyKind
  required?: boolean
  /**
   * The most Unicode code points a string or markdown value may hold; for a
   * list, the most each item may hold.
   */
  max?: number
  /** A list of strings: the value is an array, shown one item per line. */
  list?: boolean
  /** The enum type whose labels and colours `enumOptions` carries. */
  enum?: string
  /** The allowed values of an enum property. */
  values?: string[]
  /** The entity types a ref property may point to. */
  to?: string[]
}

/** An action a message of some type offers, such as promoting it to an entity. */
export interface ChannelActionDef {
  name: string
  label: string
  /** The entity type the action creates, when it promotes the message. */
  promote_to?: string
}

/** One message type of a channel type. */
export interface ChannelMessageTypeDef {
  name: string
  label: string
  /** `ephemeral` messages are signals that expire; they never enter the log. */
  persist: 'log' | 'ephemeral'
  /** Whether a message of this type may reply to another. */
  reply: boolean
  properties: ChannelPropertyDef[]
  actions: ChannelActionDef[]
  /**
   * The options of a question: `from` names its list property, and a click
   * posts `answer` with the option in `into`. The app builds the buttons.
   */
  choices?: { from: string; answer: string; into: string; other: boolean }
}

/** A lifecycle operation of a channel type, such as close or archive. */
export interface ChannelLifecycleDef {
  name: string
  label: string
  /** Draws the button in the danger tone, for an operation such as delete. */
  danger?: boolean
}

/**
 * The widgets a channel field can use. `ref` is not configurable: a ref
 * property always gets the picker button.
 */
export type ChannelWidget =
  | 'text'
  | 'textarea'
  | 'number'
  | 'checkbox'
  | 'date'
  | 'datetime'
  | 'select'
  | 'ref'

/**
 * How one property is presented: one entry of a message type's `fields:` in
 * data-entry.yaml. Every key but `property` is optional.
 */
export interface ChannelFieldPresentation {
  property: string
  label?: string
  placeholder?: string
  help?: string
  widget?: string
  /** Columns on a 12-column grid; absent or 0 is the full width. */
  span?: number
  hidden?: boolean
}

/** The presentation of one message type. */
export interface ChannelMessagePresentation {
  fields?: ChannelFieldPresentation[]
}

/** The presentation of one channel type: data-entry.yaml `channels.<type>`. */
export interface ChannelPresentation {
  /** Keyed by message type. */
  messages?: Record<string, ChannelMessagePresentation | undefined>
}

/** One enum value with the label and colour the app's schema gives it. */
export interface ChannelEnumOption {
  value: string
  label: string
  color?: TagColor
}

/** A ref the user may read. */
export interface ChannelReadableRef {
  id: string
  type: string
  title?: string
  restricted?: false
}

/**
 * A ref the user may not read. It has no `id`, so a component cannot show
 * what the server withheld.
 */
export interface ChannelRestrictedRef {
  restricted: true
}

/** A ref property value as the user sees it. */
export type ChannelRef = ChannelReadableRef | ChannelRestrictedRef

/** A stored property value: a scalar, a ref, or the items of a list property. */
export type ChannelPropValue = string | number | boolean | ChannelRef | string[]

/**
 * A composer field's raw value: input text, a checkbox state, or a ref's
 * entity id. The app converts these to request values.
 */
export type ChannelFieldValue = string | boolean | undefined

/**
 * Who wrote a message. `system` is a message the server posted itself, such
 * as a trigger's; `agent` is one posted through an agent tool.
 */
export interface ChannelAuthorView {
  name: string
  kind?: 'person' | 'agent' | 'system'
}

/** The message a reply points at. */
export interface ChannelReplyQuote {
  seq: number
  /** Absent when the app does not hold the original. */
  author?: string
  /** A short plain-text excerpt of the original. */
  excerpt?: string
}

/** Why a message is gone. */
export interface ChannelRemovalView {
  /** `author` deleted it; a `moderator` removed it. */
  kind: 'author' | 'moderator'
  /** Who removed it. The server sends this only to those who may see it. */
  by?: string
  reason?: string
  /** Preformatted by the app. */
  at?: string
}

/** The state of the action run on a message. */
export interface ChannelMessageActionView {
  name: string
  status: 'pending' | 'done'
  /** The entity the action created, for a promote. */
  promoted?: ChannelRef
}

/** One message in the log. */
export interface ChannelMessageView {
  seq: number
  /** Changes on every edit, removal or action, so a row re-renders. */
  rev: number
  /** Absent on a removed message. */
  type?: string
  author?: ChannelAuthorView
  /** Preformatted by the app. */
  at?: string
  /** The machine-readable time, for the `<time>` element. */
  datetime?: string
  props?: Record<string, ChannelPropValue>
  /**
   * Sanitized HTML for the markdown properties, keyed by property name.
   *
   * The app renders and sanitizes it; the library inserts it as it is and
   * never renders or cleans markdown itself. A markdown property without an
   * entry here is shown as plain text.
   */
  html?: Record<string, string>
  /**
   * Preformatted text for properties the app formats itself, such as a
   * datetime in the user's zone, keyed by property name. Used instead of
   * the raw value.
   */
  text?: Record<string, string>
  replyTo?: ChannelReplyQuote
  /** Set once the message was edited; preformatted, shown as a tooltip. */
  editedAt?: string
  removed?: ChannelRemovalView
  action?: ChannelMessageActionView
  /**
   * What the user may do with this message now: the operations `edit`,
   * `delete`, `remove` and `release`, and the names of the message type's
   * actions. A control for anything not listed is not shown.
   */
  actions?: string[]
  /**
   * Set on a session notice: the system message the server appends when a
   * message moves the channel's session. The row then shows one line and no
   * controls instead of a card.
   */
  notice?: ChannelNoticeView
}

/** What a session notice says, with the state labels the app looked up. */
export interface ChannelNoticeView {
  fromLabel: string
  toLabel: string
  /** The session entered the state it was already in. */
  reentry: boolean
}

/**
 * A session state's status: `working`, `input_required` (it waits for a
 * named role), or the outcome of a final state.
 */
export type ChannelSessionStatus = 'working' | 'input_required' | 'completed' | 'cancelled' | 'failed'

/** One state of a channel's session. */
export interface ChannelSessionStateView {
  name: string
  label: string
  /** A state the session ends in. */
  final?: boolean
}

/**
 * A channel's session, for the stepper: the states in declaration order and
 * where the session is now. The app builds it from the channel type's
 * descriptor and the channel's head.
 */
export interface ChannelSessionView {
  /** The session's label. */
  label: string
  /** Declaration order; the stepper adds no other order. */
  states: ChannelSessionStateView[]
  /** The current state's name; absent when stranded or loading. */
  current?: string
  /** The current state's label, or the stored name when stranded or loading. */
  currentLabel: string
  /** The states a transition out of the current state leads to. */
  next: string[]
  /** The current state is one the session ends in. */
  final?: boolean
  /** The stored state is one the configuration no longer declares. */
  stranded?: boolean
  /**
   * The states shown do not include the current one yet: the app holds an
   * older description of the session and is loading the new one.
   */
  loading?: boolean
  /** The channel is not open, so the session does not move. */
  paused?: 'closed' | 'archived'
  /** The current state's status; absent reads as working, or completed when final. */
  status?: ChannelSessionStatus
  /** Whom an input_required state waits for, by label. */
  waitingFor?: { label: string }[]
  /** The session waits for the reader. */
  waitingOnMe?: boolean
}

/**
 * A message type whose post would move the session, as the composer marks
 * it. The app builds the list from the caller's affordances and the head.
 */
export interface ChannelComposerTransition {
  messageType: string
  /**
   * The labels of the states the session may move to, without repeats, in
   * the order the server tries the type's transitions. Usually one.
   */
  toLabels: string[]
  /** Every transition has a condition, so the move depends on the message. */
  guarded?: boolean
  /**
   * Some transitions have a condition and some do not, so whether a post
   * moves the session depends on the poster or the message. The composer
   * then says the post may move it, as for several targets.
   */
  mayMove?: boolean
}

/**
 * A button that posts a message to move the session, or opens its form.
 * The app builds the list from the caller's affordances, the head and the
 * channel type's descriptor; the library only renders it and emits `key`.
 */
export interface ChannelDecisionView {
  /** Unique in the list; what `decide` emits and the pending key uses. */
  key: string
  messageType: string
  /** Props the post carries, for a button that stands for one choice. */
  props?: Record<string, ChannelPropValue>
  label: string
  /** The target state's label; absent when the edges lead to several. */
  toLabel?: string
  /** The button opens the composer instead of posting. */
  opensForm?: boolean
  /**
   * Ask before posting: the move ends the session. Only on a one-click
   * decision, which always has `toLabel`.
   */
  confirm?: boolean
  /** Draws the button in the danger tone: the session ends cancelled or failed. */
  danger?: boolean
}

/** An ephemeral signal, shown as one compact line while it lasts. */
export interface ChannelEphemeralView {
  id: string
  type: string
  author: ChannelAuthorView
  props?: Record<string, ChannelPropValue>
  /**
   * The line to show after the type's label, preformatted by the app (times
   * in the user's zone, enum labels). Without it the props are shown in
   * presentation order with enum labels, and readable refs by title.
   */
  text?: string
}

/**
 * Someone else typing in the channel, from the head's `typing_now`. The app
 * passes exactly what the last head says; the server expires an entry and
 * sends a new head then, so the library runs no timer.
 */
export interface ChannelTypingView {
  /** Unique within the list, such as the author's user. */
  key: string
  author: ChannelAuthorView
}

/**
 * The state of a channel. Named apart from the app's wire type of the same
 * shape, so a consumer can import both.
 */
export type ChannelStateView = 'open' | 'closed' | 'archived'

/** One channel in an entity's channel list. */
export interface ChannelRowView {
  /** Unique within the list; the app's key for selecting it. */
  key: string
  title: string
  state: ChannelStateView
  /** A preformatted line under the title, such as the last activity. */
  summary?: string
  /**
   * How many messages the user has not read. The server stops counting at
   * 100; above 99 the badge shows `99+`. Absent or 0 shows no badge.
   */
  unread?: number
}

/** The channels of one channel type on an entity. */
export interface ChannelGroupView {
  type: string
  label: string
  cardinality: 'one' | 'many'
  /** Whether the user may create a channel of this type now. */
  canCreate: boolean
  /** The create button's label; defaults to `New channel`. */
  createLabel?: string
  channels: ChannelRowView[]
}

/** A new message, as the composer emits it. Values are raw; the app converts them. */
export interface ChannelPostEvent {
  type: string
  /** One value per rendered field, keyed by property name. */
  values: Record<string, ChannelFieldValue>
  /**
   * The properties the composer rendered, for an edit that carries the rest.
   * A restricted ref the user did not replace is not among them, so the app
   * sends it as unrendered and the server keeps it.
   */
  rendered: string[]
  replyTo?: number
}

/** An edit, as the composer emits it in edit mode. */
export interface ChannelEditEvent extends ChannelPostEvent {
  seq: number
}

/** A message being edited: the composer's prefill. */
export interface ChannelEditing {
  seq: number
  type: string
  values: Record<string, ChannelFieldValue>
  /** The current readable refs, so the picker shows titles rather than ids. */
  refs?: Record<string, ChannelReadableRef>
  /**
   * Ref properties that hold a ref the user may not read. They show the
   * restricted placeholder with a Replace button, count as filled for
   * `required`, and stay out of the emitted `values` and `rendered` unless
   * the user picks a replacement.
   */
  restricted?: string[]
}

/**
 * Where an inbox row leads. Like `NavItem`, it carries the component itself
 * (a router link) and its attributes, so the library needs no router.
 */
export interface InboxRowLink {
  /** What the row's link renders as; a plain anchor when absent. */
  as?: 'a' | Component
  /** Attributes for the link, such as `to` for a router link or `href`. */
  attrs: Record<string, unknown>
}

/**
 * A session that waits on the user. Every string is already formatted by
 * the app: the library does no date handling and no lookups.
 */
export interface InboxWaitingRow {
  id: string
  /** The anchor entity's title, as the server let the user read it. */
  anchorTitle: string
  /** The anchor entity's type label. */
  entityType: string
  /** The channel's title, or its type label. */
  channel: string
  /** The session state's label. */
  state: string
  /** Since when it waits, formatted. */
  since: string
  /** The machine-readable time of `since`, for `<time datetime>`. */
  sinceIso?: string
  link: InboxRowLink
}

/** An unread message that mentions the user or replies to the user. */
export interface InboxMentionRow {
  id: string
  kind: 'mention' | 'reply'
  /** The author's display name. */
  author: string
  anchorTitle: string
  channel: string
  /** When the message was posted, formatted. */
  at: string
  atIso?: string
  link: InboxRowLink
}

/** A channel with messages the user has not read. */
export interface InboxUnreadRow {
  id: string
  anchorTitle: string
  channel: string
  /** How many messages are unread. */
  unread: number
  link: InboxRowLink
}
