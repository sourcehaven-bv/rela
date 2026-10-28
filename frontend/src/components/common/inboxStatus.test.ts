import { describe, it, expect } from 'vitest'
import { inboxStatus } from './inboxStatus'
import { t as i18nT } from '@/i18n'
import type { InboxSummary } from '@/types/channels'

const t = (key: string, n: number) => i18nT(key, { n }, n)

function summary(over: Partial<InboxSummary> = {}): InboxSummary {
  return {
    waiting: 0,
    waiting_truncated: false,
    unread_channels: 0,
    mentions: 0,
    degraded: false,
    ...over,
  }
}

describe('inboxStatus', () => {
  it.each([
    {
      name: 'waiting sessions count, in warning',
      in: { waiting: 2 },
      want: { tone: 'warning', label: '2 waiting', count: 2 },
    },
    {
      name: 'waiting plus mentions, still warning',
      in: { waiting: 1, mentions: 3, unread_channels: 4 },
      want: { tone: 'warning', label: '1 waiting, 3 mentions', count: 4 },
    },
    {
      name: 'mentions only, tone new',
      in: { mentions: 1, unread_channels: 2 },
      want: { tone: 'new', label: '1 mention', count: 1 },
    },
    {
      name: 'unread channels only, a dot without a count',
      in: { unread_channels: 3 },
      want: { tone: 'info', label: '3 unread channels' },
    },
    { name: 'nothing, no marker', in: {}, want: undefined },
    {
      name: 'degraded alone changes nothing',
      in: { degraded: true },
      want: undefined,
    },
  ])('$name', ({ in: over, want }) => {
    expect(inboxStatus(summary(over), t)).toEqual(want)
  })

  it('passes a count over 99 through; RlNavStatus caps the display', () => {
    expect(inboxStatus(summary({ waiting: 20000, waiting_truncated: true }), t)?.count).toBe(20000)
  })
})
