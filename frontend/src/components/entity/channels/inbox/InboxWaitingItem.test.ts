import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ApiError } from '@/api/errors'
import { useToasts } from 'rela-components/components/feedback/useToasts'
import type { ChannelTypeSchema, InboxEntry, Message } from '@/types/channels'
import InboxWaitingItem from './InboxWaitingItem.vue'

// The waiting detail of an inbox row (TKT-V05706X, AC 8, 9, 11, 11a, 12).

const api = vi.hoisted(() => ({
  postMessage: vi.fn(),
  getChannelSchema: vi.fn(),
}))
vi.mock('@/api/channels', async (orig) => ({
  ...(await orig<typeof import('@/api/channels')>()),
  ...api,
}))
const confirm = vi.hoisted(() => vi.fn(async () => true))
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ confirm }) }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))

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
    log('ask', 'Question', {
      properties: [
        { name: 'note', label: 'Question', type: 'string' },
        { name: 'options', label: 'Options', type: 'string', list: true },
      ],
      choices: { from: 'options', answer: 'answer', into: 'note', other: true },
    }),
    log('answer', 'Answer'),
    log('plan', 'Plan'),
    log('approve', 'Approve', { properties: [], decision_button: { confirm: true } }),
    log('revise', 'Revise', {
      properties: [{ name: 'why', label: 'Why', type: 'string', required: true }],
    }),
  ],
  session: {
    label: 'Run',
    initial: 'working',
    states: [
      { name: 'working', label: 'Working', status: 'working' },
      { name: 'asking', label: 'Asking', status: 'input_required' },
      { name: 'review', label: 'Review', status: 'input_required' },
    ],
  },
} as unknown as ChannelTypeSchema

const ask: Message = {
  seq: 41,
  rev: 41,
  type: 'ask',
  author: { user: 'runner' },
  at: '2026-11-01T09:00:00Z',
  props: { note: 'Which backend?', options: ['Postgres', 'SQLite'] },
}

const plan: Message = { ...ask, type: 'plan', props: { note: 'The plan' } }

function question(over: Partial<InboxEntry> = {}): InboxEntry {
  return {
    kind: 'waiting',
    channel: {
      anchor: 'TKT-2',
      entity_type: 'ticket',
      anchor_title: 'Two',
      type: 'run',
      state: 'open',
    },
    session: {
      state: 'asking',
      status: 'input_required',
      generation: 7,
      cause: 41,
      transitions: [{ message_type: 'answer', to: 'working', entry: 0 }],
    },
    roles: ['owner'],
    since: '2026-11-01T09:00:00Z',
    transitions: ['answer'],
    actions: {
      post: ['answer'],
      transitions: ['answer'],
      entries: [{ message_type: 'answer', entry: 0 }],
    },
    last_seq: 42,
    message: ask,
    links: { entity: '', channel: '' },
    ...over,
  }
}

function approval(over: Partial<InboxEntry> = {}): InboxEntry {
  return question({
    session: {
      state: 'review',
      status: 'input_required',
      generation: 7,
      cause: 41,
      transitions: [
        { message_type: 'approve', to: 'working', entry: 0 },
        { message_type: 'revise', to: 'working', entry: 1 },
      ],
    },
    transitions: ['approve', 'revise'],
    actions: {
      post: ['approve', 'revise'],
      transitions: ['approve', 'revise'],
      entries: [
        { message_type: 'approve', entry: 0 },
        { message_type: 'revise', entry: 1 },
      ],
    },
    message: plan,
    ...over,
  })
}

function problem(type: string, status = 409) {
  return new ApiError(type, {
    kind: 'http',
    status,
    problem: { type: `https://rela.dev/errors/${type}`, title: type, status },
    original: { response: { headers: {} } },
  })
}

let wrapper: VueWrapper | undefined
const afterPost = vi.fn(async () => {})
const refetch = vi.fn(async () => {})

async function mountItem(entry: InboxEntry) {
  wrapper = mount(InboxWaitingItem, { props: { entry, afterPost, refetch } })
  await flushPromises()
  return wrapper
}

function button(w: VueWrapper, text: string) {
  const b = w.findAll('button').find((x) => x.text().includes(text))
  if (!b) throw new Error(`no button ${text}`)
  return b
}

const toasts = () => useToasts().toasts.value.map((x) => x.title)

beforeEach(() => {
  setActivePinia(createPinia())
  api.postMessage.mockReset().mockResolvedValue({})
  api.getChannelSchema.mockReset().mockResolvedValue({ channel_types: [runType] })
  confirm.mockReset().mockResolvedValue(true)
  afterPost.mockClear()
  refetch.mockClear()
  useToasts().clear()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
})

async function loaded() {
  const { useChannelSchemaStore } = await import('@/stores/channelSchema')
  await useChannelSchemaStore().ensureLoaded()
}

describe('InboxWaitingItem', () => {
  beforeEach(loaded)

  it('a choice posts the answer as a reply at the entry generation', async () => {
    const w = await mountItem(question())
    expect(w.text()).toContain('Which backend?')
    await button(w, 'SQLite').trigger('click')
    await flushPromises()
    expect(api.postMessage).toHaveBeenCalledWith(
      { type: 'ticket', address: 'TKT-2', channelType: 'run' },
      'answer',
      { note: 'SQLite' },
      41,
      7
    )
    expect(afterPost).toHaveBeenCalledOnce()
  })

  it.each([
    { name: 'already replied to', m: { ...ask, reply_count: 1 } },
    { name: 'a reply itself', m: { ...ask, reply_to: 3 } },
  ])('offers no choices on a question $name', async ({ m }) => {
    const w = await mountItem(question({ message: m }))
    expect(w.findAll('button').some((b) => b.text().includes('SQLite'))).toBe(false)
  })

  it('Other answer asks the inbox to start the choice in the drawer', async () => {
    const w = await mountItem(question())
    await button(w, 'Other answer').trigger('click')
    expect(w.emitted('answer')).toEqual([
      [{ kind: 'choose', key: 'choice:41:other', generation: 7, cause: 41, answerType: 'answer' }],
    ])
    expect(api.postMessage).not.toHaveBeenCalled()
  })

  it('a one-click decision asks first, then posts at the generation', async () => {
    const w = await mountItem(approval())
    expect(w.text()).toContain('The plan')
    await button(w, 'Approve').trigger('click')
    await flushPromises()
    expect(confirm).toHaveBeenCalledOnce()
    expect(api.postMessage).toHaveBeenCalledWith(
      expect.objectContaining({ address: 'TKT-2' }),
      'approve',
      {},
      41,
      7
    )
    expect(afterPost).toHaveBeenCalledOnce()
  })

  it('a removed waiting message keeps the decisions, which reply to nothing', async () => {
    const removed: Message = {
      ...plan,
      props: undefined,
      removed: { kind: 'author', at: '2026-11-01T09:05:00Z' },
    }
    const w = await mountItem(approval({ message: removed }))
    await button(w, 'Approve').trigger('click')
    await flushPromises()
    expect(api.postMessage).toHaveBeenCalledWith(
      expect.objectContaining({ address: 'TKT-2' }),
      'approve',
      {},
      undefined,
      7
    )
  })

  it('a form decision on a removed message names no cause', async () => {
    const removed: Message = {
      ...plan,
      props: undefined,
      removed: { kind: 'author', at: '2026-11-01T09:05:00Z' },
    }
    const w = await mountItem(approval({ message: removed }))
    await button(w, 'Revise').trigger('click')
    expect(w.emitted('answer')).toEqual([[{ kind: 'decide', key: 'revise', generation: 7 }]])
  })

  it('a declined confirm posts nothing', async () => {
    confirm.mockResolvedValue(false)
    const w = await mountItem(approval())
    await button(w, 'Approve').trigger('click')
    await flushPromises()
    expect(api.postMessage).not.toHaveBeenCalled()
  })

  it('a form decision asks the inbox to start it in the drawer', async () => {
    const w = await mountItem(approval())
    await button(w, 'Revise').trigger('click')
    expect(w.emitted('answer')).toEqual([
      [{ kind: 'decide', key: 'revise', generation: 7, cause: 41 }],
    ])
  })

  it.each([
    {
      name: 'session_moved',
      err: problem('session_moved'),
      toast: 'The session moved on. Choose again.',
    },
    { name: 'a 403', err: problem('channel_post_denied', 403), toast: 'channel_post_denied' },
    { name: 'a 404', err: problem('channel_not_found', 404), toast: 'channel_not_found' },
  ])('a refusal by $name toasts and reads the inbox again', async ({ err, toast }) => {
    api.postMessage.mockRejectedValue(err)
    const w = await mountItem(question())
    await button(w, 'Postgres').trigger('click')
    await flushPromises()
    expect(toasts().join('|')).toContain(toast)
    expect(refetch).toHaveBeenCalled()
    expect(afterPost).not.toHaveBeenCalled()
  })

  it('a second click while one runs does nothing', async () => {
    let release!: () => void
    api.postMessage.mockReturnValue(new Promise<void>((r) => (release = r)))
    const w = await mountItem(question())
    await button(w, 'Postgres').trigger('click')
    await button(w, 'SQLite').trigger('click')
    release()
    await flushPromises()
    expect(api.postMessage).toHaveBeenCalledOnce()
  })

  it.each([
    { last: 42, text: undefined },
    { last: 45, text: '3 newer messages' },
    { last: 44, text: '2 newer messages' },
    { last: 400, text: '99+ newer messages' },
  ])('last_seq $last says $text', async ({ last, text }) => {
    const w = await mountItem(question({ last_seq: last }))
    if (!text) {
      expect(w.text()).not.toContain('newer')
      return
    }
    expect(w.text()).toContain(text)
    await button(w, 'Open channel').trigger('click')
    expect(w.emitted('open')).toHaveLength(1)
  })

  it('shows no buttons when the caller may post nothing', async () => {
    const w = await mountItem(approval({ actions: { post: [], transitions: [], entries: [] } }))
    expect(w.text()).toContain('The plan')
    expect(w.findAll('button').some((b) => b.text().includes('Approve'))).toBe(false)
  })

  it('shows no buttons for a type the descriptor lacks, and reloads it once', async () => {
    const before = api.getChannelSchema.mock.calls.length
    const w = await mountItem(
      approval({
        actions: { post: ['approve', 'later'], transitions: ['approve', 'later'], entries: [] },
        session: { ...approval().session, transitions: [{ message_type: 'later', to: 'working' }] },
      })
    )
    expect(w.findAll('button').some((b) => b.text().includes('later'))).toBe(false)
    expect(api.getChannelSchema.mock.calls.length).toBe(before + 1)
  })
})

describe('InboxWaitingItem before the descriptor', () => {
  it('shows no message and no buttons until the descriptor is held', async () => {
    api.getChannelSchema.mockReturnValue(new Promise(() => {}))
    const w = await mountItem(approval())
    expect(w.findAll('button')).toHaveLength(0)
  })
})
