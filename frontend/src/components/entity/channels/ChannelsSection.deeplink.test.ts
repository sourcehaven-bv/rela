import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { defineComponent, reactive } from 'vue'
import { useChannelSchemaStore } from '@/stores/channelSchema'
import { useSchemaStore } from '@/stores/schema'
import type { ChannelAnchor, ChannelTarget } from '@/api/channels'
import type {
  Channel,
  ChannelHead,
  ChannelList,
  ChannelState,
  ChannelTypeSchema,
  Message,
  MessagePage,
} from '@/types/channels'
import ChannelsSection from './ChannelsSection.vue'

// The `?channel=` deep link on an entity page (TKT-9YRRM5, AC 23). The inbox
// links there; the section opens the named channel, looks under the `all`
// filter once for a closed or archived one, and scrolls itself into view.

const listChannels = vi.hoisted(() =>
  vi.fn<(a: ChannelAnchor, state?: ChannelState) => Promise<ChannelList>>()
)
// The message-link cases mount the real OpenChannel, which reads these.
const api = vi.hoisted(() => ({
  getChannelHead: vi.fn<(t: ChannelTarget) => Promise<ChannelHead>>(),
  getMessages: vi.fn<(t: ChannelTarget, q?: { before?: number }) => Promise<MessagePage>>(),
  markChannelRead: vi.fn<(...a: unknown[]) => Promise<never>>(),
}))
vi.mock('@/api/channels', async (orig) => ({
  ...(await orig<typeof import('@/api/channels')>()),
  listChannels,
  ...api,
}))
vi.mock('@/composables/useLiveSubscription', () => ({ useLiveSubscription: vi.fn() }))
vi.mock('@/composables/useEvents', () => ({ useEvents: () => ({ on: vi.fn() }) }))
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ confirm: vi.fn() }) }))

const route = reactive<{ query: Record<string, unknown>; hash: string }>({ query: {}, hash: '' })
const replace = vi.hoisted(() => vi.fn())
vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({
    replace,
    push: vi.fn(),
    resolve: (to: { path: string }) => ({ href: to.path }),
  }),
}))

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
  route.hash = ''
  replace
    .mockReset()
    .mockImplementation((to: { query: Record<string, unknown>; hash?: string }) => {
      route.query = to.query
      route.hash = to.hash ?? ''
    })
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

// TKT-O6RC8Z AC 10: `?seq=` beside `?channel=`, as the inbox's mention and
// reply rows link. These cases mount the real OpenChannel and RlChannel
// over mocked reads; the case channel hides `claim` and collapses
// `progress`, as the agent-runner example does.
describe('ChannelsSection ?seq= message link', () => {
  const logType = (name: string) => ({
    name,
    label: name,
    persist: 'log' as const,
    reply: true,
    actions: [],
    properties: [{ name: 'body', label: 'Body', type: 'markdown' as const }],
  })
  const caseType: ChannelTypeSchema = {
    ...type('case', 'one'),
    message_types: ['text', 'claim', 'progress'].map(logType),
  }
  const topicType: ChannelTypeSchema = {
    ...type('topic', 'many'),
    message_types: ['text', 'claim', 'progress'].map(logType),
  }

  function m(seq: number, t: string): Message {
    return {
      seq,
      rev: seq,
      type: t,
      author: { user: 'robot' },
      at: '2026-03-01T10:00:00Z',
      props: { body: `${t} ${seq}` },
      _actions: [],
    }
  }
  function head(t: ChannelTarget): ChannelHead {
    return {
      channel: { ...ch(t.channelType, t.name), last_seq: 20, rev: 20 },
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
  function pageOf(messages: Message[], over: Partial<MessagePage> = {}): MessagePage {
    return {
      channel: ch('case', undefined),
      messages,
      has_more: false,
      next_after_rev: 20,
      ...over,
    }
  }
  /** Serves newest as the first page and older[i] for the i-th older read. */
  function serve(newest: Message[], ...older: Message[][]) {
    api.getMessages.mockImplementation(async (_t, q) => {
      if (q?.before === undefined) {
        return pageOf(newest, {
          has_more: older.length > 0,
          next_before: newest[0]?.seq,
        })
      }
      const i = older.findIndex((p) => p.some((x) => x.seq < q.before!))
      const got = older[i] ?? []
      return pageOf(got, { has_more: i < older.length - 1, next_before: got[0]?.seq })
    })
  }

  beforeEach(() => {
    api.getChannelHead.mockReset().mockImplementation(async (t) => head(t))
    api.getMessages.mockReset()
    api.markChannelRead.mockReset().mockReturnValue(new Promise<never>(() => {}))
    useChannelSchemaStore().channelTypes = [caseType, topicType]
    const presentation = {
      messages: {
        claim: { display: 'hidden' as const },
        progress: { display: 'collapsed' as const },
      },
    }
    useSchemaStore().channelPresentations = new Map([
      ['case', presentation],
      ['topic', presentation],
    ])
  })

  async function mountReal() {
    wrapper = mount(ChannelsSection, {
      props: { entityType: 'ticket', entityId: 'TKT-1' },
      attachTo: document.body,
    })
    await settle()
    return wrapper
  }
  async function settle() {
    for (let i = 0; i < 4; i++) await flushPromises()
  }
  const row = (seq: number) => wrapper!.find(`.rl-channel-message[data-seq="${seq}"]`)
  const focused = (seq: number) => document.activeElement === row(seq).element
  const folds = () => wrapper!.findAll('button.rl-channel-fold')
  const announcement = () =>
    wrapper!.find('.rl-channel > p.rl-visually-hidden[aria-live="polite"]').text()
  const listRow = (title: string) =>
    wrapper!.findAll('button.rl-entity-channels__row').find((b) => b.text().includes(title))!

  it('seq reveals a hidden message on the first page', async () => {
    serve([m(10, 'text'), m(11, 'claim'), m(12, 'text')])
    route.query = { channel: 'case', seq: '11' }
    await mountReal()
    expect(row(11).exists()).toBe(true)
    expect(focused(11)).toBe(true)
    // The section's own scroll into view did not run after the reveal.
    // The log's scroll to the bottom sets scrollTop, so this does not show
    // the order of that scroll and the reveal.
    expect(scrolled).toHaveBeenLastCalledWith({ block: 'center' })
  })

  it('seq expands a collapsed run', async () => {
    serve([m(10, 'progress'), m(11, 'progress'), m(12, 'text')])
    route.query = { channel: 'case', seq: '10' }
    await mountReal()
    expect(folds()).toHaveLength(1)
    expect(folds()[0]!.attributes('aria-expanded')).toBe('true')
    expect(focused(10)).toBe(true)
  })

  it('seq on an older page loads and reveals', async () => {
    serve([m(12, 'text')], [m(5, 'claim'), m(6, 'text')])
    route.query = { channel: 'case', seq: '5' }
    await mountReal()
    expect(api.getMessages).toHaveBeenCalledWith(expect.anything(), { before: 12 })
    expect(row(5).exists()).toBe(true)
    expect(focused(5)).toBe(true)
  })

  it('older pages for a link are not announced', async () => {
    serve([m(12, 'text')], [m(8, 'claim'), m(9, 'claim')], [m(5, 'claim')])
    route.query = { channel: 'case', seq: '5' }
    await mountReal()
    expect(focused(5)).toBe(true)
    expect(row(8).exists()).toBe(false)
    expect(announcement()).toBe('')
  })

  it.each(['0', '-3', 'abc', '1.5', '', '99'])('bad seq %j is ignored', async (seq) => {
    serve([m(10, 'text'), m(11, 'claim'), m(12, 'text')])
    route.query = { channel: 'case', seq }
    await mountReal()
    expect(wrapper!.find('[data-testid="open-channel"]').exists()).toBe(true)
    expect(row(11).exists()).toBe(false)
    expect(wrapper!.find('.rl-banner').exists()).toBe(false)
    expect(document.activeElement?.closest('.rl-channel-message')).toBeNull()
  })

  it('seq change in the same channel reveals the new target without re-fetch', async () => {
    serve([m(9, 'progress'), m(10, 'progress'), m(11, 'claim'), m(12, 'text')])
    route.query = { channel: 'case', seq: '11' }
    await mountReal()
    expect(focused(11)).toBe(true)
    const reads = [listChannels, api.getChannelHead, api.getMessages].map(
      (f) => f.mock.calls.length
    )
    const open = wrapper!.find('[data-testid="open-channel"]').element

    route.query = { channel: 'case', seq: '10' }
    await settle()
    expect(focused(10)).toBe(true)
    expect(row(11).exists()).toBe(true)
    expect(
      [listChannels, api.getChannelHead, api.getMessages].map((f) => f.mock.calls.length)
    ).toEqual(reads)
    // The same mounted channel, neither closed nor reopened.
    expect(wrapper!.find('[data-testid="open-channel"]').element).toBe(open)
  })

  it('seq is dropped from the URL after the reveal', async () => {
    serve([m(10, 'text'), m(11, 'claim'), m(12, 'text')])
    route.query = { channel: 'case', seq: '11' }
    await mountReal()
    expect(replace).toHaveBeenCalledTimes(1)
    expect(replace).toHaveBeenCalledWith({ query: { channel: 'case' }, hash: '' })
    expect(route.query).toEqual({ channel: 'case' })

    // The same mention clicked again reveals again.
    ;(document.activeElement as HTMLElement | null)?.blur()
    route.query = { channel: 'case', seq: '11' }
    await settle()
    expect(focused(11)).toBe(true)
    expect(replace).toHaveBeenCalledTimes(2)
  })

  it('the reveal keeps the hash', async () => {
    serve([m(10, 'text'), m(11, 'claim'), m(12, 'text')])
    route.query = { channel: 'case', seq: '11' }
    route.hash = '#notes'
    await mountReal()
    expect(replace).toHaveBeenCalledWith({ query: { channel: 'case' }, hash: '#notes' })
  })

  it('a bad seq drops a pending reveal', async () => {
    // The case head answers only when the test says so, so the first
    // link's reveal is still pending when seq turns bad.
    let answer: () => void = () => {}
    api.getChannelHead.mockImplementation((t) =>
      t.channelType === 'case'
        ? new Promise((resolve) => {
            answer = () => resolve(head(t))
          })
        : Promise.resolve(head(t))
    )
    serve([m(10, 'text'), m(11, 'claim'), m(12, 'text')])
    route.query = { channel: 'case', seq: '11' }
    await mountReal()
    route.query = { channel: 'case', seq: 'abc' }
    await settle()
    answer()
    await settle()
    expect(row(12).exists()).toBe(true)
    expect(row(11).exists()).toBe(false)
    expect(replace).not.toHaveBeenCalled()
  })

  it('another channel gets no reveal', async () => {
    // The case head never answers, so the link's reveal stays pending.
    api.getChannelHead.mockImplementation((t) =>
      t.channelType === 'case' ? new Promise<never>(() => {}) : Promise.resolve(head(t))
    )
    serve([m(10, 'text'), m(11, 'claim'), m(12, 'text')])
    route.query = { channel: 'case', seq: '11' }
    await mountReal()
    await listRow('x').trigger('click')
    await settle()
    expect(wrapper!.find('.open-channel').exists()).toBe(true)
    expect(row(12).exists()).toBe(true)
    expect(row(11).exists()).toBe(false)
    expect(replace).not.toHaveBeenCalled()
  })

  it('reopening the linked channel does not jump again', async () => {
    serve([m(10, 'text'), m(11, 'claim'), m(12, 'text')])
    route.query = { channel: 'case', seq: '11' }
    await mountReal()
    expect(row(11).exists()).toBe(true)
    await listRow('Case').trigger('click')
    await settle()
    expect(wrapper!.find('.open-channel').exists()).toBe(false)
    await listRow('Case').trigger('click')
    await settle()
    expect(row(12).exists()).toBe(true)
    expect(row(11).exists()).toBe(false)
    expect(replace).toHaveBeenCalledTimes(1)
  })
})
