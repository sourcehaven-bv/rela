/**
 * The channel wire types (TKT-91BK6F), mirrored by hand from
 * internal/apiwire/v1/channels.go. CHANNEL_WIRE_FIELDS below lists the JSON
 * keys of each type. Each list is checked against its interface at compile
 * time (wireKeys), and wireFields.test.ts compares the lists with the field
 * sets the Go test TestChannelWireGolden writes. A field added or renamed in
 * Go, in a list or in an interface alone therefore fails the type check or
 * a test.
 */
import type { Warning } from './entity'

export interface ChannelAuthor {
  user: string
  tool?: string
  principal_type?: string
  /** A message the server posted itself, such as a trigger's. */
  system?: boolean
}

export type ChannelState = 'open' | 'closed' | 'archived'

export interface Channel {
  /** The entity address the channel hangs off (`ID` or `ID@face`). */
  anchor: string
  type: string
  /** Empty for a cardinality `one` channel type. */
  name?: string
  state: ChannelState
  title?: string
  created_by?: ChannelAuthor
  created_at?: string
  closed_at?: string
  archived_at?: string
  last_seq: number
  rev: number
  /** The channel's session; absent for a channel type without one. */
  session?: ChannelSession
}

/**
 * A channel's session: the state it is in and the transitions out of it.
 * generation counts the transitions (0: never moved). stranded marks a
 * stored state realtime.yaml no longer declares; label is then the stored
 * name and transitions is empty. timers are the after: entries of the
 * current state; due_at is when the first comes due, set only on an open
 * channel under a configuration that is not degraded.
 */
export interface ChannelSession {
  state: string
  label?: string
  generation: number
  entered_at: string
  final?: boolean
  stranded?: boolean
  transitions?: ChannelSessionTransition[]
  due_at?: string
  timers?: ChannelSessionTimer[]
  reminders?: ChannelSessionReminder[]
}

/**
 * A reminder active in the session's current state: it posts a message
 * every `every`. count is the number posted in the current span; max caps
 * it. due_at is the next reminder, set only on an open channel under a
 * configuration that is not degraded, and not once count reaches max.
 */
export interface ChannelSessionReminder {
  name: string
  label?: string
  every: string
  count: number
  max?: number
  due_at?: string
}

/**
 * A timer of the session's current state: after its period, counted from
 * since, the session moves to to. due_at follows ChannelSession.due_at.
 */
export interface ChannelSessionTimer {
  name: string
  label?: string
  to: string
  since: 'state' | 'session'
  due_at?: string
}

/** A message of message_type moves the session to to. */
export interface ChannelSessionTransition {
  message_type: string
  to: string
  /** 'trigger' when only a trigger's message fires the transition. */
  by?: 'trigger'
  /** The transition has a guard, so a message moves the session only when it holds. */
  guarded?: boolean
  /** The number of graph writes the transition makes; absent when none. */
  outputs?: number
}

export interface ChannelTypeList {
  type: string
  label?: string
  cardinality: 'one' | 'many'
  /** Whether the caller may create a channel of this type now. */
  create: boolean
  channels: Channel[]
}

export interface ChannelList {
  anchor: string
  types: ChannelTypeList[]
}

export interface ChannelActions {
  /** The message types the caller may post now. */
  post: string[]
  edit_own: boolean
  delete_own: boolean
  remove: boolean
  /** The lifecycle operations the caller may run now. */
  lifecycle: string[]
  /** Whether the caller may POST …/_typing now. */
  typing: boolean
  /**
   * The message types whose post by the caller would move the session now,
   * by name: a subset of post, empty without a session. A guarded
   * transition is listed because its guard is judged on the message.
   */
  transitions: string[]
}

export interface ChannelHead {
  channel: Channel
  last?: Message
  ephemeral: EphemeralMessage[]
  /**
   * Who else is typing, by `since`. Never the caller. Show it as it is: the
   * server expires an entry and frames the head then, so the client runs no
   * clock on it.
   */
  typing_now: TypingEntry[]
  _actions: ChannelActions
}

/** One live typing entry. A refresh keeps `since`. */
export interface TypingEntry {
  author: ChannelAuthor
  since: string
}

/** The answer to POST …/_typing (202). */
export interface TypingAccepted {
  /** How long the entry lives without another call. */
  ttl_seconds: number
}

/** A ref the caller may read: the server resolved it. */
export interface ReadableRef {
  id: string
  type: string
  title?: string
  restricted?: false
}

/**
 * A ref the caller may not read. It carries nothing else, and the type has
 * no `id`, so code cannot show what the server withheld.
 */
export interface RestrictedRef {
  restricted: true
}

/** The wire RefProp: a ref property value as the caller sees it. */
export type RefProp = ReadableRef | RestrictedRef

export interface MessageRemoval {
  at: string
  /** Who removed it: the author (delete) or a moderator (remove). */
  kind: 'author' | 'moderator'
  by?: ChannelAuthor
  reason?: string
}

export interface MessageAction {
  name: string
  status: 'pending' | 'done'
  at: string
  promoted?: RefProp
}

/**
 * The outputs mark of a session notice: the graph writes the move made, as
 * writer, the person whose message moved the session.
 */
export interface ChannelMessageOutputs {
  status: 'pending' | 'done' | 'failed'
  writer: ChannelAuthor
  results?: ChannelMessageOutputResult[]
}

/** What one output, or the on_final close, did. */
export interface ChannelMessageOutputResult {
  /** transition: the one result of a mark whose transition the config no longer declares. */
  kind: 'set' | 'create' | 'close' | 'transition'
  status: 'done' | 'failed' | 'skipped' | 'superseded' | 'started'
  /** Why the output failed; never an error text or an id. */
  code?: string
  /** For a set, the properties written; for an invalid output, those validation named. */
  props?: string[]
  /** The face a set writes, when the output names one. */
  face?: string
  /** The entity a create made. */
  entity?: RefProp
  warnings?: string[]
}

/**
 * The message operations a message's `_actions` names next to its action
 * names. The server reserves these names, so no action takes one.
 * wireFields.test.ts compares them with realtimecfg.MessageOpNames.
 */
export type MessageOp = 'edit' | 'delete' | 'remove' | 'release'

/** The MessageOp names, to tell them from action names in `_actions`. */
export const MESSAGE_OPS: ReadonlySet<string> = new Set<MessageOp>([
  'edit',
  'delete',
  'remove',
  'release',
])

/**
 * The lifecycle operation that deletes a channel, as realtimecfg's
 * LifecycleDelete names it. It is the one op drawn in the danger tone and
 * confirmed first.
 */
export const LIFECYCLE_DELETE = 'delete'

/** A prop value: a scalar, or a RefProp for a ref property. */
export type MessagePropValue = string | number | boolean | RefProp

export interface Message {
  seq: number
  rev: number
  /** Absent on a removed message. */
  type?: string
  author?: ChannelAuthor
  at?: string
  props?: Record<string, MessagePropValue>
  reply_to?: number
  reply_count?: number
  edited_at?: string
  triggered_by?: string
  removed?: MessageRemoval
  action?: MessageAction
  /** A session notice's outputs mark, in place of action. */
  outputs?: ChannelMessageOutputs
  /** Action names and MessageOp names the caller may use now. */
  _actions?: string[]
  /**
   * When `edit` stops being offered. Set while it is offered and the edit
   * window applies; re-read the message then.
   */
  edit_until?: string
  /**
   * When `release` starts being offered. Set while only the pending time
   * keeps it back; re-read the message then.
   */
  release_after?: string
}

export interface MessageActionResponse {
  message: Message
  warnings?: Warning[]
}

export interface MessagePage {
  channel: Channel
  messages: Message[]
  has_more: boolean
  next_after?: number
  next_before?: number
  /** The rev to pass as `after_rev` to follow changes. */
  next_after_rev?: number
}

export interface EphemeralMessage {
  id: string
  type: string
  author: ChannelAuthor
  at: string
  expires_at: string
  props?: Record<string, MessagePropValue>
}

export interface EphemeralAccepted {
  id: string
  expires_at: string
}

/** A prop value as a request sends it: a ref is its entity id. */
export type MessagePropInput = string | number | boolean

export interface PostMessageRequest {
  type: string
  props?: Record<string, MessagePropInput>
  reply_to?: number
}

export interface EditMessageRequest {
  props: Record<string, MessagePropInput>
}

export interface RemoveMessageRequest {
  reason?: string
}

export interface CreateChannelRequest {
  title?: string
}

/** GET /api/v1/_schema/channels. The same for every principal. */
export interface ChannelSchema {
  channel_types: ChannelTypeSchema[]
}

/**
 * Every `label` in the channel type descriptor is the label realtime.yaml
 * sets, or the name when it sets none: never empty, so show it as it is.
 * `create_label` is the one optional label.
 */
export interface ChannelTypeSchema {
  name: string
  label: string
  attach_to: string[]
  /** Whether a channel hangs off the entity family or one face. */
  anchor: 'family' | 'face'

  cardinality: 'one' | 'many'
  create_label?: string
  remove_reason_required: boolean
  /** Whether the type offers the built-in typing indicator. */
  typing: boolean
  lifecycle: ChannelLifecycleSchema[]
  message_types: ChannelMessageTypeSchema[]
  /** The type's session; absent when it has none. */
  session?: ChannelSessionSchema
}

/**
 * A channel type's session: its states in declaration order. The
 * transitions out of a channel's current state come with the channel.
 */
export interface ChannelSessionSchema {
  label: string
  initial: string
  states: ChannelSessionStateSchema[]
}

export interface ChannelSessionStateSchema {
  name: string
  label: string
  /** A state the session ends in. */
  final?: boolean
}

export interface ChannelLifecycleSchema {
  name: string
  label: string
}

export interface ChannelMessageTypeSchema {
  name: string
  label: string
  persist: 'log' | 'ephemeral'
  ttl_seconds?: number
  /** Whether a message of this type may reply to another. */
  reply: boolean
  /** Required first, then by name. */
  properties: ChannelPropertySchema[]
  actions: ChannelActionSchema[]
}

export type ChannelPropertyType =
  'string' | 'markdown' | 'integer' | 'boolean' | 'date' | 'datetime' | 'ref' | 'enum'

export interface ChannelPropertySchema {
  name: string
  label: string
  type: ChannelPropertyType
  required?: boolean
  /** The most code points a string or markdown value may hold. */
  max?: number
  /** The schema enum type the values come from, for labels and colours. */
  enum?: string
  values?: string[]
  /** The entity types a ref may point to. */
  to?: string[]
}

export interface ChannelActionSchema {
  name: string
  label: string
  promote_to?: string
}

/** Every key of T, of each member when T is a union. */
type AllKeys<T> = T extends unknown ? keyof T : never

/**
 * Returns keys unchanged, and fails to compile unless keys lists every key
 * of T and no other: a key T lacks is refused by the element type, and a key
 * keys lacks makes the argument require a `missing` property naming it.
 */
function wireKeys<T>() {
  return <const K extends readonly AllKeys<T>[]>(
    keys: K &
      ([Exclude<AllKeys<T>, K[number]>] extends [never]
        ? unknown
        : { missing: Exclude<AllKeys<T>, K[number]> })
  ): readonly string[] => keys as readonly string[]
}

/**
 * The JSON keys of each wire type, by the Go type name. Each list is tied to
 * its interface by wireKeys, so it cannot drift from the interface.
 */
export const CHANNEL_WIRE_FIELDS: Record<string, readonly string[]> = {
  ChannelAuthor: wireKeys<ChannelAuthor>()(['user', 'tool', 'principal_type', 'system']),
  Channel: wireKeys<Channel>()([
    'anchor',
    'type',
    'name',
    'state',
    'title',
    'created_by',
    'created_at',
    'closed_at',
    'archived_at',
    'last_seq',
    'rev',
    'session',
  ]),
  ChannelSession: wireKeys<ChannelSession>()([
    'state',
    'label',
    'generation',
    'entered_at',
    'final',
    'stranded',
    'transitions',
    'due_at',
    'timers',
    'reminders',
  ]),
  ChannelSessionReminder: wireKeys<ChannelSessionReminder>()([
    'name',
    'label',
    'every',
    'count',
    'max',
    'due_at',
  ]),
  ChannelSessionTimer: wireKeys<ChannelSessionTimer>()(['name', 'label', 'to', 'since', 'due_at']),
  ChannelSessionTransition: wireKeys<ChannelSessionTransition>()([
    'message_type',
    'to',
    'by',
    'guarded',
    'outputs',
  ]),
  ChannelTypeList: wireKeys<ChannelTypeList>()([
    'type',
    'label',
    'cardinality',
    'create',
    'channels',
  ]),
  ChannelList: wireKeys<ChannelList>()(['anchor', 'types']),
  ChannelActions: wireKeys<ChannelActions>()([
    'post',
    'edit_own',
    'delete_own',
    'remove',
    'lifecycle',
    'typing',
    'transitions',
  ]),
  ChannelHead: wireKeys<ChannelHead>()(['channel', 'last', 'ephemeral', 'typing_now', '_actions']),
  TypingEntry: wireKeys<TypingEntry>()(['author', 'since']),
  TypingAccepted: wireKeys<TypingAccepted>()(['ttl_seconds']),
  RefProp: wireKeys<RefProp>()(['id', 'type', 'title', 'restricted']),
  MessageRemoval: wireKeys<MessageRemoval>()(['at', 'kind', 'by', 'reason']),
  MessageAction: wireKeys<MessageAction>()(['name', 'status', 'at', 'promoted']),
  ChannelMessageOutputs: wireKeys<ChannelMessageOutputs>()(['status', 'writer', 'results']),
  ChannelMessageOutputResult: wireKeys<ChannelMessageOutputResult>()([
    'kind',
    'status',
    'code',
    'props',
    'face',
    'entity',
    'warnings',
  ]),
  Message: wireKeys<Message>()([
    'seq',
    'rev',
    'type',
    'author',
    'at',
    'props',
    'reply_to',
    'reply_count',
    'edited_at',
    'triggered_by',
    'removed',
    'action',
    'outputs',
    '_actions',
    'edit_until',
    'release_after',
  ]),
  MessageActionResponse: wireKeys<MessageActionResponse>()(['message', 'warnings']),
  MessagePage: wireKeys<MessagePage>()([
    'channel',
    'messages',
    'has_more',
    'next_after',
    'next_before',
    'next_after_rev',
  ]),
  EphemeralMessage: wireKeys<EphemeralMessage>()([
    'id',
    'type',
    'author',
    'at',
    'expires_at',
    'props',
  ]),
  EphemeralAccepted: wireKeys<EphemeralAccepted>()(['id', 'expires_at']),
  PostMessageRequest: wireKeys<PostMessageRequest>()(['type', 'props', 'reply_to']),
  EditMessageRequest: wireKeys<EditMessageRequest>()(['props']),
  RemoveMessageRequest: wireKeys<RemoveMessageRequest>()(['reason']),
  CreateChannelRequest: wireKeys<CreateChannelRequest>()(['title']),
  ChannelSchema: wireKeys<ChannelSchema>()(['channel_types']),
  ChannelTypeSchema: wireKeys<ChannelTypeSchema>()([
    'name',
    'label',
    'attach_to',
    'anchor',
    'cardinality',
    'create_label',
    'remove_reason_required',
    'typing',
    'lifecycle',
    'message_types',
    'session',
  ]),
  ChannelSessionSchema: wireKeys<ChannelSessionSchema>()(['label', 'initial', 'states']),
  ChannelSessionStateSchema: wireKeys<ChannelSessionStateSchema>()(['name', 'label', 'final']),
  ChannelLifecycleSchema: wireKeys<ChannelLifecycleSchema>()(['name', 'label']),
  ChannelMessageTypeSchema: wireKeys<ChannelMessageTypeSchema>()([
    'name',
    'label',
    'persist',
    'ttl_seconds',
    'reply',
    'properties',
    'actions',
  ]),
  ChannelPropertySchema: wireKeys<ChannelPropertySchema>()([
    'name',
    'label',
    'type',
    'required',
    'max',
    'enum',
    'values',
    'to',
  ]),
  ChannelActionSchema: wireKeys<ChannelActionSchema>()(['name', 'label', 'promote_to']),
}

/** The prefix of triggered_by on a session notice; only the server writes it. */
export const SESSION_NOTICE_MARKER = 'session:'

/**
 * Whether m is a session notice: the system message the server appends
 * when a message moves the session. It mirrors the server's check, so a
 * message type named `session` on a type without a session is not one.
 */
export function isSessionNotice(m: Pick<Message, 'type' | 'triggered_by'>): boolean {
  return m.type === 'session' && (m.triggered_by ?? '').startsWith(SESSION_NOTICE_MARKER)
}
