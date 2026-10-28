import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { defineComponent } from 'vue'
import { ApiError } from '@/api/errors'
import { useToasts } from 'rela-components/components/feedback/useToasts'
import type { ChannelTarget } from '@/api/channels'
import type { Inbox, InboxChannel, ChannelMarkRead } from '@/types/channels'
import InboxView from './InboxView.vue'

// The inbox page (TKT-9YRRM5, AC 22 and 24): three sections from the wire,
// rows that link to their channel, and mark read per row.

const api = vi.hoisted(() => ({
  getInbox: vi.fn<() => Promise<Inbox>>(),
  markChannelRead: vi.fn<(t: ChannelTarget, seq?: number) => Promise<ChannelMarkRead>>(),
  getChannelSchema: vi.fn(async () => ({ channel_types: [] })),
}))
vi.mock('@/api/channels', async (orig) => ({
  ...(await orig<typeof import('@/api/channels')>()),
  ...api,
}))
const subs = vi.hoisted(() => [] as { label: string; path: unknown; onChange: () => void }[])
vi.mock('@/composables/useLiveSubscription', () => ({
  useLiveSubscription: (label: string, path: unknown, onChange: () => void) =>
    subs.push({ label, path, onChange }),
}))
vi.mock('@/composables/useEvents', () => ({ useEvents: () => ({ on: vi.fn() }) }))
vi.mock('vue-router', () => ({
  RouterLink: defineComponent({
    props: ['to'],
    template: '<a :href="to"><slot /></a>',
  }),
}))

function chan(over: Partial<InboxChannel> = {}): InboxChannel {
  return {
    anchor: 'TKT-12',
    entity_type: 'ticket',
    anchor_title: 'Fix login',
    type: 'case',
    state: 'open',
    ...over,
  }
}

const links = { entity: '', channel: '' }

function inbox(over: Partial<Inbox> = {}): Inbox {
  return {
    notice: '',
    degraded: false,
    count: 1,
    truncated: false,
    entries: [
      {
        kind: 'waiting',
        channel: chan({
          anchor: 'INC-1',
          entity_type: 'incident',
          anchor_title: 'Outage',
          type: 'room',
        }),
        session: { state: 'triage', label: 'Triage', status: 'input_required', generation: 1 },
        roles: ['lead'],
        since: '2026-10-28T09:00:00Z',
        transitions: [],
        links,
      },
    ],
    mentions: [
      {
        kind: 'mention',
        channel: chan(),
        seq: 7,
        at: '2026-10-28T10:04:00Z',
        message_type: 'note',
        author: { user: 'bob' },
        links,
      },
    ],
    mention_count: 1,
    unread_channels: [
      {
        channel: chan({ anchor: 'TKT-1@public', type: 'topic', name: 'n1', title: 'Planning' }),
        seq: 2,
        last_seq: 9,
        unread: 7,
        links,
      },
    ],
    unread_count: 1,
    ...over,
  }
}

let wrapper: VueWrapper | undefined

async function mountView() {
  wrapper = mount(InboxView, { global: { plugins: [createPinia(), PiniaColada] } })
  await flushPromises()
  return wrapper
}

function section(w: VueWrapper, name: string) {
  return w.findAll(`li[data-section="${name}"]`)
}

function markReadButton(row: ReturnType<VueWrapper['find']>) {
  const b = row.findAll('button').find((x) => x.text().includes('Mark read'))
  if (!b) throw new Error('no Mark read button')
  return b
}

beforeEach(() => {
  api.getInbox.mockReset().mockResolvedValue(inbox())
  api.markChannelRead.mockReset().mockResolvedValue({ seq: 9, unread: 0, last_seq: 9 })
  subs.length = 0
  useToasts().clear()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
})

describe('InboxView', () => {
  it('lists three sections, each row linking to its channel', async () => {
    const w = await mountView()
    expect(w.text()).toContain('Waiting on you')
    expect(w.text()).toContain('Mentions and replies')
    expect(w.text()).toContain('Unread channels')

    const [waiting] = section(w, 'waiting')
    expect(waiting!.text()).toContain('Outage')
    expect(waiting!.text()).toContain('Triage')
    expect(waiting!.find('a').attributes('href')).toBe('/entity/incident/INC-1?channel=room%2F')

    const [mention] = section(w, 'mentions')
    expect(mention!.text()).toContain('bob mentioned you')
    expect(mention!.text()).toContain('Fix login')
    expect(mention!.find('a').attributes('href')).toBe('/entity/ticket/TKT-12?channel=case%2F')

    const [unread] = section(w, 'unread')
    expect(unread!.text()).toContain('Planning')
    expect(unread!.find('a').attributes('href')).toBe(
      '/entity/ticket/TKT-1%40public?channel=topic%2Fn1'
    )
    expect(subs.map((s) => s.path)).toEqual(['/api/v1/_inbox'])
  })

  it('shows the empty state of each section', async () => {
    api.getInbox.mockResolvedValue(
      inbox({ entries: [], mentions: [], unread_channels: [], count: 0 })
    )
    const w = await mountView()
    expect(w.text()).toContain('Nothing waits on you.')
    expect(w.text()).toContain('No unread mentions or replies.')
    expect(w.text()).toContain('All channels are read.')
    expect(w.text()).not.toContain('Showing')
  })

  it.each([
    { name: 'more than the page holds', over: { count: 120 }, note: 'Showing 1 of 120' },
    {
      name: 'a truncated count',
      over: { count: 20000, truncated: true },
      note: 'Showing 1 of 20000+',
    },
  ])('notes $name', async ({ over, note }) => {
    api.getInbox.mockResolvedValue(inbox(over))
    const w = await mountView()
    expect(w.text()).toContain(note)
  })

  it('shows the degraded notice', async () => {
    api.getInbox.mockResolvedValue(inbox({ degraded: true }))
    const w = await mountView()
    expect(w.text()).toContain('Some channel settings could not be loaded.')
  })

  it('marks an unread channel read up to its last message, then refetches', async () => {
    const w = await mountView()
    const before = api.getInbox.mock.calls.length
    await markReadButton(section(w, 'unread')[0]!).trigger('click')
    await flushPromises()
    expect(api.markChannelRead).toHaveBeenCalledWith(
      { type: 'ticket', address: 'TKT-1@public', channelType: 'topic', name: 'n1' },
      9
    )
    expect(api.getInbox.mock.calls.length).toBe(before + 1)
  })

  it('marks a mention read up to its seq, also on a channel without a marker', async () => {
    // The case channel has no unread row: the caller never opened it, so
    // it has no marker. Marking the mention creates one at seq 7.
    const w = await mountView()
    const before = api.getInbox.mock.calls.length
    await markReadButton(section(w, 'mentions')[0]!).trigger('click')
    await flushPromises()
    expect(api.markChannelRead).toHaveBeenCalledWith(
      { type: 'ticket', address: 'TKT-12', channelType: 'case', name: undefined },
      7
    )
    expect(api.getInbox.mock.calls.length).toBe(before + 1)
  })

  it('reports a failed mark through the channel error rules and refetches', async () => {
    api.markChannelRead.mockRejectedValue(
      new ApiError('Channel not found', {
        kind: 'http',
        status: 404,
        problem: {
          type: 'https://rela.dev/errors/channel_not_found',
          title: 'Channel not found',
          status: 404,
        },
        original: {},
      })
    )
    const w = await mountView()
    const before = api.getInbox.mock.calls.length
    await markReadButton(section(w, 'unread')[0]!).trigger('click')
    await flushPromises()
    expect(useToasts().toasts.value.map((x) => x.title)).toContain('Channel not found')
    expect(api.getInbox.mock.calls.length).toBe(before + 1)
  })

  it('refetches on a live frame', async () => {
    await mountView()
    api.getInbox.mockResolvedValue(inbox({ entries: [], count: 0 }))
    subs[0]!.onChange()
    await flushPromises()
    expect(wrapper!.text()).toContain('Nothing waits on you.')
  })

  it('shows the load error and drops the rows on a refusal', async () => {
    const w = await mountView()
    api.getInbox.mockRejectedValue(
      new ApiError('Realtime is off', {
        kind: 'http',
        status: 404,
        problem: {
          type: 'https://rela.dev/errors/not_found',
          title: 'Realtime is off',
          status: 404,
        },
        original: {},
      })
    )
    subs[0]!.onChange()
    await flushPromises()
    expect(w.text()).toContain('The inbox could not be loaded.')
    expect(w.text()).toContain('Realtime is off')
    expect(section(w, 'waiting')).toHaveLength(0)
  })
})
