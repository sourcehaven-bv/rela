import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h as render } from 'vue'
import { useSchemaStore } from '@/stores/schema'
import { registerEntityPlurals } from '@/api/entities'
import type { ChannelAnchor, ChannelTarget } from '@/api/channels'
import type {
  Channel,
  ChannelHead,
  ChannelList,
  ChannelSchema,
  ChannelTypeSchema,
  MessagePage,
} from '@/types/channels'
import type { ChannelPresentation } from '@/types'
import ChannelsPanel from './ChannelsPanel.vue'

// Channel placement (TKT-VXBE16H). EntityDetail mounts two blocks: one with
// placement above_relations before the first relation section, and the
// default one at the end. These tests mount both blocks as the page does,
// over a mocked channel API, and pin which types each shows, the headings,
// and the live subscriptions the two together hold.

const api = vi.hoisted(() => ({
  getChannelSchema: vi.fn<() => Promise<ChannelSchema>>(),
  listChannels: vi.fn<(a: ChannelAnchor, state?: string) => Promise<ChannelList>>(),
  getChannelHead: vi.fn<(t: ChannelTarget) => Promise<ChannelHead>>(),
  getMessages: vi.fn<(t: ChannelTarget) => Promise<MessagePage>>(),
  markChannelRead: vi.fn<(...a: unknown[]) => Promise<never>>(),
}))
vi.mock('@/api/channels', async (orig) => ({
  ...(await orig<typeof import('@/api/channels')>()),
  ...api,
}))

const subs: { label: string; path: () => string | null }[] = []
vi.mock('@/composables/useLiveSubscription', () => ({
  useLiveSubscription: (label: string, path: () => string | null) => {
    subs.push({ label, path })
  },
}))
vi.mock('@/composables/useEvents', () => ({ useEvents: () => ({ on: vi.fn() }) }))
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ confirm: vi.fn() }) }))
vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    resolve: (to: { path: string }) => ({ href: to.path }),
  }),
  useRoute: () => ({ query: {}, hash: '' }),
}))

function type(name: string, label: string): ChannelTypeSchema {
  return {
    name,
    label,
    attach_to: ['feature'],
    anchor: 'family',
    cardinality: 'one',
    remove_reason_required: false,
    typing: false,
    lifecycle: [],
    message_types: [],
  }
}

function channel(t: string): Channel {
  return { anchor: 'FEAT-1', type: t, state: 'open', last_seq: 0, rev: 0 }
}

function head(t: ChannelTarget): ChannelHead {
  return {
    channel: channel(t.channelType),
    ephemeral: [],
    typing_now: [],
    _actions: {
      post: [],
      edit_own: false,
      delete_own: false,
      remove: false,
      lifecycle: [],
      typing: false,
      transitions: [],
    },
  }
}

const types = [type('run', 'Run'), type('review', 'Review'), type('chat', 'Chat')]

let wrapper: VueWrapper | undefined

/** Mounts the two blocks in page order. */
async function mountPage() {
  const Page = defineComponent({
    setup() {
      const props = { entityType: 'feature', entityId: 'FEAT-1' }
      return () =>
        render('div', [
          render('div', { class: 'above' }, [
            render(ChannelsPanel, { ...props, placement: 'above_relations' }),
          ]),
          render('div', { class: 'end' }, [render(ChannelsPanel, props)]),
        ])
    },
  })
  wrapper = mount(Page, { attachTo: document.body })
  await flushPromises()
  await flushPromises()
  return wrapper
}

function setPlacements(p: Record<string, ChannelPresentation>) {
  useSchemaStore().channelPresentations = new Map(Object.entries(p))
}

function heading(block: 'above' | 'end'): string | null {
  const el = wrapper!.find(`.${block} .channels-panel h2`)
  return el.exists() ? el.text() : null
}

beforeEach(() => {
  registerEntityPlurals(new Map([['feature', 'features']]))
  subs.length = 0
  for (const fn of Object.values(api)) fn.mockReset()
  api.markChannelRead.mockReturnValue(new Promise<never>(() => {}))
  api.getChannelSchema.mockResolvedValue({ channel_types: types })
  api.listChannels.mockResolvedValue({
    anchor: 'FEAT-1',
    types: types.map((t) => ({
      type: t.name,
      label: t.label,
      cardinality: 'one' as const,
      create: false,
      channels: [channel(t.name)],
    })),
  })
  api.getChannelHead.mockImplementation(async (t) => head(t))
  api.getMessages.mockResolvedValue({
    channel: channel('run'),
    messages: [],
    has_more: false,
    next_after_rev: 0,
  })
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
})

describe('ChannelsPanel placement', () => {
  it('renders one block at the end, headed Channels, without placements', async () => {
    setPlacements({})
    await mountPage()
    expect(heading('above')).toBeNull()
    expect(heading('end')).toBe('Channels')
    expect(wrapper!.findAll('.channels-panel')).toHaveLength(1)
  })

  it('renders nothing above for an unknown placement value', async () => {
    setPlacements({ run: { placement: 'tab' } })
    await mountPage()
    expect(heading('above')).toBeNull()
    expect(heading('end')).toBe('Channels')
  })

  it('moves a placed type to the block above and heads it with its label', async () => {
    setPlacements({ run: { placement: 'above_relations' } })
    await mountPage()
    expect(heading('above')).toBe('Run')
    expect(heading('end')).toBe('Channels')
    expect(wrapper!.find('.above').text()).not.toContain('Review')
    expect(wrapper!.find('.end').text()).toContain('Review')
    expect(wrapper!.find('.end').text()).toContain('Chat')
    expect(wrapper!.find('.end').text()).not.toContain('Run')
  })

  it('shares one block for types with the same placement', async () => {
    setPlacements({
      run: { placement: 'above_relations' },
      review: { placement: 'above_relations' },
    })
    await mountPage()
    expect(heading('above')).toBe('Channels')
    expect(wrapper!.find('.above').text()).toContain('Run')
    expect(wrapper!.find('.above').text()).toContain('Review')
    // The one type left at the end is headed with its label.
    expect(heading('end')).toBe('Chat')
  })

  it('renders nothing at the end when every type is placed above', async () => {
    setPlacements({
      run: { placement: 'above_relations' },
      review: { placement: 'above_relations' },
      chat: { placement: 'above_relations' },
    })
    await mountPage()
    // The page has one block, so it keeps the default heading.
    expect(heading('above')).toBe('Channels')
    expect(heading('end')).toBeNull()
  })

  it('gives the two headings distinct ids', async () => {
    setPlacements({ run: { placement: 'above_relations' } })
    await mountPage()
    const sections = wrapper!.findAll('.channels-panel')
    expect(sections).toHaveLength(2)
    const ids = sections.map((s) => s.attributes('aria-labelledby'))
    expect(new Set(ids).size).toBe(2)
    for (const [i, s] of sections.entries()) {
      expect(s.find('h2').attributes('id')).toBe(ids[i])
    }
  })

  it('holds at most two live keys per block', async () => {
    // Each block in single mode: its list and its one open channel.
    setPlacements({ run: { placement: 'above_relations' } })
    api.listChannels.mockResolvedValue({
      anchor: 'FEAT-1',
      types: ['run', 'chat'].map((name) => ({
        type: name,
        label: name,
        cardinality: 'one' as const,
        create: false,
        channels: [channel(name)],
      })),
    })
    await mountPage()
    // Each registration is its own key in the live registry, so two blocks
    // listing the same anchor count twice.
    const live = subs.map((s) => s.path()).filter((p) => p !== null)
    expect(live.sort()).toEqual([
      '/api/v1/features/FEAT-1/_channels/chat',
      '/api/v1/features/FEAT-1/_channels/run',
      '/api/v1/features/FEAT-1/_channels?state=open',
      '/api/v1/features/FEAT-1/_channels?state=open',
    ])
  })

  it('holds at most two live keys per block with a channel chosen from a list', async () => {
    // The end block lists two channels; choosing one drops the list key.
    setPlacements({ run: { placement: 'above_relations' } })
    await mountPage()
    const live = () => subs.map((s) => s.path()).filter((p) => p !== null)
    expect(live()).toContain('/api/v1/features/FEAT-1/_channels/run')
    await wrapper!.find('.end .rl-entity-channels__row').trigger('click')
    await flushPromises()
    await flushPromises()
    expect(live().sort()).toEqual([
      '/api/v1/features/FEAT-1/_channels/review',
      '/api/v1/features/FEAT-1/_channels/run',
      '/api/v1/features/FEAT-1/_channels?state=open',
    ])
  })

  it('follows a reload that moves a type to the other block', async () => {
    setPlacements({ run: { placement: 'above_relations' } })
    await mountPage()
    expect(heading('above')).toBe('Run')
    setPlacements({ review: { placement: 'above_relations' } })
    await flushPromises()
    await flushPromises()
    expect(heading('above')).toBe('Review')
    expect(wrapper!.find('.above').text()).not.toContain('Run')
    expect(wrapper!.find('.end').text()).toContain('Run')
    expect(wrapper!.find('.end').text()).not.toContain('Review')
  })

  it('shows a type the held schema does not know in the end block', async () => {
    setPlacements({ run: { placement: 'above_relations' } })
    api.listChannels.mockResolvedValue({
      anchor: 'FEAT-1',
      types: ['run', 'chat', 'fresh'].map((name) => ({
        type: name,
        label: name === 'fresh' ? 'Fresh' : name,
        cardinality: 'one' as const,
        create: false,
        channels: [channel(name)],
      })),
    })
    await mountPage()
    expect(wrapper!.find('.end').text()).toContain('Fresh')
    expect(wrapper!.find('.above').text()).not.toContain('Fresh')
  })
})
