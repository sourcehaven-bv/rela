import { describe, it, expect, vi, beforeEach } from 'vitest'
import { nextTick, ref } from 'vue'
import { ApiError } from '@/api/errors'
import type { ChannelHead, ChannelTypeSchema, Message } from '@/types/channels'
import { useSessionDecisions, type DecisionChannel, type DecisionDeps } from './useSessionDecisions'

const logType = (name: string, label: string, required = false) => ({
  name,
  label,
  persist: 'log' as const,
  reply: !required,
  actions: [],
  properties: required ? [{ name: 'why', label: 'Why', type: 'string', required: true }] : [],
})

const ct = {
  name: 'run',
  message_types: [
    logType('approve', 'Approve'),
    logType('accept', 'Accept'),
    logType('revise', 'Revise', true),
  ],
  session: {
    label: 'Run',
    initial: 'review',
    states: [
      { name: 'review', label: 'Review', status: 'input_required' },
      { name: 'working', label: 'Working', status: 'working' },
      { name: 'accepted', label: 'Accepted', final: true, status: 'completed' },
    ],
  },
} as unknown as ChannelTypeSchema

function head(generation: number, state = 'review'): ChannelHead {
  return {
    channel: {
      state: 'open',
      session: {
        state,
        status: 'input_required',
        generation,
        entered_at: '2026-03-01T10:00:00Z',
        transitions: [
          { message_type: 'approve', to: 'working' },
          { message_type: 'accept', to: 'accepted' },
          { message_type: 'revise', to: 'working' },
        ],
      },
    },
    _actions: {
      post: ['approve', 'accept', 'revise'],
      transitions: state === 'review' ? ['approve', 'accept', 'revise'] : [],
    },
  } as unknown as ChannelHead
}

function problem(type: string) {
  return new ApiError(type, {
    kind: 'http',
    status: 409,
    problem: { type: `https://rela.dev/errors/${type}`, title: type, status: 409 },
    original: { response: { headers: {} } },
  })
}

/** A promise the test settles by hand. */
function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function setup() {
  const h = ref<ChannelHead | undefined>(head(4))
  const actionsGeneration = ref<number | undefined>(4)
  const key = ref('a')
  const ch = {
    head: h,
    messages: ref<Message[]>([]),
    actionsGeneration,
    refreshHead: vi.fn(async () => {}),
    refetch: vi.fn(async () => {}),
  } as unknown as DecisionChannel
  const deps = {
    channelType: () => ct,
    channelKey: () => key.value,
    ch,
    messageType: (n: string) => ct.message_types.find((m) => m.name === n),
    post: vi.fn(async () => {}),
    setPending: vi.fn(),
    submitting: ref(false),
    composer: {
      editing: vi.fn(() => false),
      discardEdit: vi.fn(),
      endReply: vi.fn(),
      select: vi.fn(() => true),
      showError: vi.fn(),
    },
    ui: {
      confirm: vi.fn(async () => true),
      toast: vi.fn(),
      handle: vi.fn((): string | undefined => undefined),
      focusRow: vi.fn(() => false),
      focusHeader: vi.fn(),
    },
  } satisfies DecisionDeps
  /** Moves the session; the held actions follow, as a head fetch does. */
  const move = (g: number, state = 'review') => {
    h.value = head(g, state)
    actionsGeneration.value = g
  }
  return { deps, ch, key, move, d: useSessionDecisions(deps) }
}

const STALE = 'The session moved on. Choose again.'

describe('useSessionDecisions', () => {
  let s: ReturnType<typeof setup>
  beforeEach(() => {
    s = setup()
  })

  it('offers the one-click and form decisions; accept asks first', () => {
    expect(s.d.decisions.value.map((v) => [v.key, !!v.opensForm, !!v.confirm])).toEqual([
      ['approve', false, false],
      ['accept', false, true],
      ['revise', true, false],
    ])
  })

  it('posts with the generation read at the click, though the same state came back during the dialog', async () => {
    const ok = deferred<boolean>()
    s.deps.ui.confirm.mockReturnValue(ok.promise)
    const done = s.d.onDecide('accept', 'header')
    s.move(6)
    ok.resolve(true)
    await done
    expect(s.deps.post).toHaveBeenCalledWith('accept', {}, undefined, 4)
  })

  it('shows the pending key only while the post is in flight, not during the dialog', async () => {
    const ok = deferred<boolean>()
    const sent = deferred<void>()
    s.deps.ui.confirm.mockReturnValue(ok.promise)
    s.deps.post.mockReturnValue(sent.promise)
    const done = s.d.onDecide('accept', 'header')
    await nextTick()
    expect(s.d.deciding.value).toBe(true)
    expect(s.deps.setPending).not.toHaveBeenCalled()
    ok.resolve(true)
    await vi.waitFor(() => expect(s.deps.setPending).toHaveBeenCalledWith('decision:accept', true))
    sent.resolve()
    await done
    expect(s.deps.setPending).toHaveBeenLastCalledWith('decision:accept', false)
    expect(s.d.deciding.value).toBe(false)
  })

  it('ignores a second click while one is in progress', async () => {
    const sent = deferred<void>()
    s.deps.post.mockReturnValue(sent.promise)
    const first = s.d.onDecide('approve', 'header')
    await s.d.onDecide('approve', 'header')
    sent.resolve()
    await first
    expect(s.deps.post).toHaveBeenCalledTimes(1)
  })

  it('a channel switch releases the lock and drops the old click’s after-steps', async () => {
    const sent = deferred<void>()
    s.deps.post.mockReturnValueOnce(sent.promise)
    const old = s.d.onDecide('approve', 'row')
    await vi.waitFor(() => expect(s.deps.post).toHaveBeenCalled())
    s.key.value = 'b'
    s.d.resetDecisions()
    expect(s.d.deciding.value).toBe(false)
    expect(s.deps.setPending).toHaveBeenLastCalledWith('decision:approve', false)
    // The new channel can decide at once.
    await s.d.onDecide('approve', 'header')
    expect(s.deps.post).toHaveBeenCalledTimes(2)
    expect(s.deps.ch.refreshHead).toHaveBeenCalledTimes(1)
    sent.resolve()
    await old
    expect(s.deps.ch.refreshHead).toHaveBeenCalledTimes(1)
    expect(s.deps.ui.focusRow).not.toHaveBeenCalled()
    expect(s.deps.ui.focusHeader).toHaveBeenCalledTimes(1)
  })

  it('a refusal that lands after a channel switch writes nothing into the new composer', async () => {
    const sent = deferred<void>()
    s.deps.post.mockReturnValueOnce(sent.promise)
    s.deps.ui.handle.mockReturnValue('Too long')
    const old = s.d.onDecide('approve', 'header')
    await vi.waitFor(() => expect(s.deps.post).toHaveBeenCalled())
    s.key.value = 'b'
    s.d.resetDecisions()
    sent.reject(problem('invalid_message'))
    await old
    expect(s.deps.ui.handle).not.toHaveBeenCalled()
    expect(s.deps.composer.select).not.toHaveBeenCalled()
    expect(s.deps.composer.showError).not.toHaveBeenCalled()
  })

  it.each([
    { type: 'session_moved', handled: true },
    { type: 'session_wrong_state', handled: false },
  ])('$type: answered once', async ({ type, handled }) => {
    s.deps.post.mockRejectedValue(problem(type))
    await s.d.onDecide('approve', 'header')
    expect(s.deps.ui.handle).toHaveBeenCalledTimes(handled ? 1 : 0)
    expect(s.deps.ui.toast.mock.calls.flat().includes(STALE)).toBe(!handled)
    expect(s.deps.ch.refetch).toHaveBeenCalledTimes(handled ? 0 : 1)
    expect(s.deps.ch.refreshHead).not.toHaveBeenCalled()
  })

  it('a form refusal opens the composer with the type and shows the message there', async () => {
    s.deps.post.mockRejectedValue(problem('invalid_message'))
    s.deps.ui.handle.mockReturnValue('Why is required')
    await s.d.onDecide('approve', 'header')
    expect(s.deps.composer.endReply).not.toHaveBeenCalled()
    expect(s.deps.composer.select).toHaveBeenCalledWith('approve')
    expect(s.deps.composer.showError).toHaveBeenCalledWith('Why is required')
  })

  it('a form decision asks before it discards an edit, ends a reply the type cannot carry, then selects it', async () => {
    s.deps.composer.editing.mockReturnValue(true)
    await s.d.onDecide('revise', 'header')
    expect(s.deps.ui.confirm).toHaveBeenCalledTimes(1)
    expect(s.deps.composer.discardEdit).toHaveBeenCalled()
    expect(s.deps.composer.endReply).toHaveBeenCalled()
    expect(s.deps.composer.select).toHaveBeenCalledWith('revise')
    expect(s.deps.post).not.toHaveBeenCalled()
    expect(s.deps.setPending).not.toHaveBeenCalled()
  })

  it('a decision the session has left is not sent', async () => {
    const ok = deferred<boolean>()
    s.deps.ui.confirm.mockReturnValue(ok.promise)
    const done = s.d.onDecide('accept', 'header')
    s.move(5, 'working')
    ok.resolve(true)
    await done
    expect(s.deps.post).not.toHaveBeenCalled()
    expect(s.deps.ui.toast).toHaveBeenCalledWith(STALE)
  })

  it('after a post, focuses the deciding row when it is shown, else the header', async () => {
    s.deps.ch.messages.value = [
      { seq: 12, rev: 12, type: 'card', props: {} },
      {
        seq: 13,
        rev: 13,
        type: 'session',
        triggered_by: 'session:run',
        props: { from: 'working', to: 'review', generation: 4, cause: 12 },
      },
    ] as unknown as Message[]
    s.deps.ui.focusRow.mockReturnValue(true)
    await s.d.onDecide('approve', 'row')
    expect(s.deps.ui.focusRow).toHaveBeenCalledWith(12)
    expect(s.deps.ui.focusHeader).not.toHaveBeenCalled()
  })
})
