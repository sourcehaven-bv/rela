import { inboxRowKey } from '@/utils/inboxRoutes'
import type { NotificationPrefs } from '@/utils/notificationPrefs'
import type { ChannelAuthor, Inbox, InboxChannel, InboxSession } from '@/types/channels'

/**
 * What an inbox page could notify about (TKT-48S88RB), and which of it is
 * new. Pure, so the diff is tested without a browser.
 *
 * Identity:
 *
 *   - a mention or reply: `m:<row key>#<seq>`, the inbox page's own row id;
 *   - a waiting entry: `w:<row key>@<session generation>`, so a session that
 *     leaves a state and comes back to it notifies again.
 *
 * The notifier reads only the inbox's first page. Two rules keep that from
 * notifying late: the seen set keeps earlier ids (up to SEEN_CAP), so a row
 * that leaves the page and comes back is not new; and a row whose time is
 * older than the newest time of the previous read, less LATE_SLACK_MS, is
 * not shown, so a row that only now moved onto the page stays quiet. Both
 * times come from the server, so the client clock plays no part.
 */

export type Candidate =
  | {
      id: string
      kind: 'mention' | 'reply'
      channel: InboxChannel
      seq: number
      author: ChannelAuthor
      /** When the message was posted (server time). */
      at: string
    }
  | {
      id: string
      kind: 'waiting'
      channel: InboxChannel
      session: InboxSession
      /** When the session entered its state (server time). */
      at: string
    }

/** What the notifier remembers between reads. */
export interface Seen {
  ids: Set<string>
  /** The newest candidate time of the reads so far, or null for none. */
  newest: string | null
}

/** At most this many ids are kept; the ids of the latest read come first. */
export const SEEN_CAP = 500

/** How much older than the previous read's newest row a new row may be. */
export const LATE_SLACK_MS = 60_000

/** At most this many notifications per read; the rest become one summary. */
export const MAX_PER_READ = 5

/** Every candidate on the page, waiting entries first, mentions newest first. */
export function inboxCandidates(inbox: Inbox): Candidate[] {
  const out: Candidate[] = []
  for (const e of inbox.entries ?? []) {
    out.push({
      id: `w:${inboxRowKey(e.channel)}@${e.session.generation}`,
      kind: 'waiting',
      channel: e.channel,
      session: e.session,
      at: e.session.entered_at ?? e.since,
    })
  }
  for (const m of inbox.mentions ?? []) {
    out.push({
      id: `m:${inboxRowKey(m.channel)}#${m.seq}`,
      kind: m.kind,
      channel: m.channel,
      seq: m.seq,
      author: m.author,
      at: m.at,
    })
  }
  return out
}

function wanted(c: Candidate, prefs: NotificationPrefs): boolean {
  return c.kind === 'waiting' ? prefs.waiting : prefs.mentions
}

/** The outcome of one read. */
export interface Diff {
  /** What to show, at most MAX_PER_READ. */
  show: Candidate[]
  /** How many more new candidates were left out of show. */
  more: number
  /** What to store for the next read. */
  seen: Seen
}

function time(at: string | null | undefined): number {
  const t = at ? Date.parse(at) : NaN
  return Number.isNaN(t) ? NaN : t
}

function newestOf(candidates: Candidate[], prev: string | null): string | null {
  let best = prev
  for (const c of candidates) {
    if (!Number.isNaN(time(c.at)) && (best === null || time(c.at) > time(best))) best = c.at
  }
  return best
}

/**
 * Compares a read with the stored seen state. With none stored the read is
 * a silent baseline. Ids of kinds the user turned off still enter the seen
 * set, so turning a kind on later does not replay the whole inbox. A
 * message listed both as a mention and as a reply counts once.
 */
export function diffInbox(
  candidates: Candidate[],
  seen: Seen | null,
  prefs: NotificationPrefs
): Diff {
  const unique = [...new Map(candidates.map((c) => [c.id, c])).values()]
  const ids = [...new Set([...unique.map((c) => c.id), ...(seen?.ids ?? [])])].slice(0, SEEN_CAP)
  const next: Seen = { ids: new Set(ids), newest: newestOf(unique, seen?.newest ?? null) }
  if (seen === null) return { show: [], more: 0, seen: next }
  const floor = seen.newest === null ? NaN : time(seen.newest) - LATE_SLACK_MS
  const late = (c: Candidate) => !Number.isNaN(floor) && time(c.at) < floor
  const fresh = unique.filter((c) => !seen.ids.has(c.id) && wanted(c, prefs) && !late(c))
  return {
    show: fresh.slice(0, MAX_PER_READ),
    more: Math.max(0, fresh.length - MAX_PER_READ),
    seen: next,
  }
}
