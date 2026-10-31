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
  /**
   * The caller's read marker. Only the channel list and the head carry it,
   * and only for a stored channel the caller has opened or posted in.
   */
  read?: ChannelRead
}

/**
 * Where the caller stopped reading a channel. seq is the highest message
 * seq read (0: opened, nothing read). unread counts the messages above it,
 * leaving out removed messages, session notices and messages of a type
 * with `unread: false`, and stops at 100.
 */
export interface ChannelRead {
  seq: number
  unread: number
}

/**
 * The answer to PUT …/_read: the stored marker, which may be higher than
 * the seq sent, and the channel's last seq.
 */
export interface ChannelMarkRead {
  seq: number
  unread: number
  last_seq: number
}

/** The body of PUT …/_read. Without seq the channel is read up to its head. */
export interface MarkReadRequest {
  seq?: number
}

/**
 * A channel's session: the state it is in and the transitions out of it.
 * generation counts the transitions (0: never moved). stranded marks a
 * stored state realtime.yaml no longer declares; label is then the stored
 * name and transitions is empty. timers are the after: entries of the
 * current state; due_at is when the first comes due, set only on an open
 * channel under a configuration that is not degraded. status is the
 * state's status (a stranded state reads failed); waiting_for lists whom an
 * input_required state waits for; waiting_on_me is per reader. entered_at
 * is absent for a channel that is not stored yet.
 */
export interface ChannelSession {
  state: string
  label?: string
  status: SessionStatus
  waiting_for?: ChannelSessionWaiting[]
  waiting_on_me?: boolean
  generation: number
  entered_at?: string
  final?: boolean
  stranded?: boolean
  transitions?: ChannelSessionTransition[]
  due_at?: string
  timers?: ChannelSessionTimer[]
  reminders?: ChannelSessionReminder[]
}

/** A session state's status, after the MCP tasks statuses. */
export type SessionStatus = 'working' | 'input_required' | 'completed' | 'cancelled' | 'failed'

/**
 * One role an input_required state waits for. holders are, for a role read
 * from an anchor property, that property's values as the reader may see
 * them: entity data, shown as text and never followed. A creator role has
 * none; the channel's created_by names the creator.
 */
export interface ChannelSessionWaiting {
  role: string
  label?: string
  holders?: string[]
}

/**
 * GET /api/v1/_inbox: the sessions that wait on the caller, oldest wait
 * first. count is the whole window, not this page; truncated counts only
 * rows the caller may read; next_cursor is absent on the last page. With
 * kinds=unread the waiting fields are absent. unread_channels and
 * unread_count come only on the first page when unread is asked for.
 */
export interface Inbox {
  notice: string
  entries?: InboxEntry[]
  count?: number
  truncated?: boolean
  degraded: boolean
  next_cursor?: string
  unread_channels?: InboxUnread[]
  unread_count?: number
  /** Messages that mention the caller or reply to the caller's messages. */
  mentions?: InboxMention[]
  /** Like unread_count, for mentions; only when mention or reply is asked for. */
  mention_count?: number
}

/**
 * GET /api/v1/_inbox/summary: the counts of the caller's inbox first page,
 * for the sidebar badge. waiting, waiting_truncated, unread_channels and
 * mentions equal count, truncated, unread_count and mention_count of the
 * inbox read with default parameters.
 */
export interface InboxSummary {
  waiting: number
  waiting_truncated: boolean
  unread_channels: number
  mentions: number
  degraded: boolean
}

/**
 * A message that mentions the caller (kind "mention") or replies to a
 * message of the caller (kind "reply"). reply_to is the seq replied to.
 */
export interface InboxMention {
  kind: 'mention' | 'reply'
  channel: InboxChannel
  seq: number
  at: string
  message_type: string
  author: ChannelAuthor
  reply_to?: number
  links: InboxLinks
}

/** A channel with messages the caller has not read. */
export interface InboxUnread {
  channel: InboxChannel
  seq: number
  last_seq: number
  unread: number
  links: InboxLinks
}

/** One session that waits on the caller. kind is "waiting". */
export interface InboxEntry {
  kind: string
  channel: InboxChannel
  session: InboxSession
  roles: string[]
  since: string
  transitions: string[]
  links: InboxLinks
}

/** The channel of an inbox entry; anchor_title is redacted for the caller. */
export interface InboxChannel {
  anchor: string
  entity_type: string
  anchor_title: string
  type: string
  name?: string
  title?: string
  state: string
}

/** The part of a session that says what it waits for. */
export interface InboxSession {
  state: string
  label?: string
  status: string
  waiting_for?: ChannelSessionWaiting[]
  generation: number
  entered_at?: string
}

/**
 * The API paths of an inbox entry's anchor entity and channel, and for a
 * mention the message itself.
 */
export interface InboxLinks {
  entity: string
  channel: string
  message?: string
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
  /** The number of graph writes the timer's move makes, as the session's run_as; absent when none. */
  outputs?: number
}

/**
 * A message of message_type moves the session to to. A type may appear
 * more than once, once per entry of its list, in the order the server
 * tries the entries; the first whose poster rule and guard hold moves the
 * session.
 */
export interface ChannelSessionTransition {
  message_type: string
  to: string
  /** 'trigger' when only a trigger's message fires the transition. */
  by?: 'trigger'
  /** The transition has a guard, so a message moves the session only when it holds. */
  guarded?: boolean
  /** The number of graph writes the transition makes; absent when none. */
  outputs?: number
  /** The entry's place in its list, from 0; a single mapping is entry 0. */
  entry?: number
  /** The entry's one-click preset, when the file gives it a button. */
  button?: ChannelSessionButton
}

/**
 * How the web app draws a decision button (TKT-TXZZZ2). It does not change
 * what a post does. `confirm` is true to ask before a one-click post, false
 * never to ask, and absent for the derived rule (ask into a final state).
 * `confirm_text` is the question, as written; it comes only with `confirm`
 * true. `style` is absent for the derived look.
 */
export interface ChannelButtonOptions {
  confirm?: boolean
  confirm_text?: string
  style?: 'default' | 'primary' | 'danger'
}

/**
 * A preset of a transition entry: the text on the button and the props a
 * click posts, in stored form, posted as they are. It is one-click unless
 * `compose` is true: then a click opens the composer with the props fixed,
 * and the Send adds the type's other required properties (TKT-53FN0L). Its
 * options already hold the type's `decision_button` for each one it leaves
 * out; a compose button never carries `confirm`.
 */
export interface ChannelSessionButton extends ChannelButtonOptions {
  label: string
  props?: Record<string, MessagePropInput>
  compose?: boolean
}

/** One transition entry, by message type and its place in the type's list. */
export interface ChannelEntryRef {
  message_type: string
  entry: number
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
  /**
   * The transition entries the caller's post could take now, sorted by
   * type and entry. A button entry's preset shows only when it is listed.
   */
  entries?: ChannelEntryRef[]
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
  /**
   * Why the output failed; never an error text or an id: forbidden,
   * field_denied, invalid, conflict, not_found, unconfirmed, expired,
   * transition_removed, run_as_changed or error.
   */
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

/**
 * A prop value: a scalar, a RefProp for a ref property, or the items of a
 * list property.
 */
export type MessagePropValue = string | number | boolean | RefProp | string[]

/**
 * One person or agent a message mentions. text is the code span text as
 * the author wrote it, which the renderer matches exactly. A user carries
 * ref; a job or login does not. A user the reader may not read is left out.
 */
export interface MessageMention {
  text: string
  kind: 'user' | 'job' | 'login'
  ref?: ReadableRef
}

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
  /** The people and agents the message mentions; absent on a removed message. */
  mentions?: MessageMention[]
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
export type MessagePropInput = string | number | boolean | string[]

export interface PostMessageRequest {
  type: string
  props?: Record<string, MessagePropInput>
  reply_to?: number
  /**
   * The session generation the post was decided against; a post at another
   * generation is refused with 409 session_moved.
   */
  expect_generation?: number
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
  status: SessionStatus
  /** The roles an input_required state waits for. */
  waiting_for?: string[]
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
  /** Options in a list property, shown as reply buttons. */
  choices?: ChannelChoicesSchema
  /** How the type's decision button looks; absent when the file sets none. */
  decision_button?: ChannelButtonOptions
}

/**
 * The `choices:` of a question type: `from` is its list property, and a
 * click posts `answer` with the option in `into`, as a reply to the
 * question. `other` adds a button that opens the composer.
 */
export interface ChannelChoicesSchema {
  from: string
  answer: string
  into: string
  other: boolean
}

export type ChannelPropertyType =
  'string' | 'markdown' | 'integer' | 'boolean' | 'date' | 'datetime' | 'ref' | 'enum'

export interface ChannelPropertySchema {
  name: string
  label: string
  type: ChannelPropertyType
  required?: boolean
  /**
   * The most code points a string or markdown value may hold; for a list,
   * the most each item may hold.
   */
  max?: number
  /** Set for a list of strings (type string): the value is an array. */
  list?: boolean
  /** The most items of a list property. */
  max_items?: number
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
    'read',
  ]),
  ChannelRead: wireKeys<ChannelRead>()(['seq', 'unread']),
  ChannelMarkRead: wireKeys<ChannelMarkRead>()(['seq', 'unread', 'last_seq']),
  MarkReadRequest: wireKeys<MarkReadRequest>()(['seq']),
  ChannelSession: wireKeys<ChannelSession>()([
    'state',
    'label',
    'status',
    'waiting_for',
    'waiting_on_me',
    'generation',
    'entered_at',
    'final',
    'stranded',
    'transitions',
    'due_at',
    'timers',
    'reminders',
  ]),
  ChannelSessionWaiting: wireKeys<ChannelSessionWaiting>()(['role', 'label', 'holders']),
  ChannelSessionReminder: wireKeys<ChannelSessionReminder>()([
    'name',
    'label',
    'every',
    'count',
    'max',
    'due_at',
  ]),
  ChannelSessionTimer: wireKeys<ChannelSessionTimer>()([
    'name',
    'label',
    'to',
    'since',
    'due_at',
    'outputs',
  ]),
  ChannelSessionTransition: wireKeys<ChannelSessionTransition>()([
    'message_type',
    'to',
    'by',
    'guarded',
    'outputs',
    'entry',
    'button',
  ]),
  ChannelSessionButton: wireKeys<ChannelSessionButton>()([
    'label',
    'props',
    'compose',
    'confirm',
    'confirm_text',
    'style',
  ]),
  ChannelEntryRef: wireKeys<ChannelEntryRef>()(['message_type', 'entry']),
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
    'entries',
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
    'mentions',
  ]),
  MessageMention: wireKeys<MessageMention>()(['text', 'kind', 'ref']),
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
  PostMessageRequest: wireKeys<PostMessageRequest>()([
    'type',
    'props',
    'reply_to',
    'expect_generation',
  ]),
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
  ChannelSessionStateSchema: wireKeys<ChannelSessionStateSchema>()([
    'name',
    'label',
    'final',
    'status',
    'waiting_for',
  ]),
  ChannelLifecycleSchema: wireKeys<ChannelLifecycleSchema>()(['name', 'label']),
  Inbox: wireKeys<Inbox>()([
    'notice',
    'entries',
    'count',
    'truncated',
    'degraded',
    'next_cursor',
    'unread_channels',
    'unread_count',
    'mentions',
    'mention_count',
  ]),
  InboxSummary: wireKeys<InboxSummary>()([
    'waiting',
    'waiting_truncated',
    'unread_channels',
    'mentions',
    'degraded',
  ]),
  InboxMention: wireKeys<InboxMention>()([
    'kind',
    'channel',
    'seq',
    'at',
    'message_type',
    'author',
    'reply_to',
    'links',
  ]),
  InboxUnread: wireKeys<InboxUnread>()(['channel', 'seq', 'last_seq', 'unread', 'links']),
  InboxEntry: wireKeys<InboxEntry>()([
    'kind',
    'channel',
    'session',
    'roles',
    'since',
    'transitions',
    'links',
  ]),
  InboxChannel: wireKeys<InboxChannel>()([
    'anchor',
    'entity_type',
    'anchor_title',
    'type',
    'name',
    'title',
    'state',
  ]),
  InboxSession: wireKeys<InboxSession>()([
    'state',
    'label',
    'status',
    'waiting_for',
    'generation',
    'entered_at',
  ]),
  InboxLinks: wireKeys<InboxLinks>()(['entity', 'channel', 'message']),
  ChannelMessageTypeSchema: wireKeys<ChannelMessageTypeSchema>()([
    'name',
    'label',
    'persist',
    'ttl_seconds',
    'reply',
    'properties',
    'actions',
    'choices',
    'decision_button',
  ]),
  ChannelDecisionButtonSchema: wireKeys<ChannelButtonOptions>()([
    'confirm',
    'confirm_text',
    'style',
  ]),
  ChannelChoicesSchema: wireKeys<ChannelChoicesSchema>()(['from', 'answer', 'into', 'other']),
  ChannelPropertySchema: wireKeys<ChannelPropertySchema>()([
    'name',
    'label',
    'type',
    'required',
    'max',
    'list',
    'max_items',
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
