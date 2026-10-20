/**
 * The channel wire types (TKT-91BK6F), mirrored by hand from
 * internal/apiwire/v1/channels.go. CHANNEL_WIRE_FIELDS below lists the JSON
 * keys of each type; wireFields.test.ts compares it with the field sets the
 * Go test TestChannelWireGolden writes, so a field added or renamed on one
 * side alone fails a test. Keep both in step when you change a type here.
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
 */
export type MessageOp = 'edit' | 'delete' | 'remove' | 'release'

/** The MessageOp names, to tell them from action names in `_actions`. */
export const MESSAGE_OPS: ReadonlySet<string> = new Set<MessageOp>([
  'edit',
  'delete',
  'remove',
  'release',
])

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

export interface ChannelTypeSchema {
  name: string
  label: string
  attach_to: string[]
  anchor: string
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

/**
 * The JSON keys of each wire type, by the Go type name. Every key of the
 * interface above, optional or not, appears once.
 */
export const CHANNEL_WIRE_FIELDS: Record<string, readonly string[]> = {
  ChannelAuthor: ['user', 'tool', 'principal_type', 'system'],
  Channel: [
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
  ],
  ChannelTypeList: ['type', 'label', 'cardinality', 'create', 'channels'],
  ChannelList: ['anchor', 'types'],
  ChannelActions: ['post', 'edit_own', 'delete_own', 'remove', 'lifecycle'],
  ChannelHead: ['channel', 'last', 'ephemeral', '_actions'],
  RefProp: ['id', 'type', 'title', 'restricted'],
  MessageRemoval: ['at', 'kind', 'by', 'reason'],
  MessageAction: ['name', 'status', 'at', 'promoted'],
  Message: [
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
  ],
  MessageActionResponse: ['message', 'warnings'],
  MessagePage: ['channel', 'messages', 'has_more', 'next_after', 'next_before', 'next_after_rev'],
  EphemeralMessage: ['id', 'type', 'author', 'at', 'expires_at', 'props'],
  EphemeralAccepted: ['id', 'expires_at'],
  PostMessageRequest: ['type', 'props', 'reply_to'],
  EditMessageRequest: ['props'],
  RemoveMessageRequest: ['reason'],
  CreateChannelRequest: ['title'],
  ChannelSchema: ['channel_types'],
  ChannelTypeSchema: [
    'name',
    'label',
    'attach_to',
    'anchor',
    'cardinality',
    'create_label',
    'remove_reason_required',
    'lifecycle',
    'message_types',
  ],
  ChannelLifecycleSchema: ['name', 'label'],
  ChannelMessageTypeSchema: [
    'name',
    'label',
    'persist',
    'ttl_seconds',
    'reply',
    'properties',
    'actions',
  ],
  ChannelPropertySchema: ['name', 'label', 'type', 'required', 'max', 'enum', 'values', 'to'],
  ChannelActionSchema: ['name', 'label', 'promote_to'],
}
