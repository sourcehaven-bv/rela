import { api } from './client'
import { getPlural } from './entities'
import { addressSegment } from '@/utils/entityAddress'
import type {
  Channel,
  ChannelHead,
  ChannelList,
  ChannelSchema,
  ChannelState,
  EphemeralAccepted,
  Message,
  MessageActionResponse,
  MessagePage,
  MessagePropInput,
} from '@/types/channels'

/**
 * Typed client for the channel routes under `/{plural}/{address}/_channels`
 * (TKT-91BK6F) and the channel type descriptor at `/_schema/channels`.
 *
 * `address` names the face (`ID@face`) on a type with faces, as in
 * usePresence. No request carries `?world=`: the address already names the
 * face, and the server answers a non-default world on these routes with 422
 * `world_unsupported` (worldCapablePath in internal/dataentry/world.go).
 */

/** What names one entity's channels. */
export interface ChannelAnchor {
  /** The entity type. */
  type: string
  /** `ID` on a type without faces, `ID@face` on one with faces. */
  address: string
}

/** What names one channel. */
export interface ChannelTarget extends ChannelAnchor {
  channelType: string
  /** The channel name; absent for a cardinality `one` channel type. */
  name?: string
}

/**
 * The anchor path, relative to /api/v1. Throws when the address does not
 * have the shape of an entity address, so it cannot redirect the request.
 */
export function channelsPath(a: ChannelAnchor): string {
  return `/${getPlural(a.type)}/${addressSegment(a.address)}/_channels`
}

/** The channel path, relative to /api/v1. */
export function channelPath(t: ChannelTarget): string {
  const name = t.name ? `/${encodeURIComponent(t.name)}` : ''
  return `${channelsPath(t)}/${encodeURIComponent(t.channelType)}${name}`
}

/** GET the channels of one entity, grouped by channel type. */
export async function listChannels(a: ChannelAnchor, state?: ChannelState): Promise<ChannelList> {
  return api.get<ChannelList>(channelsPath(a), state ? { state } : undefined)
}

/** GET the head of a channel: the channel, its last message, signals and affordances. */
export async function getChannelHead(t: ChannelTarget): Promise<ChannelHead> {
  return api.get<ChannelHead>(channelPath(t))
}

/**
 * The cursor of one message page request. Set one of `after`, `before` or
 * `afterRev`, or none for the newest page. `afterRev` 0 follows an empty
 * channel.
 */
export interface MessagePageQuery {
  after?: number
  before?: number
  afterRev?: number
  limit?: number
}

/** GET one page of messages. */
export async function getMessages(
  t: ChannelTarget,
  q: MessagePageQuery = {},
  signal?: AbortSignal
): Promise<MessagePage> {
  const params: Record<string, unknown> = {}
  if (q.after !== undefined) params.after = q.after
  if (q.before !== undefined) params.before = q.before
  if (q.afterRev !== undefined) params.after_rev = q.afterRev
  if (q.limit !== undefined) params.limit = q.limit
  return api.get<MessagePage>(`${channelPath(t)}/messages`, params, signal)
}

/**
 * A post's answer: the stored message (201), or the acceptance of an
 * ephemeral signal (202), which the head lists until it expires.
 */
export type PostResult =
  { kind: 'message'; message: Message } | { kind: 'ephemeral'; accepted: EphemeralAccepted }

/** POST a message. */
export async function postMessage(
  t: ChannelTarget,
  type: string,
  props: Record<string, MessagePropInput>,
  replyTo?: number
): Promise<PostResult> {
  const body: Record<string, unknown> = { type, props }
  if (replyTo) body.reply_to = replyTo
  const res = await api
    .getAxios()
    .post<Message | EphemeralAccepted>(`${channelPath(t)}/messages`, body)
  if (res.status === 202) return { kind: 'ephemeral', accepted: res.data as EphemeralAccepted }
  return { kind: 'message', message: res.data as Message }
}

/** PATCH a message. The props replace every property of the message. */
export async function editMessage(
  t: ChannelTarget,
  seq: number,
  props: Record<string, MessagePropInput>
): Promise<Message> {
  return api.patch<Message>(`${channelPath(t)}/messages/${seq}`, { props })
}

/**
 * DELETE a message. Without a reason it is the author's delete; with one,
 * a moderator's remove. The reason travels in the body, never the URL.
 */
export async function removeMessage(
  t: ChannelTarget,
  seq: number,
  reason?: string
): Promise<Message> {
  const res = await api
    .getAxios()
    .delete<Message>(`${channelPath(t)}/messages/${seq}`, { data: reason ? { reason } : {} })
  return res.data
}

/** POST a message action, such as an ack or a promote. */
export async function runMessageAction(
  t: ChannelTarget,
  seq: number,
  action: string
): Promise<MessageActionResponse> {
  return api.post<MessageActionResponse>(
    `${channelPath(t)}/messages/${seq}/_action/${encodeURIComponent(action)}`,
    {}
  )
}

/** DELETE a message's pending action, so another can run. */
export async function releaseMessageAction(t: ChannelTarget, seq: number): Promise<Message> {
  const res = await api
    .getAxios()
    .delete<Message>(`${channelPath(t)}/messages/${seq}/_action`, { data: {} })
  return res.data
}

/**
 * POST a lifecycle operation. It answers the channel, or null after a
 * delete (204).
 */
export async function runLifecycle(t: ChannelTarget, op: string): Promise<Channel | null> {
  const res = await api
    .getAxios()
    .post<Channel | ''>(`${channelPath(t)}/_lifecycle/${encodeURIComponent(op)}`, {})
  return res.status === 204 || !res.data ? null : res.data
}

/** POST a new channel of a cardinality `many` channel type. */
export async function createChannel(
  a: ChannelAnchor,
  channelType: string,
  title?: string
): Promise<Channel> {
  return api.post<Channel>(
    `${channelsPath(a)}/${encodeURIComponent(channelType)}`,
    title ? { title } : {}
  )
}

/** GET the channel type descriptor. The same for every principal. */
export async function getChannelSchema(): Promise<ChannelSchema> {
  return api.get<ChannelSchema>('/_schema/channels')
}
