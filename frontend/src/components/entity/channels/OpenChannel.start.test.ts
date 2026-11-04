import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h as render } from 'vue'
import { useChannelSchemaStore } from '@/stores/channelSchema'
import { useToasts } from 'rela-components/components/feedback/useToasts'
import { registerEntityPlurals } from '@/api/entities'
import type { ChannelTarget } from '@/api/channels'
import type {
  ChannelHead,
  ChannelStart,
  ChannelTypeSchema,
  Message,
  MessagePage,
} from '@/types/channels'
import RlChannel from 'rela-components/components/channel/RlChannel.vue'
import RlChannelComposer from 'rela-components/components/channel/RlChannelComposer.vue'
import OpenChannel from './OpenChannel.vue'

// A decision started from an inbox row (TKT-V05706X, AC 10): it runs once
// the head and its page are held and the reveal ran, and only at the
// row's generation.

const api = vi.hoisted(() => ({
  getChannelHead: vi.fn<(t: ChannelTarget) => Promise<ChannelHead>>(),
  getMessages: vi.fn<(t: ChannelTarget, q?: { before?: number }) => Promise<MessagePage>>(),
  markChannelRead: vi.fn<(...a: unknown[]) => Promise<never>>(),
  postMessage: vi.fn(),
}))
vi.mock('@/api/channels', async (orig) => ({
  ...(await orig<typeof import('@/api/channels')>()),
  ...api,
}))
const subs = vi.hoisted(
  () => [] as { path: () => string | null; onChange: () => void }[]
)
vi.mock('@/composables/useLiveSubscription', () => ({
  useLiveSubscription: (_label: string, path: () => string | null, onChange: () => void) => {
    subs.push({ path, onChange })
  },
}))
vi.mock('@/composables/useEvents', () => ({ useEvents: () => ({ on: vi.fn() }) }))
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ confirm: vi.fn() }) }))
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {}, hash: '' }),
  useRouter: () => ({ push: vi.fn(), resolve: (to: { path: string }) => ({ href: to.path }) }),
}))

const log = (name: string, label: string, extra: object = {}) => ({
  name,
  label,
  persist: 'log',
  reply: true,
  actions: [],
  properties: [{ name: 'note', label: 'Note', type: 'string' }],
  ...extra,
})

const runType = {
  name: 'run',
  label: 'Run',
  attach_to: ['ticket'],
  anchor: 'family',
  cardinality: 'one',
  remove_reason_required: false,
  typing: false,
  lifecycle: [],
  message_types: [
    log('text', 'Message'),
    log('ask', 'Question', {
      properties: [
        { name: 'note', label: 'Question', type: 'string' },
        { name: 'options', label: 'Options', type: 'string', list: true },
      ],
      choices: { from: 'options', answer: 'answer', into: 'note', other: true },
    }),
    log('answer', 'Answer'),
  ],
  session: {
    label: 'Run',
    initial: 'working',
    states: [
      { name: 'working', label: 'Working', status: 'working' },
      { name: 'asking', label: 'Asking', status: 'input_required' },
    ],
  },
} as unknown as ChannelTypeSchema

function head(generation: number): ChannelHead {
  return {
    channel: {
      anchor: 'TKT-1',
      type: 'run',
      state: 'open',
      last_seq: 50,
      rev: 50,
      session: {
        state: 'asking',
        status: 'input_required',
        generation,
        cause: 41,
        transitions: [{ message_type: 'answer', to: 'working', entry: 0 }],
      },
    },
    ephemeral: [],
    typing_now: [],
    _actions: {
      post: ['text', 'answer'],
      edit_own: false,
      delete_own: false,
      remove: false,
      lifecycle: [],
      typing: false,
      transitions: ['answer'],
      entries: [{ message_type: 'answer', entry: 0 }],
    },
  }
}

function m(seq: number, type = 'text', props: Message['props'] = { note: `m${seq}` }): Message {
  return { seq, rev: seq, type, author: { user: 'runner' }, at: '2026-11-01T09:00:00Z', props }
}

const ask = m(41, 'ask', { note: 'Which?', options: ['Postgres', 'SQLite'] })

function pageOf(messages: Message[], over: Partial<MessagePage> = {}): MessagePage {
  return {
    channel: head(7).channel,
    messages,
    has_more: false,
    next_after_rev: 50,
    ...over,
  }
}

let wrapper: VueWrapper | undefined

beforeEach(() => {
  subs.length = 0
  api.getChannelHead.mockReset().mockResolvedValue(head(7))
  api.getMessages.mockReset().mockResolvedValue(pageOf([ask, m(42)]))
  api.markChannelRead.mockReset().mockReturnValue(new Promise<never>(() => {}))
  api.postMessage.mockReset().mockResolvedValue(m(51, 'answer'))
  Element.prototype.scrollIntoView = vi.fn()
  const store = useChannelSchemaStore()
  store.channelTypes = [runType]
  store.loaded = true
  useToasts().clear()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
})

async function settle() {
  for (let i = 0; i < 5; i++) await flushPromises()
}

function mountOpen(start: ChannelStart) {
  wrapper = mount(OpenChannel, {
    props: {
      entityType: 'ticket',
      address: 'TKT-1',
      channelType: runType,
      revealSeq: 41,
      start,
    },
    attachTo: document.body,
  })
  return wrapper
}

const choose = (key: string): ChannelStart => ({
  kind: 'choose',
  key,
  generation: 7,
  cause: 41,
  answerType: 'answer',
})

describe('OpenChannel start', () => {
  it('waits for the messages, then makes the choice once', async () => {
    let release!: (p: MessagePage) => void
    api.getMessages.mockReturnValueOnce(new Promise((r) => (release = r)))
    const w = mountOpen(choose('choice:41:1'))
    await settle()
    expect(w.emitted('started')).toBeUndefined()
    release(pageOf([ask, m(42)]))
    await settle()
    expect(w.emitted('started')).toHaveLength(1)
    expect(api.postMessage).toHaveBeenCalledWith(
      expect.objectContaining({ address: 'TKT-1' }),
      'answer',
      { note: 'SQLite' },
      41,
      7
    )
    // A head that arrives again does not start it a second time.
    await w.setProps({ revealSeq: 41 })
    await settle()
    expect(api.postMessage).toHaveBeenCalledOnce()
  })

  /** The newest page lacks the question; the older page waits for `release`. */
  function olderQuestion() {
    let release!: () => void
    const older = new Promise<void>((r) => (release = r))
    api.getMessages.mockImplementation(async (_t, q) => {
      if (q?.before === undefined) {
        return pageOf([m(48), m(49), m(50)], { has_more: true, next_before: 48 })
      }
      await older
      return pageOf([ask], { has_more: false })
    })
    return () => release()
  }

  async function send(w: VueWrapper, text: string) {
    const field = w.find('.rl-channel-composer textarea, .rl-channel-composer input[type="text"]')
    await field.setValue(text)
    const button = w.findAll('.rl-channel-composer button').find((b) => /Send/.test(b.text()))!
    await button.trigger('click')
    await settle()
  }

  it('Other on an older question waits for the jump, then answers as a reply', async () => {
    const release = olderQuestion()
    const w = mountOpen(choose('choice:41:other'))
    await settle()
    expect(api.getMessages).toHaveBeenCalledWith(expect.anything(), { before: 48 })
    expect(w.emitted('started')).toBeUndefined()
    release()
    await settle()
    expect(w.emitted('started')).toHaveLength(1)
    // The reply survives the page that the jump brought in.
    expect(w.find('.rl-channel-composer__reply').text()).toContain('Which?')
    await send(w, 'MySQL')
    expect(api.postMessage).toHaveBeenCalledWith(
      expect.objectContaining({ address: 'TKT-1' }),
      'answer',
      expect.objectContaining({ note: 'MySQL' }),
      41,
      7
    )
    expect(w.emitted('startSent')).toHaveLength(1)
  })

  it('a send after the started form ended is not the start', async () => {
    const w = mountOpen(choose('choice:41:other'))
    await settle()
    expect(w.emitted('started')).toHaveLength(1)
    // The user switches the composer away: the form decision ends. Answer
    // is a button type (Other), so the composer goes back to its own type
    // (TKT-A0RTC52) and the send is a plain message.
    w.findComponent(RlChannelComposer).vm.$emit('select', 'text')
    await settle()
    await send(w, 'MySQL')
    expect(api.postMessage).toHaveBeenCalledWith(
      expect.anything(),
      'text',
      expect.objectContaining({ note: 'MySQL' }),
      41,
      undefined
    )
    expect(w.emitted('startSent')).toBeUndefined()
  })

  it('the same form opened again in the channel after a discard is not the start', async () => {
    const w = mountOpen(choose('choice:41:other'))
    await settle()
    expect(w.emitted('started')).toHaveLength(1)
    // The user discards the started form, then opens the same answer form
    // with the channel's own Other button.
    w.findComponent(RlChannelComposer).vm.$emit('select', 'text')
    await settle()
    w.findComponent(RlChannel).vm.$emit('choose', 'choice:41:other')
    await settle()
    await send(w, 'MySQL')
    expect(api.postMessage).toHaveBeenCalledWith(
      expect.anything(),
      'answer',
      expect.objectContaining({ note: 'MySQL' }),
      41,
      7
    )
    expect(w.emitted('startSent')).toBeUndefined()
  })

  it('a session that moved on says so and starts nothing', async () => {
    api.getChannelHead.mockResolvedValue(head(8))
    const w = mountOpen(choose('choice:41:1'))
    await settle()
    expect(w.emitted('started')).toBeUndefined()
    expect(api.postMessage).not.toHaveBeenCalled()
    expect(useToasts().toasts.value.map((t) => t.title)).toContain(
      'The session moved on. Choose again.'
    )
  })
})

// TKT-A0RTC52 AC 7: two channels on one page; a move in one hands the focus
// on inside that channel only.
describe('OpenChannel focus follow with two channels', () => {
  const removedAsk: Message = {
    ...ask,
    rev: 51,
    removed: { kind: 'moderator', by: { user: 'bob' }, at: '2026-11-01T10:00:00Z' },
  }

  beforeEach(() => registerEntityPlurals(new Map([['ticket', 'tickets']])))

  function mountTwo() {
    const Host = defineComponent({
      setup() {
        return () =>
          render('div', [
            render(OpenChannel, {
              entityType: 'ticket',
              address: 'TKT-1',
              channelType: runType,
              'data-section': 'a',
            }),
            render(OpenChannel, {
              entityType: 'ticket',
              address: 'TKT-2',
              channelType: runType,
              'data-section': 'b',
            }),
          ])
      },
    })
    wrapper = mount(Host, { attachTo: document.body })
    return wrapper
  }
  const section = (w: VueWrapper, name: string) => w.find(`[data-section="${name}"]`)

  /** Removes the question in channel TKT-1 and lets its live frame land. */
  async function removeInA() {
    api.getMessages.mockImplementation(async (t: ChannelTarget) =>
      t.address === 'TKT-1'
        ? pageOf([removedAsk], { channel: { ...head(7).channel, rev: 51 }, next_after_rev: 51 })
        : pageOf([])
    )
    subs.find((x) => x.path()?.includes('TKT-1'))!.onChange()
    await settle()
  }

  it('the channel that moved hands the focus on inside itself', async () => {
    const w = mountTwo()
    await settle()
    const a = section(w, 'a')
    ;(a.find('[data-decision="jump"]').element as HTMLElement).focus()
    await removeInA()
    expect(a.find('[data-decision="jump"]').exists()).toBe(false)
    expect(document.activeElement).toBe(a.find('.rl-channel-session [data-decision]').element)
  })

  it('a focus in the other channel stays where it is', async () => {
    const w = mountTwo()
    await settle()
    const jumpB = section(w, 'b').find('[data-decision="jump"]').element as HTMLElement
    jumpB.focus()
    await removeInA()
    expect(section(w, 'a').find('.rl-channel-session [data-decision="answer"]').exists()).toBe(true)
    expect(document.activeElement).toBe(jumpB)
  })
})
