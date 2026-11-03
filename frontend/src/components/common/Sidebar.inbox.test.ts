import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import { createPinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { ApiError } from '@/api/errors'
import type { SidebarData } from '@/types'
import type { ChannelSchema, InboxSummary } from '@/types/channels'
import { resetLiveSubscriptions } from '@/composables/useLiveSubscription'

let sidebar: SidebarData

vi.mock('@/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api')>()),
  getSidebar: vi.fn(async () => sidebar),
}))
vi.mock('@/composables/useNavItems', () => ({
  useNavItems: () => ({ itemsFor: () => ({ entries: [] }) }),
}))

const channelApi = vi.hoisted(() => ({
  getChannelSchema: vi.fn<() => Promise<ChannelSchema>>(),
  getInboxSummary: vi.fn<() => Promise<InboxSummary>>(),
}))
vi.mock('@/api/channels', async (orig) => ({
  ...(await orig<typeof import('@/api/channels')>()),
  ...channelApi,
}))

import Sidebar from './Sidebar.vue'

const room = {
  name: 'room',
  label: 'Room',
  attach_to: ['feature'],
  anchor: 'family',
  cardinality: 'one',
  remove_reason_required: false,
  typing: false,
  lifecycle: [],
  message_types: [],
} as unknown as ChannelSchema['channel_types'][number]

function counts(over: Partial<InboxSummary> = {}): InboxSummary {
  return {
    waiting: 0,
    waiting_truncated: false,
    unread_channels: 0,
    mentions: 0,
    degraded: false,
    ...over,
  }
}

function problem(status: number, type: string) {
  return new ApiError('refused', {
    kind: 'http',
    status,
    problem: { type: `https://rela.dev/errors/${type}`, title: 'refused', status },
    original: {},
  })
}

/**
 * The pinned Inbox entry (TKT-9YRRM5, AC 20): under Search once the channel
 * schema has a channel type, with a badge from the summary, and gone when the
 * summary says the inbox is not available.
 */
describe('Sidebar inbox', () => {
  let router: Router
  let wrapper: VueWrapper | null = null

  beforeEach(async () => {
    sidebar = { app: { name: 'Atlas' }, navigation: [] } as unknown as SidebarData
    channelApi.getChannelSchema.mockReset().mockResolvedValue({ channel_types: [room] })
    channelApi.getInboxSummary.mockReset().mockResolvedValue(counts())
    resetLiveSubscriptions()
    router = createRouter({
      history: createWebHistory(),
      routes: [{ path: '/:p(.*)*', component: { template: '<div/>' } }],
    })
    await router.push('/')
    await router.isReady()
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    document.body.innerHTML = ''
  })

  async function mountSidebar() {
    wrapper = mount(Sidebar, {
      global: { plugins: [createPinia(), PiniaColada, router] },
    })
    await flushPromises()
    await flushPromises()
    return wrapper
  }

  function pinnedLabels(w: VueWrapper): string[] {
    return w.findAll('.rl-sidebar a').map((a) => a.text().trim())
  }

  function inboxRow(w: VueWrapper) {
    return w.findAll('.rl-sidebar a').find((a) => a.attributes('href') === '/inbox')
  }

  it('shows Inbox between Search and Analysis when a channel type exists', async () => {
    const w = await mountSidebar()
    const labels = pinnedLabels(w)
    const search = labels.findIndex((l) => l.startsWith('Search'))
    expect(labels[search + 1]).toMatch(/^Inbox/)
    expect(labels[search + 2]).toMatch(/^Analysis/)
  })

  it('has no Inbox and asks for no summary without channel types', async () => {
    channelApi.getChannelSchema.mockResolvedValue({ channel_types: [] })
    const w = await mountSidebar()
    expect(inboxRow(w)).toBeUndefined()
    expect(channelApi.getInboxSummary).not.toHaveBeenCalled()
  })

  it.each([
    {
      name: 'waiting sessions: a count in the warning tone',
      summary: counts({ waiting: 2, mentions: 1 }),
      count: '3',
      tone: 'warning',
      label: '2 waiting, 1 mention',
    },
    {
      name: 'mentions only: a count',
      summary: counts({ mentions: 4, unread_channels: 2 }),
      count: '4',
      tone: 'new',
      label: '4 mentions',
    },
    {
      name: 'unread channels only: a dot without a count',
      summary: counts({ unread_channels: 2 }),
      count: '',
      tone: 'info',
      label: '2 unread channels',
    },
  ])('$name', async ({ summary, count, tone, label }) => {
    channelApi.getInboxSummary.mockResolvedValue(summary)
    const w = await mountSidebar()
    const row = inboxRow(w)
    expect(row).toBeDefined()
    expect(row!.find(`.rl-nav-status--${tone}`).exists()).toBe(true)
    expect(row!.text()).toContain(label)
    const shownCount = row!.find('.rl-nav-status__count')
    expect(shownCount.exists() ? shownCount.text() : '').toBe(count)
  })

  it('shows no badge when nothing waits', async () => {
    const w = await mountSidebar()
    expect(inboxRow(w)!.find('.rl-nav-status').exists()).toBe(false)
  })

  it.each([
    { name: '404', err: problem(404, 'not_found') },
    { name: '403 no_principal', err: problem(403, 'no_principal') },
  ])('hides the item on a $name from the summary', async ({ err }) => {
    channelApi.getInboxSummary.mockRejectedValue(err)
    const w = await mountSidebar()
    expect(inboxRow(w)).toBeUndefined()
  })
})
