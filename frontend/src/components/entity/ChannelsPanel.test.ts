import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { ApiError } from '@/api/errors'
import { useSchemaStore } from '@/stores/schema'
import { registerEntityPlurals } from '@/api/entities'
import { useToasts } from 'rela-components/components/feedback/useToasts'
import type { ChannelAnchor, ChannelTarget, PostResult } from '@/api/channels'
import type {
  Channel,
  ChannelHead,
  ChannelList,
  ChannelSchema,
  ChannelTypeSchema,
  Message,
  MessagePage,
} from '@/types/channels'
import ChannelsPanel from './ChannelsPanel.vue'
import RlChannelSession from 'rela-components/components/channel/RlChannelSession.vue'
import RlChannelComposer from 'rela-components/components/channel/RlChannelComposer.vue'

// The channel panel on an entity page (TKT-91BK6F). These tests mount the
// real panel, composables and library components over a mocked channel
// API, and pin the acceptance criteria that live in the app: the edit that
// keeps hidden props (AC 11), the error mapping by problem type (AC 14),
// ephemeral expiry (AC 15), state changes while open (AC 16), Release
// (AC 18), and the held-content rule for a refused versus a transient read.

const api = vi.hoisted(() => ({
  getChannelSchema: vi.fn<() => Promise<ChannelSchema>>(),
  listChannels: vi.fn<(a: ChannelAnchor, state?: string) => Promise<ChannelList>>(),
  getChannelHead: vi.fn<(t: ChannelTarget) => Promise<ChannelHead>>(),
  getMessages: vi.fn<(t: ChannelTarget, q?: { afterRev?: number }) => Promise<MessagePage>>(),
  postMessage: vi.fn<(...a: unknown[]) => Promise<PostResult>>(),
  editMessage: vi.fn<(...a: unknown[]) => Promise<Message>>(),
  removeMessage: vi.fn<(...a: unknown[]) => Promise<Message>>(),
  runMessageAction: vi.fn(),
  releaseMessageAction: vi.fn<(...a: unknown[]) => Promise<Message>>(),
  runLifecycle: vi.fn<(...a: unknown[]) => Promise<Channel | null>>(),
  createChannel: vi.fn<(...a: unknown[]) => Promise<Channel>>(),
}))
vi.mock('@/api/channels', async (orig) => ({
  ...(await orig<typeof import('@/api/channels')>()),
  ...api,
}))

const subs: { label: string; path: () => string | null; onChange: () => void }[] = []
vi.mock('@/composables/useLiveSubscription', () => ({
  useLiveSubscription: (label: string, path: () => string | null, onChange: () => void) => {
    subs.push({ label, path, onChange })
  },
}))
vi.mock('@/composables/useEvents', () => ({ useEvents: () => ({ on: vi.fn() }) }))

const confirm = vi.hoisted(() => vi.fn<(o: { danger?: boolean }) => Promise<boolean>>())
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ confirm }) }))

const push = vi.hoisted(() => vi.fn())
vi.mock('vue-router', () => ({
  useRouter: () => ({
    push,
    resolve: (to: { path: string }) => ({ href: to.path }),
  }),
  useRoute: () => ({ query: {} }),
}))

// --- Fixtures ----------------------------------------------------------------

const room: ChannelTypeSchema = {
  name: 'room',
  label: 'Room',
  attach_to: ['feature'],
  anchor: 'family',
  cardinality: 'one',
  remove_reason_required: true,
  typing: false,
  lifecycle: [
    { name: 'close', label: 'Close' },
    { name: 'delete', label: 'Delete' },
  ],
  message_types: [
    {
      name: 'card',
      label: 'Card',
      persist: 'log',
      reply: true,
      actions: [{ name: 'accept', label: 'Accept' }],
      properties: [
        { name: 'body', label: 'Body', type: 'markdown', required: true },
        { name: 'note', label: 'Note', type: 'string' },
        { name: 'owner', label: 'Owner', type: 'ref', to: ['person'] },
      ],
    },
    {
      name: 'typing',
      label: 'Typing',
      persist: 'ephemeral',
      reply: false,
      actions: [],
      properties: [{ name: 'note', label: 'Note', type: 'string' }],
    },
  ],
}

function channel(over: Partial<Channel> = {}): Channel {
  return { anchor: 'FEAT-1', type: 'room', state: 'open', last_seq: 1, rev: 1, ...over }
}

function head(over: Partial<ChannelHead> = {}, ch: Partial<Channel> = {}): ChannelHead {
  return {
    channel: channel(ch),
    ephemeral: [],
    typing_now: [],
    _actions: {
      post: ['card', 'typing'],
      edit_own: true,
      delete_own: true,
      remove: true,
      lifecycle: ['close', 'delete'],
      typing: false,
      transitions: [],
    },
    ...over,
  }
}

function msg(seq: number, over: Partial<Message> = {}): Message {
  return {
    seq,
    rev: seq,
    type: 'card',
    author: { user: 'ann' },
    at: '2026-03-01T10:00:00Z',
    props: { body: `message ${seq}` },
    _actions: ['edit', 'delete', 'remove'],
    ...over,
  }
}

function page(messages: Message[]): MessagePage {
  return { channel: channel(), messages, has_more: false, next_after_rev: 1 }
}

function oneRoom(): ChannelList {
  return {
    anchor: 'FEAT-1',
    types: [
      { type: 'room', label: 'Room', cardinality: 'one', create: false, channels: [channel()] },
    ],
  }
}

function problem(status: number, type: string, title = 'refused') {
  return new ApiError(title, {
    kind: 'http',
    status,
    problem: { type: `https://rela.dev/errors/${type}`, title, status },
    original: { response: { headers: {} } },
  })
}

// --- Helpers -----------------------------------------------------------------

let wrapper: VueWrapper | undefined

async function mountPanel(entityType = 'feature', world?: string) {
  wrapper = mount(ChannelsPanel, {
    props: { entityType, entityId: 'FEAT-1', world },
    attachTo: document.body,
  })
  await flushPromises()
  await flushPromises()
  return wrapper
}

function w(): VueWrapper {
  if (!wrapper) throw new Error('not mounted')
  return wrapper
}

function messageEl(seq: number) {
  return w().find(`.rl-channel-message[data-seq="${seq}"]`)
}

function buttonIn(el: { findAll: VueWrapper['findAll'] }, label: string) {
  // A button with a pending label holds both; the first is the resting one.
  const shown = (x: ReturnType<VueWrapper['find']>) => {
    const resting = x.find('.rl-button__label')
    return (resting.exists() ? resting.text() : x.text()).trim()
  }
  const b = el.findAll('button').find((x) => shown(x) === label)
  if (!b) throw new Error(`no button "${label}"`)
  return b
}

function composerForm() {
  return w().find('form.rl-channel-composer')
}

async function typeBody(text: string) {
  await composerForm().find('textarea').setValue(text)
}

async function submitComposer() {
  const editing = composerForm().attributes('aria-label') === 'Edit message'
  await buttonIn(composerForm(), editing ? 'Save' : 'Send').trigger('click')
  await flushPromises()
}

async function liveFrame(label: 'channel' | 'entity-channels') {
  const sub = [...subs].reverse().find((s) => s.label === label && s.path() !== null)
  if (!sub) throw new Error(`no live ${label} subscription`)
  sub.onChange()
  await flushPromises()
  await flushPromises()
}

function toastTitles(): string[] {
  return useToasts().toasts.value.map((t) => t.title ?? '')
}

beforeEach(() => {
  registerEntityPlurals(
    new Map([
      ['feature', 'features'],
      ['person', 'people'],
    ])
  )
  subs.length = 0
  for (const fn of Object.values(api)) fn.mockReset()
  confirm.mockReset()
  push.mockReset()
  api.getChannelSchema.mockResolvedValue({ channel_types: [room] })
  api.listChannels.mockResolvedValue(oneRoom())
  api.getChannelHead.mockResolvedValue(head())
  api.getMessages.mockResolvedValue(page([msg(1)]))
  useSchemaStore().channelPresentations = new Map([
    [
      'room',
      {
        messages: {
          card: {
            fields: [
              { property: 'body' },
              { property: 'owner' },
              { property: 'note', hidden: true },
            ],
          },
        },
      },
    ],
  ])
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  vi.useRealTimers()
  document.body.innerHTML = ''
})

// --- Tests -------------------------------------------------------------------

describe('ChannelsPanel', () => {
  it('renders nothing and lists nothing when no channel type attaches', async () => {
    await mountPanel('bug')
    expect(w().find('.channels-panel').exists()).toBe(false)
    expect(api.listChannels).not.toHaveBeenCalled()
  })

  it('opens the only channel directly, without the list', async () => {
    await mountPanel()
    expect(w().find('.rl-entity-channels').exists()).toBe(false)
    expect(messageEl(1).text()).toContain('message 1')
    expect(api.getChannelHead).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'feature', address: 'FEAT-1', channelType: 'room' })
    )
  })

  it('sends no world on channel reads or live paths, even on a world-bound page (R14)', async () => {
    // The server refuses a non-default ?world= on every channel route; the
    // address names the face.
    await mountPanel('feature', 'editorial')
    for (const call of [
      api.listChannels.mock.calls[0][0],
      api.getChannelHead.mock.calls[0][0],
      api.getMessages.mock.calls[0][0],
    ]) {
      expect(call).not.toHaveProperty('world')
    }
    const paths = subs.map((s) => s.path()).filter((p): p is string => p !== null)
    expect(paths.length).toBeGreaterThan(0)
    for (const p of paths) expect(p).not.toContain('world=')
  })

  it('drops the draft and the selection when the page moves to another entity', async () => {
    await mountPanel()
    await typeBody('half written')
    await w().setProps({ entityId: 'FEAT-2' })
    await flushPromises()
    await flushPromises()
    expect(api.getChannelHead).toHaveBeenLastCalledWith(
      expect.objectContaining({ address: 'FEAT-2' })
    )
    expect((composerForm().find('textarea').element as HTMLTextAreaElement).value).toBe('')
  })

  it('keeps the list live in single mode, so a new channel brings the list back', async () => {
    const thread = { ...room, name: 'thread', label: 'Threads', cardinality: 'many' as const }
    api.getChannelSchema.mockResolvedValue({ channel_types: [room, thread] })
    await mountPanel()
    expect(w().find('.rl-entity-channels').exists()).toBe(false)
    const listSub = [...subs].reverse().find((s) => s.label === 'entity-channels')!
    expect(listSub.path()).not.toBeNull()

    api.listChannels.mockResolvedValue({
      anchor: 'FEAT-1',
      types: [
        ...oneRoom().types,
        {
          type: 'thread',
          label: 'Threads',
          cardinality: 'many',
          create: false,
          channels: [channel({ type: 'thread', name: 't1', title: 'Launch' })],
        },
      ],
    })
    await liveFrame('entity-channels')
    expect(w().find('.rl-entity-channels').text()).toContain('Launch')
    // The channel that opened directly stays open.
    expect(messageEl(1).text()).toContain('message 1')
  })

  it('lists several channels and opens the chosen one, dropping the list subscription', async () => {
    api.listChannels.mockResolvedValue({
      anchor: 'FEAT-1',
      types: [
        {
          type: 'thread',
          label: 'Threads',
          cardinality: 'many',
          create: true,
          channels: [channel({ type: 'thread', name: 't1', title: 'Launch' })],
        },
      ],
    })
    api.getChannelSchema.mockResolvedValue({
      channel_types: [room, { ...room, name: 'thread', label: 'Threads', cardinality: 'many' }],
    })
    await mountPanel()
    const listSub = subs.find((s) => s.label === 'entity-channels')!
    expect(listSub.path()).not.toBeNull()
    expect(w().find('.rl-channel').exists()).toBe(false)

    await buttonIn(w(), 'Launch').trigger('click')
    await flushPromises()
    expect(api.getChannelHead).toHaveBeenCalledWith(
      expect.objectContaining({ channelType: 'thread', name: 't1' })
    )
    expect(listSub.path()).toBeNull()
  })

  it('creates a channel with a title and opens it', async () => {
    api.listChannels.mockResolvedValue({
      anchor: 'FEAT-1',
      types: [
        { type: 'thread', label: 'Threads', cardinality: 'many', create: true, channels: [] },
      ],
    })
    api.getChannelSchema.mockResolvedValue({
      channel_types: [
        {
          ...room,
          name: 'thread',
          label: 'Threads',
          cardinality: 'many',
          create_label: 'Start thread',
        },
      ],
    })
    api.createChannel.mockResolvedValue(channel({ type: 'thread', name: 'new-1' }))
    await mountPanel()
    await buttonIn(w(), 'Start thread').trigger('click')
    await flushPromises()
    const field = document.querySelector<HTMLInputElement>(
      '[data-testid="channel-text-dialog-field"] input, input[data-testid="channel-text-dialog-field"]'
    )!
    field.value = 'Kickoff'
    field.dispatchEvent(new Event('input'))
    document.querySelector<HTMLButtonElement>('[data-testid="channel-text-dialog-submit"]')!.click()
    await flushPromises()
    expect(api.createChannel).toHaveBeenCalledWith(
      { type: 'feature', address: 'FEAT-1' },
      'thread',
      'Kickoff'
    )
    expect(api.getChannelHead).toHaveBeenLastCalledWith(
      expect.objectContaining({ channelType: 'thread', name: 'new-1' })
    )
  })

  it('posts, then clears the composer', async () => {
    api.postMessage.mockResolvedValue({
      kind: 'message',
      message: msg(2, { props: { body: 'hello' } }),
    })
    await mountPanel()
    await typeBody('hello')
    await submitComposer()
    expect(api.postMessage).toHaveBeenCalledWith(
      expect.anything(),
      'card',
      { body: 'hello' },
      undefined
    )
    expect((composerForm().find('textarea').element as HTMLTextAreaElement).value).toBe('')
    expect(messageEl(2).text()).toContain('hello')
  })

  it('shows no composer to a user without a post role', async () => {
    api.getChannelHead.mockResolvedValue(
      head({
        _actions: {
          post: [],
          edit_own: false,
          delete_own: false,
          remove: false,
          lifecycle: [],
          typing: false,
          transitions: [],
        },
      })
    )
    await mountPanel()
    expect(messageEl(1).exists()).toBe(true)
    expect(composerForm().exists()).toBe(false)
  })

  it('edits with every prop the composer did not show, leaving a restricted ref to the server (AC 11)', async () => {
    api.getMessages.mockResolvedValue(
      page([msg(1, { props: { body: 'old', note: 'hidden value', owner: { restricted: true } } })])
    )
    api.editMessage.mockResolvedValue(
      msg(1, { rev: 2, props: { body: 'new', note: 'hidden value' } })
    )
    await mountPanel()
    await buttonIn(messageEl(1), 'Edit').trigger('click')
    await flushPromises()
    expect(composerForm().attributes('aria-label')).toBe('Edit message')
    await typeBody('new')
    await submitComposer()
    expect(api.editMessage).toHaveBeenCalledWith(expect.anything(), 1, {
      body: 'new',
      note: 'hidden value',
    })
    expect(composerForm().attributes('aria-label')).toBe('New message')
  })

  it('saves an edit from the props held when it started, after a reload dropped the message', async () => {
    api.getMessages.mockResolvedValue(
      page([msg(1, { props: { body: 'old', note: 'hidden value' } })])
    )
    api.editMessage.mockResolvedValue(
      msg(1, { rev: 2, props: { body: 'new', note: 'hidden value' } })
    )
    await mountPanel()
    await buttonIn(messageEl(1), 'Edit').trigger('click')
    await flushPromises()

    // A busy channel: the follow loop hits its page cap and reloads the
    // newest page, which no longer holds message 1.
    api.getMessages.mockImplementation(async (_t, q?: { afterRev?: number }) =>
      q?.afterRev !== undefined
        ? { ...page([]), has_more: true }
        : page([msg(600, { props: { body: 'newest' } })])
    )
    await liveFrame('channel')
    expect(messageEl(1).exists()).toBe(false)

    await typeBody('new')
    await submitComposer()
    expect(api.editMessage).toHaveBeenCalledWith(expect.anything(), 1, {
      body: 'new',
      note: 'hidden value',
    })
  })

  describe('errors map by problem type (AC 14)', () => {
    const cases: {
      type: string
      status: number
      inForm: boolean
      then: 'none' | 'refetch' | 'list' | 'schema'
    }[] = [
      { type: 'invalid_message', status: 400, inForm: true, then: 'none' },
      { type: 'invalid_ref', status: 422, inForm: true, then: 'none' },
      { type: 'too_large', status: 413, inForm: true, then: 'none' },
      { type: 'channel_not_open', status: 409, inForm: false, then: 'refetch' },
      { type: 'channel_post_denied', status: 403, inForm: false, then: 'refetch' },
      { type: 'message_type_gone', status: 409, inForm: false, then: 'schema' },
      { type: 'channel_not_found', status: 404, inForm: false, then: 'list' },
      { type: 'rate_limited', status: 429, inForm: false, then: 'none' },
    ]
    for (const tc of cases) {
      it(`${tc.type}: ${tc.inForm ? 'form' : 'toast'}, then ${tc.then}`, async () => {
        await mountPanel()
        const heads = api.getChannelHead.mock.calls.length
        const lists = api.listChannels.mock.calls.length
        const schemas = api.getChannelSchema.mock.calls.length
        api.postMessage.mockRejectedValue(problem(tc.status, tc.type, `refused as ${tc.type}`))
        await typeBody('hi')
        await submitComposer()

        if (tc.inForm) {
          expect(composerForm().text()).toContain(`refused as ${tc.type}`)
          expect(toastTitles()).toEqual([])
        } else {
          expect(toastTitles().length).toBe(1)
        }
        expect(api.getChannelHead.mock.calls.length > heads).toBe(tc.then === 'refetch')
        expect(api.getChannelSchema.mock.calls.length > schemas).toBe(tc.then === 'schema')
        expect(api.listChannels.mock.calls.length > lists).toBe(tc.then === 'list')
        // A refused write never clears what the panel shows.
        if (tc.then !== 'list') expect(messageEl(1).exists()).toBe(true)
        // The draft stays for the user to fix or resend.
        if (composerForm().exists()) {
          expect((composerForm().find('textarea').element as HTMLTextAreaElement).value).toBe('hi')
        }
      })
    }
  })

  describe('create errors map by problem type (AC 14)', () => {
    const cases: { type: string; status: number; inForm: boolean; schema: boolean }[] = [
      { type: 'invalid_message', status: 400, inForm: true, schema: false },
      { type: 'channel_create_denied', status: 403, inForm: false, schema: false },
      { type: 'config_changed', status: 409, inForm: false, schema: true },
      { type: 'channel_type_conflict', status: 409, inForm: false, schema: true },
    ]
    for (const tc of cases) {
      it(`${tc.type}: ${tc.inForm ? 'form' : 'toast'}${tc.schema ? ', reloads the descriptor' : ''}`, async () => {
        api.listChannels.mockResolvedValue({
          anchor: 'FEAT-1',
          types: [
            { type: 'thread', label: 'Threads', cardinality: 'many', create: true, channels: [] },
          ],
        })
        api.getChannelSchema.mockResolvedValue({
          channel_types: [{ ...room, name: 'thread', label: 'Threads', cardinality: 'many' }],
        })
        api.createChannel.mockRejectedValue(problem(tc.status, tc.type, `refused as ${tc.type}`))
        await mountPanel()
        const schemas = api.getChannelSchema.mock.calls.length
        await buttonIn(w(), 'New channel').trigger('click')
        await flushPromises()
        document
          .querySelector<HTMLButtonElement>('[data-testid="channel-text-dialog-submit"]')!
          .click()
        await flushPromises()

        const dialog = document.querySelector('[data-testid="channel-text-dialog-submit"]')
        expect(dialog !== null).toBe(tc.inForm)
        expect(toastTitles().length).toBe(tc.inForm ? 0 : 1)
        expect(api.getChannelSchema.mock.calls.length > schemas).toBe(tc.schema)
      })
    }
  })

  it('shows an ephemeral signal after the 202 and hides it at expires_at (AC 15)', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
    vi.setSystemTime(new Date('2026-03-01T10:00:00Z'))
    await mountPanel()
    api.postMessage.mockResolvedValue({
      kind: 'ephemeral',
      accepted: { id: 'e1', expires_at: '2026-03-01T10:00:05Z' },
    })
    api.getChannelHead.mockResolvedValue(
      head({
        ephemeral: [
          {
            id: 'e1',
            type: 'typing',
            author: { user: 'ann' },
            at: '2026-03-01T10:00:00Z',
            expires_at: '2026-03-01T10:00:05Z',
            props: { note: 'is typing' },
          },
        ],
      })
    )
    // Switch the composer to the ephemeral type.
    await buttonIn(composerForm(), 'Typing').trigger('click')
    await composerForm().find('input').setValue('is typing')
    await submitComposer()
    expect(w().text()).toContain('is typing')

    vi.advanceTimersByTime(5_000)
    await flushPromises()
    expect(w().find('.rl-channel').text()).not.toContain('is typing')
  })

  it('shows the closed notice and drops the composer when the channel closes while open (AC 16)', async () => {
    await mountPanel()
    expect(composerForm().exists()).toBe(true)
    api.getChannelHead.mockResolvedValue(
      head(
        {
          _actions: {
            post: [],
            edit_own: false,
            delete_own: false,
            remove: false,
            lifecycle: [],
            typing: false,
            transitions: [],
          },
        },
        { state: 'closed' }
      )
    )
    api.getMessages.mockResolvedValue({ ...page([msg(1)]), channel: channel({ state: 'closed' }) })
    await liveFrame('channel')
    expect(w().text()).toContain('This channel is closed.')
    expect(composerForm().exists()).toBe(false)
  })

  it('shows the archived notice when the channel is archived while open (AC 16)', async () => {
    await mountPanel()
    api.getChannelHead.mockResolvedValue(head({}, { state: 'archived' }))
    await liveFrame('channel')
    expect(w().text()).toContain('This channel is archived.')
  })

  it('returns to the list when the channel is deleted while open (AC 16)', async () => {
    await mountPanel()
    const lists = api.listChannels.mock.calls.length
    api.getChannelHead.mockRejectedValue(problem(404, 'channel_not_found'))
    api.getMessages.mockRejectedValue(problem(404, 'channel_not_found'))
    api.listChannels.mockResolvedValue({ anchor: 'FEAT-1', types: [] })
    await liveFrame('channel')
    expect(api.listChannels.mock.calls.length).toBeGreaterThan(lists)
    expect(w().find('.channels-panel').exists()).toBe(false)
  })

  it('drops the channel on a refused refetch (reader lost access)', async () => {
    await mountPanel()
    api.getChannelHead.mockRejectedValue(problem(403, 'channel_read_denied'))
    api.getMessages.mockRejectedValue(problem(403, 'channel_read_denied'))
    api.listChannels.mockResolvedValue({ anchor: 'FEAT-1', types: [] })
    await liveFrame('channel')
    expect(messageEl(1).exists()).toBe(false)
  })

  it('keeps the channel on a transient refetch failure', async () => {
    await mountPanel()
    api.getChannelHead.mockRejectedValue(
      new ApiError('boom', { kind: 'http', status: 503, original: null })
    )
    api.getMessages.mockRejectedValue(
      new ApiError('boom', { kind: 'http', status: 503, original: null })
    )
    await liveFrame('channel')
    expect(messageEl(1).text()).toContain('message 1')
  })

  it('releases a pending action (AC 18)', async () => {
    api.getMessages.mockResolvedValue(
      page([
        msg(1, {
          action: { name: 'accept', status: 'pending', at: '2026-03-01T10:00:00Z' },
          _actions: ['release'],
        }),
      ])
    )
    let finish!: (m: Message) => void
    api.releaseMessageAction.mockReturnValue(new Promise((r) => (finish = r)))
    await mountPanel()
    await buttonIn(messageEl(1), 'Release').trigger('click')
    expect(api.releaseMessageAction).toHaveBeenCalledWith(expect.anything(), 1)
    expect(messageEl(1).text()).toContain('Releasing…')
    finish(msg(1, { rev: 2, _actions: [] }))
    await flushPromises()
    expect(messageEl(1).text()).not.toContain('Releasing…')
  })

  it('deletes a message after a danger confirmation, with no reason', async () => {
    confirm.mockResolvedValue(true)
    api.removeMessage.mockResolvedValue(
      msg(1, { rev: 2, removed: { at: '2026-03-01T11:00:00Z', kind: 'author' } })
    )
    await mountPanel()
    await buttonIn(messageEl(1), 'Delete').trigger('click')
    await flushPromises()
    expect(confirm).toHaveBeenCalledWith(expect.objectContaining({ danger: true }))
    expect(api.removeMessage).toHaveBeenCalledWith(expect.anything(), 1, undefined)
  })

  it('does not delete when the confirmation is cancelled', async () => {
    confirm.mockResolvedValue(false)
    await mountPanel()
    await buttonIn(messageEl(1), 'Delete').trigger('click')
    await flushPromises()
    expect(api.removeMessage).not.toHaveBeenCalled()
  })

  it('removes a message with the reason the dialog requires', async () => {
    api.removeMessage.mockResolvedValue(
      msg(1, { rev: 2, removed: { at: '2026-03-01T11:00:00Z', kind: 'moderator', reason: 'spam' } })
    )
    await mountPanel()
    await buttonIn(messageEl(1), 'Remove…').trigger('click')
    await flushPromises()
    const submit = document.querySelector<HTMLButtonElement>(
      '[data-testid="channel-text-dialog-submit"]'
    )!
    submit.click()
    await flushPromises()
    expect(api.removeMessage).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('Required.')

    const field = document.querySelector<HTMLTextAreaElement>(
      'textarea[data-testid="channel-text-dialog-field"], [data-testid="channel-text-dialog-field"] textarea'
    )!
    field.value = 'spam'
    field.dispatchEvent(new Event('input'))
    submit.click()
    await flushPromises()
    expect(api.removeMessage).toHaveBeenCalledWith(expect.anything(), 1, 'spam')
    expect(document.querySelector('[data-testid="channel-text-dialog-submit"]')).toBeNull()
  })

  it('deletes the channel after a danger confirmation and leaves it', async () => {
    confirm.mockResolvedValue(true)
    api.runLifecycle.mockResolvedValue(null)
    await mountPanel()
    const lists = api.listChannels.mock.calls.length
    api.listChannels.mockResolvedValue({ anchor: 'FEAT-1', types: [] })
    await buttonIn(w().find('.rl-channel__lifecycle'), 'Delete').trigger('click')
    await flushPromises()
    expect(confirm).toHaveBeenCalledWith(expect.objectContaining({ danger: true }))
    expect(api.runLifecycle).toHaveBeenCalledWith(expect.anything(), 'delete')
    expect(api.listChannels.mock.calls.length).toBeGreaterThan(lists)
    expect(w().find('.rl-channel').exists()).toBe(false)
  })

  it('closes the channel without a confirmation and reads the list again', async () => {
    api.runLifecycle.mockResolvedValue(channel({ state: 'closed' }))
    await mountPanel()
    const lists = api.listChannels.mock.calls.length
    await buttonIn(w().find('.rl-channel__lifecycle'), 'Close').trigger('click')
    await flushPromises()
    expect(confirm).not.toHaveBeenCalled()
    expect(api.runLifecycle).toHaveBeenCalledWith(expect.anything(), 'close')
    expect(api.listChannels.mock.calls.length).toBeGreaterThan(lists)
  })

  it('navigates in the app from an entity link in a message', async () => {
    api.getMessages.mockResolvedValue(
      page([msg(1, { props: { body: 'x', owner: { id: 'P-1', type: 'person', title: 'Pat' } } })])
    )
    await mountPanel()
    const chip = messageEl(1).find('a[href="/entity/person/P-1"]')
    expect(chip.exists()).toBe(true)
    await chip.trigger('click', { button: 0 })
    expect(push).toHaveBeenCalledWith({ path: '/entity/person/P-1', query: {} })
  })

  describe('sessions', () => {
    const sessionRoom: ChannelTypeSchema = {
      ...room,
      message_types: [
        ...room.message_types,
        {
          name: 'contained',
          label: 'Contained',
          persist: 'log',
          reply: false,
          actions: [],
          properties: [],
        },
      ],
      session: {
        label: 'Breach response',
        initial: 'detected',
        states: [
          { name: 'detected', label: 'Detected', status: 'working' },
          { name: 'contained', label: 'Contained', final: true, status: 'completed' },
        ],
      },
    }
    const detected = {
      state: 'detected',
      status: 'working' as const,
      generation: 1,
      entered_at: '2026-03-01T10:00:00Z',
      transitions: [{ message_type: 'contained', to: 'contained' }],
    }

    function sessionHead(): ChannelHead {
      const h = head({}, { session: detected })
      return {
        ...h,
        _actions: { ...h._actions, post: ['card', 'contained'], transitions: ['contained'] },
      }
    }

    it('shows the session stepper, notices and transition types first on a session channel', async () => {
      api.getChannelSchema.mockResolvedValue({ channel_types: [sessionRoom] })
      api.getChannelHead.mockResolvedValue(sessionHead())
      api.getMessages.mockResolvedValue(
        page([
          msg(1),
          msg(2, {
            type: 'session',
            author: { user: 'system', system: true },
            triggered_by: 'session:room',
            props: { from: 'detected', to: 'detected' },
            _actions: [],
          }),
        ])
      )
      await mountPanel()

      const stepper = w().find('.rl-channel-session')
      expect(stepper.exists()).toBe(true)
      expect(stepper.find('[aria-current="step"]').text()).toContain('Detected')
      expect(messageEl(2).text()).toContain('Session entered Detected again')
      expect(messageEl(2).findAll('button')).toHaveLength(0)
      const options = composerForm().findAll('[role="radio"]')
      expect(options.map((o) => o.find('span').text())).toEqual(['Contained', 'Card'])
      expect(options[1]!.attributes('aria-checked')).toBe('true')
    })

    it('keeps the stepper and transition views when a refetch brings the same session', async () => {
      api.getChannelSchema.mockResolvedValue({ channel_types: [sessionRoom] })
      api.getChannelHead.mockImplementation(() => Promise.resolve(sessionHead()))
      api.getMessages.mockResolvedValue({
        ...page([msg(1)]),
        channel: channel({ session: detected }),
      })
      await mountPanel()
      const stepper = () => w().findComponent(RlChannelSession)
      const composer = () => w().findComponent(RlChannelComposer)
      const before = {
        session: stepper().props('session'),
        transitions: composer().props('transitions'),
      }
      const heads = api.getChannelHead.mock.calls.length

      api.postMessage.mockRejectedValue(problem(409, 'channel_not_open', 'closed'))
      await typeBody('hi')
      await submitComposer()
      expect(api.getChannelHead.mock.calls.length).toBeGreaterThan(heads)
      expect(stepper().props('session')).toBe(before.session)
      expect(composer().props('transitions')).toBe(before.transitions)

      api.getMessages.mockResolvedValue({
        ...page([msg(1)]),
        channel: channel({ session: { ...detected, generation: 2 } }),
      })
      api.getChannelHead.mockImplementation(() => {
        const h = sessionHead()
        return Promise.resolve({
          ...h,
          channel: { ...h.channel, session: { ...detected, generation: 2 } },
        })
      })
      await submitComposer()
      expect(stepper().props('session')).not.toBe(before.session)
    })

    it('shows a loading stepper and no transition marks for a state the descriptor lacks', async () => {
      api.getChannelSchema
        .mockResolvedValueOnce({ channel_types: [sessionRoom] })
        .mockReturnValue(new Promise(() => {}))
      const h = sessionHead()
      api.getChannelHead.mockResolvedValue({
        ...h,
        channel: { ...h.channel, session: { ...detected, state: 'escalated' } },
      })
      await mountPanel()
      expect(stepper().props('session')).toMatchObject({ loading: true })
      expect(w().find('.rl-channel-session').text()).not.toContain('Stranded')
      expect(composerForm().text()).not.toContain('moves the session')
      function stepper() {
        return w().findComponent(RlChannelSession)
      }
    })

    it('shows the state as stranded once the reloaded descriptor still lacks it', async () => {
      api.getChannelSchema.mockResolvedValue({ channel_types: [sessionRoom] })
      const h = sessionHead()
      api.getChannelHead.mockResolvedValue({
        ...h,
        channel: { ...h.channel, session: { ...detected, state: 'escalated' } },
      })
      await mountPanel()
      await flushPromises()
      expect(api.getChannelSchema.mock.calls.length).toBeGreaterThan(1)
      expect(w().findComponent(RlChannelSession).props('session')).toMatchObject({
        stranded: true,
      })
    })

    it('shows no stepper on a channel type without a session', async () => {
      await mountPanel()
      expect(w().find('.rl-channel').exists()).toBe(true)
      expect(w().find('.rl-channel-session').exists()).toBe(false)
    })

    it('offers no edit or delete on the causing message', async () => {
      api.getChannelSchema.mockResolvedValue({ channel_types: [sessionRoom] })
      api.getChannelHead.mockResolvedValue(sessionHead())
      api.getMessages.mockResolvedValue(page([msg(1, { _actions: ['remove'] })]))
      await mountPanel()
      const labels = messageEl(1)
        .findAll('button')
        .map((b) => b.text())
      expect(labels.some((l) => l.includes('Edit'))).toBe(false)
      expect(labels.some((l) => l.includes('Delete'))).toBe(false)
      expect(labels.some((l) => l.includes('Remove'))).toBe(true)
    })
  })
})
