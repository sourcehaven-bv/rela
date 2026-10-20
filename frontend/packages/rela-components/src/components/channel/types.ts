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
import type { TagColor } from '../../types'

/** A message property's type, as realtime.yaml declares it. */
export type ChannelPropertyType =
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
  type: ChannelPropertyType
  required?: boolean
  /** The most Unicode code points a string or markdown value may hold. */
  max?: number
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
}

/** A lifecycle operation of a channel type, such as close or archive. */
export interface ChannelLifecycleDef {
  name: string
  label: string
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

/** A stored property value: a scalar, or a ref. */
export type ChannelPropValue = string | number | boolean | ChannelRef

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
}

/** An ephemeral signal, shown as one compact line while it lasts. */
export interface ChannelEphemeralView {
  id: string
  type: string
  author: ChannelAuthorView
  props?: Record<string, ChannelPropValue>
}

/** The state of a channel. */
export type ChannelState = 'open' | 'closed' | 'archived'

/** One channel in an entity's channel list. */
export interface ChannelRowView {
  /** Unique within the list; the app's key for selecting it. */
  key: string
  title: string
  state: ChannelState
  /** A preformatted line under the title, such as the last activity. */
  summary?: string
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
  /** The properties the composer rendered, for an edit that carries the rest. */
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
}
