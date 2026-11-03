import type { ChannelHead, ChannelState, InboxEntry, Message } from '@/types/channels'

/**
 * An inbox entry as the decision builders read a channel (TKT-V05706X).
 * The entry carries the head's decision inputs for the caller, computed by
 * the same server function as the head's `_actions`, so channelView's
 * decisionViews and choiceViews run on it unchanged.
 */

/**
 * A channel head built from entry: the channel state, the session (state,
 * generation, transitions, cause) and the caller's `post`, `transitions`
 * and `entries`. Everything else the head would carry is empty: the row
 * offers no message operation, lifecycle or typing.
 */
export function inboxHead(entry: InboxEntry): ChannelHead {
  const c = entry.channel
  const s = entry.session
  return {
    channel: {
      anchor: c.anchor,
      type: c.type,
      ...(c.name ? { name: c.name } : {}),
      state: c.state as ChannelState,
      ...(c.title ? { title: c.title } : {}),
      last_seq: entry.last_seq,
      rev: 0,
      session: {
        state: s.state,
        ...(s.label ? { label: s.label } : {}),
        status: s.status as NonNullable<ChannelHead['channel']['session']>['status'],
        ...(s.waiting_for ? { waiting_for: s.waiting_for } : {}),
        waiting_on_me: true,
        generation: s.generation,
        ...(s.entered_at ? { entered_at: s.entered_at } : {}),
        transitions: s.transitions ?? [],
        ...(s.cause ? { cause: s.cause } : {}),
      },
    },
    ephemeral: [],
    typing_now: [],
    _actions: {
      post: entry.actions?.post ?? [],
      edit_own: false,
      delete_own: false,
      remove: false,
      lifecycle: [],
      typing: false,
      transitions: entry.actions?.transitions ?? [],
      entries: entry.actions?.entries ?? [],
    },
  }
}

/** The messages the row holds: the waiting message, or none. */
export function inboxMessages(entry: InboxEntry): Message[] {
  return entry.message ? [entry.message] : []
}

/**
 * Whether the row offers the waiting message's choices. Stricter than the
 * channel page: the message must have no reply at all (`reply_count` 0),
 * since the row holds no reply to tell an answer from other talk.
 */
export function offersChoices(entry: InboxEntry): boolean {
  const m = entry.message
  return !!m && !m.removed && !m.reply_to && !m.reply_count
}

/**
 * The number of messages after the waiting message and its notice. The
 * row shows more than 99 as "99+".
 */
export function newerCount(entry: InboxEntry): number {
  const cause = entry.session.cause
  if (!cause) return 0
  return Math.max(0, entry.last_seq - cause - 1)
}
