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
 * name and transitions is empty.
 */
export interface ChannelSession {
  state: string
  label?: string
  generation: number
  entered_at: string
  final?: boolean
  stranded?: boolean
  transitions?: ChannelSessionTransition[]
}

/** A message of message_type moves the session to to. */
export interface ChannelSessionTransition {
  message_type: string
  to: string
  /** 'trigger' when only a trigger's message fires the transition. */
  by?: 'trigger'
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
}

export interface ChannelHead {
  channel: Channel
  last?: Message
  ephemeral: EphemeralMessage[]
  _actions: ChannelActions
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
  lifecycle: ChannelLifecycleSchema[]
  message_types: ChannelMessageTypeSchema[]
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
  ]),
  ChannelSessionTransition: wireKeys<ChannelSessionTransition>()(['message_type', 'to', 'by']),
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
  ]),
  ChannelHead: wireKeys<ChannelHead>()(['channel', 'last', 'ephemeral', '_actions']),
  RefProp: wireKeys<RefProp>()(['id', 'type', 'title', 'restricted']),
  MessageRemoval: wireKeys<MessageRemoval>()(['at', 'kind', 'by', 'reason']),
  MessageAction: wireKeys<MessageAction>()(['name', 'status', 'at', 'promoted']),
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
    'lifecycle',
    'message_types',
  ]),
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
