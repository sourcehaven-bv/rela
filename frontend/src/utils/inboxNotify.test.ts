import { describe, it, expect } from 'vitest'
import { DEFAULT_PREFS, type Seen } from './notificationPrefs'
import { LATE_SLACK_MS, MAX_PER_READ, SEEN_CAP, diffInbox, inboxCandidates } from './inboxNotify'
import type { Inbox, InboxChannel, InboxEntry, InboxMention } from '@/types/channels'

const chan = (anchor: string): InboxChannel => ({
  anchor,
  entity_type: 'ticket',
  anchor_title: `Title ${anchor}`,
  type: 'chat',
  state: 'open',
})

const AT = '2026-11-10T10:00:00Z'
const later = (ms: number) => new Date(Date.parse(AT) + ms).toISOString()

const mention = (anchor: string, seq: number, kind: 'mention' | 'reply' = 'mention', at = AT) =>
  ({
    kind,
    channel: chan(anchor),
    seq,
    at,
    message_type: 'text',
    author: { user: 'bob' },
    links: { entity: '', channel: '' },
  }) as InboxMention

const waiting = (anchor: string, generation: number, at = AT) =>
  ({
    kind: 'waiting',
    channel: chan(anchor),
    since: AT,
    session: { state: 'review', status: 'waiting', generation, entered_at: at },
  }) as unknown as InboxEntry

const seen = (ids: string[], newest: string | null = null): Seen => ({ ids: new Set(ids), newest })

const inbox = (entries: InboxEntry[], mentions: InboxMention[]): Inbox => ({
  notice: '',
  degraded: false,
  entries,
  mentions,
})

const on = { ...DEFAULT_PREFS, enabled: true }

describe('inboxNotify', () => {
  it('identifies mentions by row and seq, waiting entries by row and generation', () => {
    const ids = inboxCandidates(inbox([waiting('T-1', 3)], [mention('T-2', 7)])).map((c) => c.id)
    expect(ids).toEqual(['w:ticket/T-1/chat/@3', 'm:ticket/T-2/chat/#7'])
  })

  it('is a silent baseline without a stored set', () => {
    const c = inboxCandidates(inbox([waiting('T-1', 1)], [mention('T-2', 1)]))
    const d = diffInbox(c, null, on)
    expect(d.show).toEqual([])
    expect(d.seen.ids.size).toBe(2)
    expect(d.seen.newest).toBe(AT)
  })

  it('shows only what is new, and keeps earlier ids with this read first', () => {
    const c = inboxCandidates(inbox([waiting('T-1', 2)], [mention('T-2', 1), mention('T-2', 2)]))
    const d = diffInbox(c, seen(['w:ticket/T-1/chat/@1', 'm:ticket/T-2/chat/#1', 'gone']), on)
    expect(d.show.map((x) => x.id)).toEqual(['w:ticket/T-1/chat/@2', 'm:ticket/T-2/chat/#2'])
    expect([...d.seen.ids]).toEqual([
      'w:ticket/T-1/chat/@2',
      'm:ticket/T-2/chat/#1',
      'm:ticket/T-2/chat/#2',
      'w:ticket/T-1/chat/@1',
      'gone',
    ])
  })

  it('does not notify a row that comes back onto the page', () => {
    const first = diffInbox(inboxCandidates(inbox([], [mention('T-1', 1)])), seen([]), on)
    const away = diffInbox(inboxCandidates(inbox([], [])), first.seen, on)
    const back = diffInbox(inboxCandidates(inbox([], [mention('T-1', 1)])), away.seen, on)
    expect(back.show).toEqual([])
  })

  it('does not notify a row older than the previous read, beyond the slack', () => {
    const prev = seen([], later(10 * LATE_SLACK_MS))
    const c = inboxCandidates(
      inbox(
        [],
        [mention('T-1', 1, 'mention', AT), mention('T-1', 2, 'mention', later(9.5 * LATE_SLACK_MS))]
      )
    )
    expect(diffInbox(c, prev, on).show.map((x) => x.id)).toEqual(['m:ticket/T-1/chat/#2'])
  })

  it('notifies a waiting session that re-entered its state after the last read', () => {
    const prev = seen(['w:ticket/T-1/chat/@1'], later(1000))
    const c = inboxCandidates(inbox([waiting('T-1', 2, later(2000))], []))
    expect(diffInbox(c, prev, on).show.map((x) => x.id)).toEqual(['w:ticket/T-1/chat/@2'])
  })

  it('counts a message listed as both mention and reply once', () => {
    const c = inboxCandidates(inbox([], [mention('T-1', 4), mention('T-1', 4, 'reply')]))
    const d = diffInbox(c, seen([]), on)
    expect(d.show).toHaveLength(1)
    expect(d.more).toBe(0)
  })

  it('keeps at most SEEN_CAP ids', () => {
    const old = Array.from({ length: SEEN_CAP }, (_, i) => `old-${i}`)
    const d = diffInbox(inboxCandidates(inbox([], [mention('T-1', 1)])), seen(old), on)
    expect(d.seen.ids.size).toBe(SEEN_CAP)
    expect(d.seen.ids.has('m:ticket/T-1/chat/#1')).toBe(true)
  })

  it('skips turned-off kinds but still marks them seen', () => {
    const c = inboxCandidates(inbox([waiting('T-1', 1)], [mention('T-2', 1, 'reply')]))
    const d = diffInbox(c, seen([]), { ...on, waiting: false })
    expect(d.show.map((x) => x.kind)).toEqual(['reply'])
    expect(d.seen.ids.size).toBe(2)
  })

  it('caps a burst and counts the rest', () => {
    const many = Array.from({ length: 12 }, (_, i) => mention('T-1', i + 1))
    const d = diffInbox(inboxCandidates(inbox([], many)), seen([]), on)
    expect(d.show).toHaveLength(MAX_PER_READ)
    expect(d.more).toBe(12 - MAX_PER_READ)
  })

  it('shows nothing for an empty inbox', () => {
    expect(diffInbox(inboxCandidates(inbox([], [])), seen(['x']), on).show).toEqual([])
  })
})
