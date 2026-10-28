import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { defineComponent, reactive } from 'vue'
import { useChannelSchemaStore } from '@/stores/channelSchema'
import type { ChannelAnchor } from '@/api/channels'
import type { Channel, ChannelList, ChannelState, ChannelTypeSchema } from '@/types/channels'
import ChannelsSection from './ChannelsSection.vue'

// The `?channel=` deep link on an entity page (TKT-9YRRM5, AC 23). The inbox
// links there; the section opens the named channel, looks under the `all`
// filter once for a closed or archived one, and scrolls itself into view.

const listChannels = vi.hoisted(() =>
  vi.fn<(a: ChannelAnchor, state?: ChannelState) => Promise<ChannelList>>()
)
vi.mock('@/api/channels', async (orig) => ({
  ...(await orig<typeof import('@/api/channels')>()),
  listChannels,
}))
vi.mock('@/composables/useLiveSubscription', () => ({ useLiveSubscription: vi.fn() }))

const route = reactive<{ query: Record<string, unknown> }>({ query: {} })
vi.mock('vue-router', () => ({ useRoute: () => route }))

// The open channel, stubbed: these tests only look at which one opens.
const OpenChannelStub = defineComponent({
  name: 'OpenChannel',
  props: ['channelType', 'name', 'entityType', 'address', 'world'],
  template: '<div class="open-stub" :data-type="channelType.name" :data-name="name ?? \'\'" />',
})

function type(name: string, cardinality: 'one' | 'many'): ChannelTypeSchema {
  return {
    name,
    label: name,
    attach_to: ['ticket'],
    anchor: 'family',
    cardinality,
    remove_reason_required: false,
    typing: false,
    lifecycle: [],
    message_types: [],
  }
}

function ch(t: string, name: string | undefined, state: ChannelState = 'open'): Channel {
  return { anchor: 'TKT-1', type: t, name, state, last_seq: 1, rev: 1 }
}

function list(topics: Channel[]): ChannelList {
  return {
    anchor: 'TKT-1',
    types: [
      {
        type: 'case',
        label: 'Case',
        cardinality: 'one',
        create: false,
        channels: [ch('case', undefined)],
      },
      { type: 'topic', label: 'Topic', cardinality: 'many', create: false, channels: topics },
    ],
  }
}

const openTopics = [ch('topic', 'x')]
const allTopics = [ch('topic', 'x'), ch('topic', 'old', 'closed'), ch('topic', 'dusty', 'archived')]

let wrapper: VueWrapper | undefined
const scrolled = vi.fn()

beforeEach(() => {
  listChannels
    .mockReset()
    .mockImplementation(async (_a, state) => list(state === 'open' ? openTopics : allTopics))
  route.query = {}
  scrolled.mockReset()
  Element.prototype.scrollIntoView = scrolled
  const store = useChannelSchemaStore()
  store.channelTypes = [type('case', 'one'), type('topic', 'many')]
  store.loaded = true
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
})

async function mountSection() {
  wrapper = mount(ChannelsSection, {
    props: { entityType: 'ticket', entityId: 'TKT-1' },
    global: { stubs: { OpenChannel: OpenChannelStub } },
    attachTo: document.body,
  })
  await flushPromises()
  await flushPromises()
  return wrapper
}

function opened(w: VueWrapper): string | null {
  const el = w.find('.open-stub')
  if (!el.exists()) return null
  return `${el.attributes('data-type')}/${el.attributes('data-name')}`
}

function states(): (ChannelState | undefined)[] {
  return listChannels.mock.calls.map((c) => c[1])
}

describe('ChannelsSection ?channel= deep link', () => {
  it.each([
    { name: 'a cardinality-one channel as `case`', query: 'case', want: 'case/' },
    { name: 'a cardinality-one channel as `case/`', query: 'case/', want: 'case/' },
    { name: 'a named channel in the open list', query: 'topic/x', want: 'topic/x' },
  ])('opens $name without leaving the open filter', async ({ query, want }) => {
    route.query = { channel: query }
    const w = await mountSection()
    expect(opened(w)).toBe(want)
    expect(states()).toEqual(['open'])
    expect(scrolled).toHaveBeenCalledWith({ block: 'start' })
  })

  it.each([
    { name: 'closed', query: 'topic/old', want: 'topic/old' },
    { name: 'archived', query: 'topic/dusty', want: 'topic/dusty' },
  ])('opens a $name channel from the all filter', async ({ query, want }) => {
    route.query = { channel: query }
    const w = await mountSection()
    expect(opened(w)).toBe(want)
    expect(states()).toEqual(['open', undefined])
    expect(scrolled).toHaveBeenCalled()
  })

  it.each(['nope', 'topic/missing', '../x'])(
    'opens nothing for an unknown value %s and sends no request with it',
    async (query) => {
      route.query = { channel: query }
      const w = await mountSection()
      expect(opened(w)).toBeNull()
      // One switch to all, then it stops looking.
      expect(states()).toEqual(['open', undefined])
      for (const call of listChannels.mock.calls) {
        expect(JSON.stringify(call)).not.toContain(query)
      }
      expect(scrolled).not.toHaveBeenCalled()
    }
  )

  it('opens nothing and makes one request without a query', async () => {
    const w = await mountSection()
    expect(opened(w)).toBeNull()
    expect(states()).toEqual(['open'])
  })

  it('follows a query change on the same page', async () => {
    const w = await mountSection()
    expect(opened(w)).toBeNull()
    route.query = { channel: 'topic/x' }
    await flushPromises()
    expect(opened(w)).toBe('topic/x')
    route.query = { channel: 'topic/old' }
    await flushPromises()
    await flushPromises()
    expect(opened(w)).toBe('topic/old')
  })
})
