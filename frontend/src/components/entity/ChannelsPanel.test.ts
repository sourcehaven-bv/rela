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
import { defineComponent, h as render } from 'vue'
import { provideMessages } from 'rela-components/composables/useMessages'
import { localeMessages } from 'rela-components/composables/localeMessages'
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
  // The open channel marks itself read; these tests do not look at it.
  markChannelRead: vi.fn<(...a: unknown[]) => Promise<never>>(),
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

/** Mounts the panel with the component library's text in language. */
async function mountPanelIn(language: string) {
  const Host = defineComponent({
    setup() {
      provideMessages(localeMessages(() => language))
      return () => render(ChannelsPanel, { entityType: 'feature', entityId: 'FEAT-1' })
    },
  })
  wrapper = mount(Host, { attachTo: document.body })
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
  api.markChannelRead.mockReturnValue(new Promise<never>(() => {}))
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
      undefined,
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

    // TKT-FPXTQJ AC 11: a type with entries to several states says it may
    // move the session to one of them.
    for (const tc of [
      { language: 'en', hint: 'Sending this may move the session to Contained or Escalated.' },
      { language: 'nl', hint: 'Dit bericht kan de sessie op Contained of Escalated zetten.' },
    ]) {
      it(`says a branching type may move the session (${tc.language})`, async () => {
        const branchRoom: ChannelTypeSchema = {
          ...sessionRoom,
          session: {
            ...sessionRoom.session!,
            states: [
              ...sessionRoom.session!.states,
              { name: 'escalated', label: 'Escalated', final: true, status: 'completed' },
            ],
          },
        }
        const branching = {
          ...detected,
          transitions: [
            { message_type: 'contained', to: 'contained', guarded: true },
            { message_type: 'contained', to: 'escalated' },
          ],
        }
        api.getChannelSchema.mockResolvedValue({ channel_types: [branchRoom] })
        const sh = sessionHead()
        api.getChannelHead.mockResolvedValue({
          ...sh,
          channel: { ...sh.channel, session: branching },
        })
        await mountPanelIn(tc.language)
        await composerForm().findAll('[role="radio"]')[0]!.trigger('click')
        await flushPromises()
        expect(composerForm().text()).toContain(tc.hint)
      })
    }

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

    // TKT-21G6JS AC 10: a working session says so in the strip's one live
    // region, and nowhere else.
    for (const tc of [
      { language: 'en', text: 'Working…' },
      { language: 'nl', text: 'Bezig…' },
    ]) {
      it(`activity line while working (${tc.language})`, async () => {
        api.getChannelSchema.mockResolvedValue({ channel_types: [sessionRoom] })
        api.getChannelHead.mockResolvedValue(sessionHead())
        await mountPanelIn(tc.language)
        const strip = w().find('.rl-channel-session')
        // The decision buttons in the strip's slot carry their own pending
        // region; the strip itself holds one.
        const regions = strip
          .findAll('[role="status"], [aria-live]')
          .filter((r) => !r.element.closest('.rl-channel-session__decisions'))
        expect(regions).toHaveLength(1)
        expect(regions[0]!.text()).toContain(tc.text)
        expect(regions[0]!.find('.rl-busy').text()).toContain('Detected')
      })
    }

    it('no activity line when paused', async () => {
      api.getChannelSchema.mockResolvedValue({ channel_types: [sessionRoom] })
      const h = sessionHead()
      api.getChannelHead.mockResolvedValue({ ...h, channel: { ...h.channel, state: 'closed' } })
      await mountPanel()
      const strip = w().find('.rl-channel-session')
      expect(strip.exists()).toBe(true)
      expect(strip.text()).not.toContain('Working…')
      expect(strip.find('.rl-busy').exists()).toBe(false)
    })

    it('no activity line for input_required', async () => {
      const waitingRoom: ChannelTypeSchema = {
        ...sessionRoom,
        session: {
          ...sessionRoom.session!,
          states: [
            { name: 'detected', label: 'Detected', status: 'input_required' },
            ...sessionRoom.session!.states.slice(1),
          ],
        },
      }
      api.getChannelSchema.mockResolvedValue({ channel_types: [waitingRoom] })
      const h = sessionHead()
      api.getChannelHead.mockResolvedValue({
        ...h,
        channel: { ...h.channel, session: { ...detected, status: 'input_required' as const } },
      })
      await mountPanel()
      const strip = w().find('.rl-channel-session')
      expect(strip.text()).toContain('Input required')
      expect(strip.text()).not.toContain('Working…')
      expect(strip.find('.rl-busy').exists()).toBe(false)
    })

    // A stranded or loading session keeps the stored status working, and
    // still shows no activity line: only the normal status does.
    it.each([
      {
        name: 'final',
        session: {
          ...detected,
          state: 'contained',
          status: 'completed' as const,
          final: true,
          transitions: [],
        },
        settle: false,
        want: { final: true },
      },
      {
        name: 'stranded',
        session: { ...detected, state: 'escalated' },
        settle: true,
        want: { stranded: true },
      },
      {
        name: 'loading',
        session: { ...detected, state: 'escalated' },
        settle: false,
        loading: true,
        want: { loading: true },
      },
    ])('no activity line when $name', async ({ session, settle, loading, want }) => {
      if (loading) {
        api.getChannelSchema
          .mockResolvedValueOnce({ channel_types: [sessionRoom] })
          .mockReturnValue(new Promise(() => {}))
      } else {
        api.getChannelSchema.mockResolvedValue({ channel_types: [sessionRoom] })
      }
      const h = sessionHead()
      api.getChannelHead.mockResolvedValue({ ...h, channel: { ...h.channel, session } })
      await mountPanel()
      if (settle) await flushPromises()
      expect(w().findComponent(RlChannelSession).props('session')).toMatchObject(want)
      const strip = w().find('.rl-channel-session')
      expect(strip.exists()).toBe(true)
      expect(strip.text()).not.toContain('Working…')
      expect(strip.find('.rl-busy').exists()).toBe(false)
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

// Session decisions as buttons (TKT-T9G1AE AC 3 to 11): the header and the
// deciding message show one button per decision the caller may make now.
describe('ChannelsPanel session decisions', () => {
  const logType = (
    name: string,
    label: string,
    properties: ChannelTypeSchema['message_types'][number]['properties'] = [],
    reply = true
  ) => ({ name, label, persist: 'log' as const, reply, actions: [], properties })
  const run: ChannelTypeSchema = {
    ...room,
    message_types: [
      room.message_types[0]!,
      logType('approve', 'Approve plan', [{ name: 'note', label: 'Note', type: 'string' }]),
      logType(
        'revise',
        'Revise plan',
        [{ name: 'feedback', label: 'Feedback', type: 'markdown', required: true }],
        false
      ),
      logType('abandon', 'Abandon'),
      logType('accept', 'Accept'),
    ],
    session: {
      label: 'Run',
      initial: 'plan-review',
      states: [
        { name: 'plan-review', label: 'Plan review', status: 'input_required' },
        { name: 'working', label: 'Working', status: 'working' },
        { name: 'accepted', label: 'Accepted', final: true, status: 'completed' },
        { name: 'abandoned', label: 'Abandoned', final: true, status: 'cancelled' },
      ],
    },
  }
  const inReview = {
    state: 'plan-review',
    status: 'input_required' as const,
    generation: 4,
    entered_at: '2026-03-01T10:00:00Z',
    transitions: [
      { message_type: 'approve', to: 'working' },
      { message_type: 'revise', to: 'working' },
      { message_type: 'abandon', to: 'abandoned' },
      { message_type: 'accept', to: 'accepted' },
    ],
  }
  const working = { ...inReview, state: 'working', status: 'working' as const, generation: 5 }
  const POST = ['card', 'approve', 'revise', 'abandon', 'accept']

  function runHead(
    session: Channel['session'] = inReview,
    transitions = ['approve', 'revise', 'abandon', 'accept'],
    ch: Partial<Channel> = {}
  ): ChannelHead {
    const h = head({}, { last_seq: 13, rev: 13, session, ...ch })
    return { ...h, _actions: { ...h._actions, post: POST, transitions } }
  }
  const notice = msg(13, {
    type: 'session',
    author: { user: 'system', system: true },
    triggered_by: 'session:room',
    props: { from: 'working', to: 'plan-review', generation: 4, cause: 12 },
    _actions: [],
  })

  /** The head every fetch answers; a test replaces it to move the session. */
  let current: ChannelHead

  beforeEach(() => {
    current = runHead()
    api.getChannelSchema.mockResolvedValue({ channel_types: [run] })
    api.getChannelHead.mockImplementation(() => Promise.resolve(current))
    api.getMessages.mockResolvedValue({
      ...page([msg(12, { props: { body: 'the plan' } }), notice]),
      channel: channel({ last_seq: 13, rev: 13, session: inReview }),
      next_after_rev: 13,
    })
  })

  const header = () => w().find('.rl-channel-session')
  const decision = (scope: ReturnType<VueWrapper['find']>, key: string) =>
    scope.find(`[data-decision="${key}"]`)
  const posted = (over: Partial<Message> = {}): PostResult => ({
    kind: 'message',
    message: msg(14, { type: 'approve', props: {}, reply_to: 12, _actions: [], ...over }),
  })
  async function click(el: ReturnType<VueWrapper['find']>) {
    await el.trigger('click')
    await flushPromises()
    await flushPromises()
  }

  it('posts approve from the header button and the buttons go once the session moves', async () => {
    await mountPanel()
    api.postMessage.mockImplementation(async () => {
      current = runHead(working, [])
      return posted()
    })
    await click(decision(header(), 'approve'))
    expect(api.postMessage).toHaveBeenCalledTimes(1)
    expect(api.postMessage).toHaveBeenCalledWith(expect.anything(), 'approve', {}, 12, 4)
    expect(w().findAll('[data-decision]')).toHaveLength(0)
    expect(document.activeElement).toBe(header().element)
  })

  it('shows the same buttons on the deciding message and on no other', async () => {
    await mountPanel()
    expect(decision(messageEl(12), 'approve').exists()).toBe(true)
    expect(decision(messageEl(13), 'approve').exists()).toBe(false)
    expect(
      header()
        .findAll('[data-decision]')
        .map((b) => b.attributes('data-decision'))
    ).toEqual(['approve', 'revise', 'abandon', 'accept'])
  })

  it('focus returns to the row after a row decision', async () => {
    await mountPanel()
    api.postMessage.mockImplementation(async () => {
      current = runHead(working, [])
      return posted()
    })
    await click(decision(messageEl(12), 'approve'))
    expect(document.activeElement).toBe(messageEl(12).element)
  })

  it('the row buttons go when the cause is removed, the header stays', async () => {
    api.getMessages.mockResolvedValue({
      ...page([
        msg(12, {
          removed: { kind: 'moderator', by: { user: 'bob' }, at: '2026-03-01T10:00:00Z' },
        }),
        notice,
      ]),
      channel: channel({ last_seq: 13, rev: 13, session: inReview }),
      next_after_rev: 13,
    })
    await mountPanel()
    expect(messageEl(12).find('[data-decision]').exists()).toBe(false)
    expect(decision(header(), 'approve').exists()).toBe(true)
  })

  it('shows no decision to a caller without the transitions', async () => {
    current = runHead(inReview, [])
    await mountPanel()
    expect(w().findAll('[data-decision]')).toHaveLength(0)
  })

  // TKT-FPXTQJ AC 13: the form's Send carries the generation read at the
  // click; switching the composer to another type drops it.
  it('the Send of a form decision carries its generation; another type drops it', async () => {
    await mountPanel()
    api.postMessage.mockResolvedValue(posted({ type: 'revise' }))
    await click(decision(header(), 'revise'))
    await composerForm().find('textarea').setValue('redo the plan')
    await submitComposer()
    expect(api.postMessage).toHaveBeenLastCalledWith(
      expect.anything(),
      'revise',
      { feedback: 'redo the plan' },
      undefined,
      4
    )

    await click(decision(header(), 'revise'))
    const card = composerForm()
      .findAll('[role="radio"]')
      .find((o) => o.text().includes('Card'))!
    await card.trigger('click')
    await flushPromises()
    const revise = composerForm()
      .findAll('[role="radio"]')
      .find((o) => o.text().includes('Revise plan'))!
    await revise.trigger('click')
    await flushPromises()
    await composerForm().find('textarea').setValue('again')
    await submitComposer()
    expect(api.postMessage).toHaveBeenLastCalledWith(
      expect.anything(),
      'revise',
      { feedback: 'again' },
      undefined,
      undefined
    )
  })

  it('Revise plan opens the composer and sends nothing', async () => {
    await mountPanel()
    await click(decision(header(), 'revise'))
    expect(api.postMessage).not.toHaveBeenCalled()
    const composer = w().findComponent(RlChannelComposer)
    const checked = composerForm().find('[role="radio"][aria-checked="true"]')
    expect(checked.text()).toContain('Revise plan')
    expect(composer.find('textarea').element).toBe(document.activeElement)
  })

  it('asks before discarding an edit; cancel keeps the edit and sends nothing', async () => {
    await mountPanel()
    await buttonIn(messageEl(12), 'Edit').trigger('click')
    await flushPromises()
    expect(composerForm().attributes('aria-label')).toBe('Edit message')
    confirm.mockResolvedValue(false)
    await click(decision(header(), 'revise'))
    expect(confirm).toHaveBeenCalledTimes(1)
    expect(composerForm().attributes('aria-label')).toBe('Edit message')
    expect(api.postMessage).not.toHaveBeenCalled()

    confirm.mockResolvedValue(true)
    await click(decision(header(), 'revise'))
    expect(composerForm().attributes('aria-label')).not.toBe('Edit message')
    expect(composerForm().find('[role="radio"][aria-checked="true"]').text()).toContain(
      'Revise plan'
    )
  })

  it('a double click on the header and the row posts once', async () => {
    await mountPanel()
    let answer!: (r: PostResult) => void
    api.postMessage.mockReturnValue(new Promise<PostResult>((r) => (answer = r)))
    await decision(header(), 'approve').trigger('click')
    await decision(header(), 'approve').trigger('click')
    await decision(messageEl(12), 'approve').trigger('click')
    await flushPromises()
    expect(api.postMessage).toHaveBeenCalledTimes(1)
    const pending = decision(header(), 'approve')
    expect(pending.attributes('aria-disabled')).toBe('true')
    expect(pending.attributes('disabled')).toBeUndefined()
    expect(decision(header(), 'revise').attributes('disabled')).toBeDefined()
    // The row shows the same decision, so it shows the same pending state.
    expect(decision(messageEl(12), 'approve').attributes('aria-disabled')).toBe('true')
    expect(decision(messageEl(12), 'revise').attributes('disabled')).toBeDefined()
    current = runHead(working, [])
    answer(posted())
    await flushPromises()
    await flushPromises()
    expect(w().findAll('[data-decision]')).toHaveLength(0)
  })

  it('the composer cannot send while a decision is in flight and shows no Sending label', async () => {
    await mountPanel()
    let answer!: (r: PostResult) => void
    api.postMessage.mockReturnValueOnce(new Promise<PostResult>((r) => (answer = r)))
    await click(decision(header(), 'approve'))
    await typeBody('hello')
    await composerForm().trigger('submit')
    await flushPromises()
    expect(api.postMessage).toHaveBeenCalledTimes(1)
    const send = buttonIn(composerForm(), 'Send')
    expect(send.attributes('disabled')).toBeDefined()
    expect(send.attributes('aria-busy')).toBeUndefined()
    current = runHead(working, [])
    answer(posted())
    await flushPromises()
  })

  it('two clicks on Abandon confirm once and post once', async () => {
    await mountPanel()
    let ok!: (v: boolean) => void
    confirm.mockReturnValue(new Promise<boolean>((r) => (ok = r)))
    api.postMessage.mockImplementation(async () => {
      current = runHead({ ...inReview, state: 'abandoned', final: true, generation: 5 }, [])
      return posted({ type: 'abandon' })
    })
    await decision(header(), 'abandon').trigger('click')
    await decision(header(), 'abandon').trigger('click')
    await flushPromises()
    expect(confirm).toHaveBeenCalledTimes(1)
    expect(confirm.mock.calls[0]![0]).toMatchObject({ danger: true })
    ok(true)
    await flushPromises()
    await flushPromises()
    expect(api.postMessage).toHaveBeenCalledTimes(1)
    expect(api.postMessage).toHaveBeenCalledWith(expect.anything(), 'abandon', {}, 12, 4)
  })

  it('abandon asks first; cancel sends nothing and releases the lock', async () => {
    await mountPanel()
    confirm.mockResolvedValue(false)
    await click(decision(header(), 'abandon'))
    expect(api.postMessage).not.toHaveBeenCalled()
    expect(decision(header(), 'approve').attributes('disabled')).toBeUndefined()
  })

  it('accept asks without the danger tone and names the final state', async () => {
    await mountPanel()
    confirm.mockResolvedValue(true)
    api.postMessage.mockResolvedValue(posted({ type: 'accept' }))
    await click(decision(header(), 'accept'))
    expect(confirm.mock.calls[0]![0]).toMatchObject({
      danger: false,
      message: 'This ends the session in Accepted. You cannot undo it.',
    })
    expect(api.postMessage).toHaveBeenCalledTimes(1)
  })

  it('the session moved while the dialog was open: nothing is sent', async () => {
    await mountPanel()
    let ok!: (v: boolean) => void
    confirm.mockReturnValue(new Promise<boolean>((r) => (ok = r)))
    await decision(header(), 'accept').trigger('click')
    await flushPromises()
    current = runHead(working, [])
    api.getMessages.mockResolvedValue({
      ...page([]),
      channel: channel({ last_seq: 14, rev: 14, session: working }),
      next_after_rev: 14,
    })
    await liveFrame('channel')
    ok(true)
    await flushPromises()
    expect(api.postMessage).not.toHaveBeenCalled()
    expect(toastTitles()).toContain('The session moved on. Choose again.')
  })

  it('the same state came back at a newer generation while the dialog was open: the post keeps the old one', async () => {
    await mountPanel()
    let ok!: (v: boolean) => void
    confirm.mockReturnValue(new Promise<boolean>((r) => (ok = r)))
    api.postMessage.mockRejectedValue(problem(409, 'session_moved', 'moved'))
    await decision(header(), 'accept').trigger('click')
    await flushPromises()
    // review -> working -> review: the same decisions, two generations on.
    const again = { ...inReview, generation: 6 }
    current = runHead(again)
    api.getMessages.mockResolvedValue({
      ...page([]),
      channel: channel({ last_seq: 15, rev: 15, session: again }),
      next_after_rev: 15,
    })
    await liveFrame('channel')
    expect(decision(header(), 'accept').exists()).toBe(true)
    ok(true)
    await flushPromises()
    expect(api.postMessage).toHaveBeenCalledTimes(1)
    expect(api.postMessage.mock.calls[0]?.[4]).toBe(4)
    expect(toastTitles()).toContain('The session moved on. Choose again.')
  })

  it('a 201 that moves nothing brings the still-valid decisions back', async () => {
    await mountPanel()
    api.postMessage.mockImplementation(async () => {
      current = runHead(inReview, ['revise'])
      return posted()
    })
    await click(decision(header(), 'approve'))
    expect(
      header()
        .findAll('[data-decision]')
        .map((b) => b.attributes('data-decision'))
    ).toEqual(['revise'])
  })

  it('a live frame before the POST answer shows no old decisions', async () => {
    await mountPanel()
    let answer!: (r: PostResult) => void
    api.postMessage.mockReturnValue(new Promise<PostResult>((r) => (answer = r)))
    await click(decision(header(), 'approve'))
    // The page moves the session; the head that follows is held open.
    let headAnswer!: (h: ChannelHead) => void
    api.getChannelHead.mockReturnValueOnce(new Promise<ChannelHead>((r) => (headAnswer = r)))
    api.getMessages.mockResolvedValue({
      ...page([]),
      channel: channel({ last_seq: 14, rev: 14, session: working }),
      next_after_rev: 14,
    })
    await liveFrame('channel')
    expect(w().findAll('[data-decision]')).toHaveLength(0)
    current = runHead(working, [])
    headAnswer(current)
    answer(posted())
    await flushPromises()
    await flushPromises()
    expect(w().findAll('[data-decision]')).toHaveLength(0)
  })

  it('a failed head refresh after the 201 releases the lock with a toast', async () => {
    await mountPanel()
    api.postMessage.mockImplementation(async () => {
      api.getChannelHead.mockRejectedValueOnce(problem(500, 'internal', 'server error'))
      return posted()
    })
    await click(decision(header(), 'approve'))
    expect(toastTitles()).toContain('server error')
    expect(decision(header(), 'revise').attributes('disabled')).toBeUndefined()
  })

  it('the channel closes during the request: no decisions after', async () => {
    await mountPanel()
    api.postMessage.mockImplementation(async () => {
      current = runHead(inReview, ['approve', 'revise'], { state: 'closed' })
      api.getMessages.mockResolvedValue({
        ...page([]),
        channel: channel({ last_seq: 13, rev: 14, state: 'closed', session: inReview }),
        next_after_rev: 14,
      })
      throw problem(409, 'channel_not_open', 'The channel is closed.')
    })
    await click(decision(header(), 'approve'))
    expect(toastTitles()).toContain('The channel is closed.')
    expect(w().findAll('[data-decision]')).toHaveLength(0)
  })

  describe('decision refusals map by problem type', () => {
    const cases: { name: string; status: number; type: string; toast?: string; form?: string }[] = [
      {
        name: 'session moved',
        status: 409,
        type: 'session_moved',
        toast: 'The session moved on. Choose again.',
      },
      {
        name: 'wrong state',
        status: 409,
        type: 'session_wrong_state',
        toast: 'The session moved on. Choose again.',
      },
      {
        name: 'output denied',
        status: 403,
        type: 'session_output_denied',
        toast: 'refused as session_output_denied',
      },
      {
        name: 'invalid message',
        status: 400,
        type: 'invalid_message',
        form: 'refused as invalid_message',
      },
      {
        name: 'rate limited',
        status: 429,
        type: 'rate_limited',
        toast: 'Too many messages. Try again shortly.',
      },
    ]
    for (const tc of cases) {
      it(tc.name, async () => {
        await mountPanel()
        const heads = api.getChannelHead.mock.calls.length
        // The refetch finds nothing new, so it reads the head again.
        api.getMessages.mockResolvedValue({
          ...page([]),
          channel: channel({ last_seq: 13, rev: 13, session: inReview }),
          next_after_rev: 13,
        })
        api.postMessage.mockRejectedValue(problem(tc.status, tc.type, `refused as ${tc.type}`))
        await click(decision(header(), 'approve'))
        if (tc.toast) expect(toastTitles()).toContain(tc.toast)
        if (tc.form) {
          expect(composerForm().text()).toContain(tc.form)
          expect(composerForm().find('[role="radio"][aria-checked="true"]').text()).toContain(
            'Approve plan'
          )
        }
        if (tc.type.startsWith('session_') || tc.status === 403) {
          expect(api.getChannelHead.mock.calls.length).toBeGreaterThan(heads)
        }
        expect(decision(header(), 'revise').attributes('disabled')).toBeUndefined()
      })
    }

    it('a network error is a toast', async () => {
      await mountPanel()
      api.postMessage.mockRejectedValue(
        new ApiError('Network down', { kind: 'network', original: null })
      )
      await click(decision(header(), 'approve'))
      expect(toastTitles().length).toBeGreaterThan(0)
      expect(decision(header(), 'revise').attributes('disabled')).toBeUndefined()
    })
  })
})

// TKT-03FQPD: a question's options as reply buttons, in a PoC-shaped
// session. ask (seq 12) moved the session to asking at generation 5.
describe('ChannelsPanel choices', () => {
  const ask = {
    name: 'ask',
    label: 'Ask',
    persist: 'log' as const,
    reply: true,
    actions: [],
    properties: [
      { name: 'note', label: 'Note', type: 'string' as const },
      { name: 'options', label: 'Options', type: 'string' as const, list: true },
    ],
    choices: { from: 'options', answer: 'answer', into: 'note', other: true },
  }
  const answer = {
    name: 'answer',
    label: 'Answer',
    persist: 'log' as const,
    reply: true,
    actions: [],
    properties: [{ name: 'note', label: 'Note', type: 'string' as const }],
  }
  const run: ChannelTypeSchema = {
    ...room,
    message_types: [room.message_types[0]!, ask, answer],
    session: {
      label: 'Run',
      initial: 'working',
      states: [
        { name: 'working', label: 'Working', status: 'working' },
        { name: 'asking', label: 'Asking', status: 'input_required' },
      ],
    },
  }
  const asking = {
    state: 'asking',
    status: 'input_required' as const,
    generation: 5,
    entered_at: '2026-03-01T10:00:00Z',
    transitions: [{ message_type: 'answer', to: 'working' }],
  }
  const runHead = (): ChannelHead => {
    const h = head({}, { last_seq: 13, rev: 13, session: asking })
    return { ...h, _actions: { ...h._actions, post: ['card', 'answer'], transitions: ['answer'] } }
  }
  const question = msg(12, {
    type: 'ask',
    author: { user: 'robot' },
    props: { note: 'Which one?', options: ['A', 'B'] },
    _actions: [],
  })
  const notice = msg(13, {
    type: 'session',
    author: { user: 'system', system: true },
    triggered_by: 'session:room',
    props: { from: 'working', to: 'asking', generation: 5, cause: 12 },
    _actions: [],
  })

  beforeEach(() => {
    api.getChannelSchema.mockResolvedValue({ channel_types: [run] })
    api.getChannelHead.mockImplementation(() => Promise.resolve(runHead()))
    api.getMessages.mockResolvedValue({
      ...page([question, notice]),
      channel: channel({ last_seq: 13, rev: 13, session: asking }),
      next_after_rev: 13,
    })
    api.postMessage.mockResolvedValue({
      kind: 'message',
      message: msg(14, { type: 'answer', props: { note: 'B' }, reply_to: 12, _actions: [] }),
    })
  })

  const header = () => w().find('.rl-channel-session')
  async function click(el: ReturnType<VueWrapper['find']>) {
    await el.trigger('click')
    await flushPromises()
    await flushPromises()
  }

  it('shows the options on the question, and the answer decision only in the header', async () => {
    await mountPanel()
    const row = messageEl(12)
    const group = row.find('[role="group"][aria-label="Choices"]')
    expect(group.findAll('[data-decision]').map((b) => b.attributes('data-decision'))).toEqual([
      'choice:12:0',
      'choice:12:1',
      'choice:12:other',
    ])
    expect(row.find('[data-decision="answer"]').exists()).toBe(false)
    expect(header().findAll('[data-decision="answer"]')).toHaveLength(1)
    // While the buttons show, the options are not listed a second time.
    expect(row.find('.rl-channel-message__list').exists()).toBe(false)
  })

  it('a click on an option posts the answer as a reply at the head generation', async () => {
    await mountPanel()
    await click(messageEl(12).find('[data-decision="choice:12:1"]'))
    expect(api.postMessage).toHaveBeenCalledTimes(1)
    expect(api.postMessage).toHaveBeenCalledWith(expect.anything(), 'answer', { note: 'B' }, 12, 5)
  })

  it('Other opens the composer as a reply to the question, and the Send carries the generation', async () => {
    await mountPanel()
    await click(messageEl(12).find('[data-decision="choice:12:other"]'))
    expect(api.postMessage).not.toHaveBeenCalled()
    const checked = composerForm().find('[role="radio"][aria-checked="true"]')
    expect(checked.text()).toContain('Answer')
    expect(composerForm().text()).toContain('robot')
    await composerForm().find('input[type="text"], textarea').setValue('Neither')
    await submitComposer()
    expect(api.postMessage).toHaveBeenCalledWith(
      expect.anything(),
      'answer',
      { note: 'Neither' },
      12,
      5
    )
  })

  it('a caller who may not post the answer sees the options as a list', async () => {
    api.getChannelHead.mockImplementation(() => {
      const h = runHead()
      return Promise.resolve({ ...h, _actions: { ...h._actions, post: ['card'], transitions: [] } })
    })
    await mountPanel()
    const row = messageEl(12)
    expect(row.find('[data-decision]').exists()).toBe(false)
    expect(row.findAll('.rl-channel-message__list li').map((li) => li.text())).toEqual(['A', 'B'])
  })
})

// TKT-03DMXH: message types whose presentation sets `display`. The schema
// is based on the agent-runner PoC, plus a hidden `handoff` that moves the
// session into a state where the owner has decisions.
describe('ChannelsPanel quiet types', () => {
  const logType = (
    name: string,
    label: string,
    properties: ChannelTypeSchema['message_types'][number]['properties'] = [
      { name: 'body', label: 'Body', type: 'markdown' },
    ],
    extra: Partial<ChannelTypeSchema['message_types'][number]> = {}
  ) => ({ name, label, persist: 'log' as const, reply: true, actions: [], properties, ...extra })
  const run: ChannelTypeSchema = {
    ...room,
    message_types: [
      logType('text', 'Message'),
      logType('claim', 'Claim', [{ name: 'worktree', label: 'Worktree', type: 'string' }]),
      logType('resume', 'Resume', [{ name: 'worktree', label: 'Worktree', type: 'string' }]),
      logType('progress', 'Progress', undefined, {
        actions: [{ name: 'record', label: 'Record' }],
      }),
      logType('handoff', 'Handoff'),
      logType('approve', 'Approve'),
      logType(
        'ask',
        'Question',
        [
          { name: 'note', label: 'Note', type: 'string' },
          { name: 'options', label: 'Options', type: 'string', list: true },
        ],
        { choices: { from: 'options', answer: 'answer', into: 'note', other: false } }
      ),
      logType('answer', 'Answer', [{ name: 'note', label: 'Note', type: 'string' }]),
      { ...logType('heartbeat', 'Heartbeat'), persist: 'ephemeral' as const, reply: false },
      { ...logType('typing', 'Typing'), persist: 'ephemeral' as const, reply: false },
      { ...logType('ping', 'Ping'), persist: 'ephemeral' as const, reply: false },
    ],
    session: {
      label: 'Run',
      initial: 'working',
      states: [
        { name: 'working', label: 'Working', status: 'working' },
        { name: 'review', label: 'Review', status: 'input_required' },
        { name: 'asking', label: 'Asking', status: 'input_required' },
      ],
    },
  }
  const state = (
    name: string,
    generation: number,
    transitions: { message_type: string; to: string }[]
  ) => ({
    state: name,
    status: name === 'working' ? ('working' as const) : ('input_required' as const),
    generation,
    entered_at: '2026-03-01T10:00:00Z',
    transitions,
  })
  const working = state('working', 6, [])
  const review = state('review', 5, [{ message_type: 'approve', to: 'working' }])
  const asking = state('asking', 5, [{ message_type: 'answer', to: 'working' }])
  const POST = ['text', 'approve', 'answer']

  function runHead(
    session: Channel['session'],
    transitions: string[] = [],
    over: Partial<ChannelHead> = {}
  ) {
    const h = head(over, { last_seq: 13, rev: 13, session })
    return { ...h, _actions: { ...h._actions, post: POST, transitions } }
  }
  const m = (seq: number, type: string, over: Partial<Message> = {}) =>
    msg(seq, {
      type,
      author: { user: 'robot' },
      props: { body: `${type} ${seq}` },
      _actions: [],
      ...over,
    })
  const notice = (seq: number, to: string, cause: number) =>
    msg(seq, {
      type: 'session',
      author: { user: 'system', system: true },
      triggered_by: 'session:room',
      props: { from: 'working', to, generation: 5, cause },
      _actions: [],
    })
  function serve(
    messages: Message[],
    session: Channel['session'] = working,
    over: Partial<MessagePage> = {}
  ) {
    api.getMessages.mockResolvedValue({
      ...page(messages),
      channel: channel({ last_seq: 13, rev: 13, session }),
      next_after_rev: 13,
      ...over,
    })
  }

  let current: ChannelHead

  beforeEach(() => {
    current = runHead(working)
    api.getChannelSchema.mockResolvedValue({ channel_types: [run] })
    api.getChannelHead.mockImplementation(() => Promise.resolve(current))
    useSchemaStore().channelPresentations = new Map([
      [
        'room',
        {
          messages: {
            claim: { display: 'hidden' },
            resume: { display: 'hidden' },
            handoff: { display: 'hidden' },
            ask: { display: 'hidden' },
            heartbeat: { display: 'collapsed' },
            ping: { display: 'hidden' },
            progress: { display: 'collapsed' },
          },
        },
      ],
    ])
  })

  async function click(el: ReturnType<VueWrapper['find']>) {
    await el.trigger('click')
    await flushPromises()
    await flushPromises()
  }
  const announcement = () =>
    w().find('.rl-channel > p.rl-visually-hidden[aria-live="polite"]').text()
  const folds = () => w().findAll('button.rl-channel-fold')
  const quote = (seq: number) => messageEl(seq).find('button.rl-channel-message__quote')

  /** Serves `messages` on the next live frame, as an after_rev page. */
  async function liveAdds(messages: Message[], session: Channel['session'] = working) {
    api.getMessages.mockImplementation(async (_t, q?: { afterRev?: number }) => ({
      ...page(q?.afterRev !== undefined ? messages : []),
      channel: channel({ last_seq: 20, rev: 20, session }),
      next_after_rev: 20,
    }))
    await liveFrame('channel')
  }

  it('hidden decision cause keeps its buttons', async () => {
    current = runHead(review, ['approve'])
    serve([m(10, 'text'), m(11, 'claim'), m(12, 'handoff'), notice(13, 'review', 12)], review)
    await mountPanel()
    expect(messageEl(11).exists()).toBe(false)
    expect(messageEl(12).find('[data-decision="approve"]').exists()).toBe(true)
  })

  it('decided hidden cause keeps its row and focus', async () => {
    current = runHead(review, ['approve'])
    serve([m(10, 'text'), m(12, 'handoff'), notice(13, 'review', 12)], review)
    await mountPanel()
    api.postMessage.mockImplementation(async () => {
      current = runHead(working)
      return { kind: 'message', message: m(14, 'approve', { reply_to: 12 }) }
    })
    await click(messageEl(12).find('[data-decision="approve"]'))
    expect(api.postMessage).toHaveBeenCalledTimes(1)
    expect(messageEl(12).exists()).toBe(true)
    expect(messageEl(12).find('[data-decision]').exists()).toBe(false)
    expect(document.activeElement).toBe(messageEl(12).element)
  })

  it('hidden question keeps its choices', async () => {
    current = runHead(asking, ['answer'])
    serve(
      [m(12, 'ask', { props: { note: 'Which?', options: ['A', 'B'] } }), notice(13, 'asking', 12)],
      asking
    )
    await mountPanel()
    expect(messageEl(12).findAll('[data-decision^="choice:12:"]')).toHaveLength(2)
  })

  it('answered hidden question keeps its row and focus', async () => {
    current = runHead(asking, ['answer'])
    serve(
      [m(12, 'ask', { props: { note: 'Which?', options: ['A', 'B'] } }), notice(13, 'asking', 12)],
      asking
    )
    await mountPanel()
    api.postMessage.mockImplementation(async () => {
      current = runHead(working)
      return { kind: 'message', message: m(14, 'answer', { props: { note: 'B' }, reply_to: 12 }) }
    })
    await click(messageEl(12).find('[data-decision="choice:12:1"]'))
    expect(api.postMessage).toHaveBeenCalledWith(expect.anything(), 'answer', { note: 'B' }, 12, 5)
    expect(messageEl(12).exists()).toBe(true)
    expect(messageEl(12).find('[data-decision]').exists()).toBe(false)
    expect(document.activeElement).toBe(messageEl(12).element)
  })

  it('reply quote reveals a hidden target', async () => {
    serve([m(11, 'claim'), m(12, 'text', { reply_to: 11 })])
    await mountPanel()
    expect(messageEl(11).exists()).toBe(false)
    await click(quote(12))
    expect(messageEl(11).exists()).toBe(true)
    expect(document.activeElement).toBe(messageEl(11).element)
  })

  it('reply quote to a removed message between hidden ones', async () => {
    serve([
      m(10, 'text'),
      m(11, 'claim'),
      m(12, 'claim', { removed: { at: '2026-03-01T11:00:00Z', kind: 'author' } }),
      m(13, 'claim'),
      m(14, 'text', { reply_to: 12 }),
    ])
    await mountPanel()
    expect(messageEl(12).exists()).toBe(true)
    await click(quote(14))
    expect(document.activeElement).toBe(messageEl(12).element)
  })

  it('reply quote expands a collapsed run', async () => {
    serve([m(10, 'progress'), m(11, 'progress'), m(12, 'text', { reply_to: 11 })])
    await mountPanel()
    expect(folds()).toHaveLength(1)
    expect(folds()[0]!.attributes('aria-expanded')).toBe('false')
    await click(quote(12))
    expect(folds()[0]!.attributes('aria-expanded')).toBe('true')
    expect(messageEl(10).exists()).toBe(true)
    expect(document.activeElement).toBe(messageEl(11).element)
  })

  it('reply quote reveals a hidden target on an older page', async () => {
    api.getMessages.mockImplementation(async (_t, q?: { afterRev?: number; before?: number }) =>
      q?.before !== undefined
        ? {
            ...page([m(5, 'claim'), m(6, 'resume')]),
            channel: channel({ last_seq: 13, rev: 13, session: working }),
          }
        : {
            ...page([m(12, 'text', { reply_to: 5 })]),
            channel: channel({ last_seq: 13, rev: 13, session: working }),
            has_more: true,
            next_before: 12,
            next_after_rev: 13,
          }
    )
    await mountPanel()
    await click(quote(12))
    await flushPromises()
    expect(messageEl(5).exists()).toBe(true)
    expect(document.activeElement).toBe(messageEl(5).element)
    expect(announcement()).toBe('')
  })

  it('reply quote to the last message of an expanded run', async () => {
    serve([m(10, 'progress'), m(11, 'progress'), m(12, 'text', { reply_to: 11 })])
    await mountPanel()
    await click(folds()[0]!)
    expect(messageEl(11).exists()).toBe(true)
    await click(quote(12))
    expect(document.activeElement).toBe(messageEl(11).element)
  })

  it('toggle absent without quiet types', async () => {
    useSchemaStore().channelPresentations = new Map()
    serve([m(10, 'claim'), m(11, 'progress')])
    await mountPanel()
    expect(w().text()).not.toContain('Show all messages')
    expect(messageEl(10).exists()).toBe(true)
  })

  it('the toggle shows every message and is not stored', async () => {
    serve([m(10, 'text'), m(11, 'claim'), m(12, 'progress')])
    await mountPanel()
    const toggle = buttonIn(w(), 'Show all messages')
    expect(toggle.attributes('aria-pressed')).toBe('false')
    await click(toggle)
    expect(buttonIn(w(), 'Show all messages').attributes('aria-pressed')).toBe('true')
    expect(messageEl(11).exists()).toBe(true)
    expect(folds()).toHaveLength(0)
    w().unmount()
    await mountPanel()
    expect(buttonIn(w(), 'Show all messages').attributes('aria-pressed')).toBe('false')
    expect(messageEl(11).exists()).toBe(false)
  })

  it('live quiet message not announced', async () => {
    serve([m(10, 'text')])
    await mountPanel()
    await liveAdds([m(11, 'text'), m(12, 'progress')])
    expect(messageEl(11).exists()).toBe(true)
    expect(announcement()).toBe('Message from robot')
  })

  it('kept quiet row is announced', async () => {
    serve([m(10, 'text')])
    await mountPanel()
    await liveAdds([m(11, 'progress', { _actions: ['record'] })])
    expect(messageEl(11).exists()).toBe(true)
    expect(announcement()).toBe('Progress from robot')
  })

  it('hidden question announced once its choices arrive', async () => {
    serve([m(10, 'text')])
    await mountPanel()
    current = runHead(asking, ['answer'])
    await liveAdds(
      [m(11, 'ask', { props: { note: 'Which?', options: ['A', 'B'] } }), notice(12, 'asking', 11)],
      asking
    )
    expect(messageEl(11).findAll('[data-decision^="choice:11:"]')).toHaveLength(2)
    expect(announcement()).toBe('Question from robot')
  })

  it('only quiet additions announce nothing', async () => {
    serve([m(10, 'text')])
    await mountPanel()
    await liveAdds([m(11, 'claim'), m(12, 'progress')])
    expect(folds()).toHaveLength(1)
    expect(announcement()).toBe('')
  })

  it('quiet signal not shown', async () => {
    const signal = (id: string, type: string, note: string) => ({
      id,
      type,
      author: { user: 'robot' },
      at: '2026-03-01T10:00:00Z',
      expires_at: '2099-01-01T00:00:00Z',
      props: { body: note },
    })
    current = runHead(working, [], {
      ephemeral: [
        signal('e1', 'heartbeat', 'tick'),
        signal('e2', 'typing', 'is typing'),
        signal('e3', 'ping', 'pong'),
      ],
    })
    serve([m(10, 'text')])
    await mountPanel()
    const area = w().find('.rl-channel__ephemeral')
    expect(area.text()).toContain('is typing')
    expect(area.text()).not.toContain('tick')
    expect(area.text()).not.toContain('pong')
    await click(buttonIn(w(), 'Show all messages'))
    expect(w().find('.rl-channel__ephemeral').text()).toContain('tick')
    expect(w().find('.rl-channel__ephemeral').text()).toContain('pong')
  })

  it('all hidden shows the hint', async () => {
    serve([m(10, 'claim'), m(11, 'resume')])
    await mountPanel()
    expect(w().text()).toContain('Only hidden messages here. Use Show all messages to see them.')
    expect(w().text()).not.toContain('No messages yet.')
  })

  it('all hidden with older pages', async () => {
    serve([m(10, 'claim'), m(11, 'resume')], working, { has_more: true, next_before: 10 })
    await mountPanel()
    expect(w().text()).toContain(
      'Only hidden messages so far. Load older messages or use Show all messages to see them.'
    )
  })

  it('load older with only hidden is announced', async () => {
    serve([m(10, 'text')], working, { has_more: true, next_before: 10 })
    await mountPanel()
    serve([m(8, 'claim'), m(9, 'resume')])
    await click(buttonIn(w(), 'Load older messages'))
    expect(api.getMessages).toHaveBeenLastCalledWith(expect.anything(), { before: 10 })
    expect(announcement()).toBe('2 older messages are hidden. Use Show all messages to see them.')
  })

  it('focus inside an expanded run survives an append', async () => {
    serve([m(10, 'text'), m(11, 'progress'), m(12, 'progress')])
    await mountPanel()
    await click(folds()[0]!)
    const row = messageEl(11).element as HTMLElement
    row.focus()
    expect(document.activeElement).toBe(row)
    await liveAdds([m(13, 'progress')])
    expect(folds()[0]!.attributes('aria-expanded')).toBe('true')
    expect(messageEl(13).exists()).toBe(true)
    expect(messageEl(11).element).toBe(row)
    expect(document.activeElement).toBe(row)
  })

  it('focus on the tail fold survives an append', async () => {
    serve([m(10, 'text'), m(11, 'progress')])
    await mountPanel()
    const button = folds()[0]!.element as HTMLButtonElement
    button.focus()
    await liveAdds([m(12, 'progress')])
    expect(folds()).toHaveLength(1)
    expect(folds()[0]!.element).toBe(button)
    expect(document.activeElement).toBe(button)
    expect(button.textContent).toContain('2 messages: Progress')
  })
})

describe('ChannelsPanel preset buttons', () => {
  const logType = (
    name: string,
    label: string,
    properties: ChannelTypeSchema['message_types'][number]['properties'] = [],
    extra: Partial<ChannelTypeSchema['message_types'][number]> = {}
  ) => ({ name, label, persist: 'log' as const, reply: true, actions: [], properties, ...extra })
  const run: ChannelTypeSchema = {
    ...room,
    message_types: [
      room.message_types[0]!,
      logType('decide', 'Decide', [
        { name: 'decision', label: 'Decision', type: 'string', required: true },
        { name: 'note', label: 'Note', type: 'string' },
      ]),
      logType(
        'ask',
        'Question',
        [
          { name: 'note', label: 'Note', type: 'string' },
          { name: 'options', label: 'Options', type: 'string', list: true },
        ],
        { choices: { from: 'options', answer: 'answer', into: 'note', other: false } }
      ),
      logType('answer', 'Answer', [{ name: 'note', label: 'Note', type: 'string' }]),
    ],
    session: {
      label: 'Plan',
      initial: 'review',
      states: [
        { name: 'review', label: 'Review', status: 'input_required' },
        { name: 'working', label: 'Working', status: 'working' },
        { name: 'drafting', label: 'Drafting', status: 'working' },
      ],
    },
  }
  const review = {
    state: 'review',
    status: 'input_required' as const,
    generation: 4,
    entered_at: '2026-03-01T10:00:00Z',
    transitions: [
      { message_type: 'answer', to: 'working', entry: 0 },
      {
        message_type: 'decide',
        to: 'working',
        guarded: true,
        entry: 0,
        button: { label: 'Approve', props: { decision: 'approve' } },
      },
      { message_type: 'decide', to: 'drafting', entry: 1 },
    ],
  }
  const runHead = (): ChannelHead => {
    const h = head({}, { last_seq: 13, rev: 13, session: review })
    return {
      ...h,
      _actions: {
        ...h._actions,
        post: ['card', 'decide', 'answer'],
        transitions: ['answer', 'decide'],
        entries: [
          { message_type: 'answer', entry: 0 },
          { message_type: 'decide', entry: 0 },
          { message_type: 'decide', entry: 1 },
        ],
      },
    }
  }
  const question = msg(12, {
    type: 'ask',
    author: { user: 'robot' },
    props: { note: 'Which one?', options: ['A', 'B'] },
    _actions: [],
  })
  const notice = msg(13, {
    type: 'session',
    author: { user: 'system', system: true },
    triggered_by: 'session:room',
    props: { from: 'drafting', to: 'review', generation: 4, cause: 12 },
    _actions: [],
  })

  beforeEach(() => {
    api.getChannelSchema.mockResolvedValue({ channel_types: [run] })
    api.getChannelHead.mockImplementation(() => Promise.resolve(runHead()))
    api.getMessages.mockResolvedValue({
      ...page([question, notice]),
      channel: channel({ last_seq: 13, rev: 13, session: review }),
      next_after_rev: 13,
    })
    api.postMessage.mockResolvedValue({
      kind: 'message',
      message: msg(14, {
        type: 'decide',
        props: { decision: 'approve' },
        reply_to: 12,
        _actions: [],
      }),
    })
    useSchemaStore().channelPresentations = new Map()
  })

  const header = () => w().find('.rl-channel-session')
  const keys = (scope: ReturnType<VueWrapper['find']>) =>
    scope.findAll('[data-decision]').map((b) => b.attributes('data-decision'))

  it('plan review shows Approve and the form button', async () => {
    await mountPanel()
    expect(keys(header())).toEqual(['answer', 'preset:decide:0', 'decide'])
    expect(header().find('[data-decision="preset:decide:0"]').text()).toContain('Approve')
    // The question's row shows its choices and the presets of other types.
    expect(keys(messageEl(12))).toEqual(['choice:12:0', 'choice:12:1', 'preset:decide:0', 'decide'])
  })

  it('Approve posts the preset', async () => {
    await mountPanel()
    await header().find('[data-decision="preset:decide:0"]').trigger('click')
    await flushPromises()
    await flushPromises()
    expect(api.postMessage).toHaveBeenCalledTimes(1)
    expect(api.postMessage).toHaveBeenCalledWith(
      expect.anything(),
      'decide',
      { decision: 'approve' },
      12,
      4
    )
    expect(confirm).not.toHaveBeenCalled()
  })

  it('choice click disables the presets', async () => {
    await mountPanel()
    let answer!: (r: PostResult) => void
    api.postMessage.mockReturnValue(new Promise<PostResult>((r) => (answer = r)))
    await messageEl(12).find('[data-decision="choice:12:0"]').trigger('click')
    await flushPromises()
    expect(header().find('[data-decision="preset:decide:0"]').attributes('disabled')).toBeDefined()
    await header().find('[data-decision="preset:decide:0"]').trigger('click')
    await flushPromises()
    expect(api.postMessage).toHaveBeenCalledTimes(1)
    answer({
      kind: 'message',
      message: msg(14, { type: 'answer', props: { note: 'A' }, reply_to: 12, _actions: [] }),
    })
    await flushPromises()
  })

  it('authored confirm asks before Allow posts', async () => {
    const withAllow: ChannelTypeSchema = {
      ...run,
      message_types: [
        ...run.message_types,
        logType('allow', 'Allow', [], {
          decision_button: { confirm: true, confirm_text: 'This runs the tool on your machine.' },
        }),
      ],
    }
    const allowReview = {
      ...review,
      transitions: [...review.transitions, { message_type: 'allow', to: 'working', entry: 0 }],
    }
    api.getChannelSchema.mockResolvedValue({ channel_types: [withAllow] })
    api.getChannelHead.mockImplementation(() => {
      const h = runHead()
      return Promise.resolve({
        ...h,
        channel: { ...h.channel, session: allowReview },
        _actions: {
          ...h._actions,
          post: [...h._actions.post, 'allow'],
          transitions: [...(h._actions.transitions ?? []), 'allow'],
          entries: [...(h._actions.entries ?? []), { message_type: 'allow', entry: 0 }],
        },
      })
    })
    api.getMessages.mockResolvedValue({
      ...page([question, notice]),
      channel: channel({ last_seq: 13, rev: 13, session: allowReview }),
      next_after_rev: 13,
    })
    confirm.mockResolvedValue(true)
    await mountPanel()
    await header().find('[data-decision="allow"]').trigger('click')
    await flushPromises()
    await flushPromises()
    expect(confirm).toHaveBeenCalledWith(
      expect.objectContaining({
        title: 'Allow',
        message: 'This runs the tool on your machine.',
        danger: false,
      })
    )
    expect(api.postMessage).toHaveBeenCalledWith(expect.anything(), 'allow', {}, 12, 4)
  })

  it('hidden decision type keeps its presets', async () => {
    useSchemaStore().channelPresentations = new Map([
      ['room', { messages: { decide: { display: 'hidden' }, ask: { display: 'hidden' } } }],
    ])
    await mountPanel()
    expect(keys(header())).toContain('preset:decide:0')
    expect(messageEl(12).find('[data-decision="preset:decide:0"]').exists()).toBe(true)
  })
})
