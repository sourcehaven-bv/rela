import { describe, it, expect, vi, beforeEach } from 'vitest'
import { nextTick, ref } from 'vue'
import { ApiError } from '@/api/errors'
import { i18n } from '@/i18n'
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
    hasNewer: ref(false),
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
      reply: vi.fn(),
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

// TKT-FPXTQJ AC 13: a decision that opens the form carries the generation
// read at the click into the composer's Send.
describe('useSessionDecisions form decisions', () => {
  let s: ReturnType<typeof setup>
  beforeEach(() => {
    s = setup()
    s.move(3)
  })
  const send = (type: string) => s.d.sendComposer(type, {}, undefined)
  const lastGeneration = () => (s.deps.post.mock.lastCall as unknown[] | undefined)?.[3]

  it('the Send of the form type carries the generation read at the click', async () => {
    await s.d.onDecide('revise', 'header')
    expect(s.d.formDecision.value).toEqual({ messageType: 'revise', generation: 3 })
    await send('revise')
    expect(s.deps.post).toHaveBeenLastCalledWith('revise', {}, undefined, 3)
    expect(s.d.formDecision.value).toBeUndefined()
  })

  it('a Send of another type carries no generation and ends the form decision', async () => {
    await s.d.onDecide('revise', 'header')
    await send('approve')
    expect(s.deps.post).toHaveBeenLastCalledWith('approve', {}, undefined, undefined)
    expect(s.d.formDecision.value).toBeUndefined()
    await send('revise')
    expect(lastGeneration()).toBeUndefined()
  })

  it('switching the composer to another type ends the form decision without a message', async () => {
    await s.d.onDecide('revise', 'header')
    s.d.onComposerSelect('revise')
    expect(s.d.formDecision.value).toEqual({ messageType: 'revise', generation: 3 })
    s.d.onComposerSelect('approve')
    expect(s.d.formDecision.value).toBeUndefined()
    s.d.onComposerSelect('revise')
    s.move(4)
    await nextTick()
    expect(s.deps.ui.toast).not.toHaveBeenCalled()
    await send('revise')
    expect(lastGeneration()).toBeUndefined()
  })

  it('a plain composer post sends no generation', async () => {
    await send('revise')
    expect(s.deps.post).toHaveBeenLastCalledWith('revise', {}, undefined, undefined)
  })

  it('a move before the Send drops the form decision and says the session moved on', async () => {
    await s.d.onDecide('revise', 'header')
    s.move(4)
    await nextTick()
    expect(s.d.formDecision.value).toBeUndefined()
    expect(s.deps.ui.toast).toHaveBeenCalledWith(STALE)
    await send('revise')
    expect(lastGeneration()).toBeUndefined()
  })

  it('a move during the Send is not reported as stale', async () => {
    await s.d.onDecide('revise', 'header')
    s.deps.submitting.value = true
    s.deps.post.mockImplementationOnce(async () => {
      s.move(4)
      await nextTick()
    })
    await send('revise')
    expect(lastGeneration()).toBe(3)
    expect(s.deps.ui.toast).not.toHaveBeenCalled()
    expect(s.d.formDecision.value).toBeUndefined()
  })

  it('a session_moved refusal on the Send ends the form decision and passes the error on', async () => {
    await s.d.onDecide('revise', 'header')
    const err = problem('session_moved')
    s.deps.post.mockRejectedValueOnce(err)
    await expect(send('revise')).rejects.toBe(err)
    expect(s.d.formDecision.value).toBeUndefined()
  })

  it('another refusal keeps the form decision for the corrected Send', async () => {
    await s.d.onDecide('revise', 'header')
    s.deps.post.mockRejectedValueOnce(problem('invalid_message'))
    await expect(send('revise')).rejects.toBeTruthy()
    expect(s.d.formDecision.value).toEqual({ messageType: 'revise', generation: 3 })
  })

  it('a channel switch ends the form decision', async () => {
    await s.d.onDecide('revise', 'header')
    s.d.resetDecisions()
    expect(s.d.formDecision.value).toBeUndefined()
  })
})

// A question type ask whose options post answer (TKT-03FQPD). With a
// session, ask at seq 10 moved the session to generation 5.
const askType = {
  name: 'ask',
  label: 'Ask',
  persist: 'log' as const,
  reply: true,
  actions: [],
  properties: [
    { name: 'note', label: 'Note', type: 'string' },
    { name: 'options', label: 'Options', type: 'string', list: true },
  ],
  choices: { from: 'options', answer: 'answer', into: 'body', other: true },
}
const answerType = {
  name: 'answer',
  label: 'Answer',
  persist: 'log' as const,
  reply: true,
  actions: [],
  properties: [{ name: 'body', label: 'Body', type: 'string' }],
}

function choiceSetup(withSession: boolean) {
  const cct = {
    name: 'run',
    message_types: [askType, answerType],
    ...(withSession
      ? {
          session: {
            label: 'Run',
            initial: 'working',
            states: [
              { name: 'working', label: 'Working', status: 'working' },
              { name: 'asking', label: 'Asking', status: 'input_required' },
            ],
          },
        }
      : {}),
  } as unknown as ChannelTypeSchema
  const question: Message = {
    seq: 10,
    rev: 10,
    type: 'ask',
    at: '2026-03-01T10:00:00Z',
    author: { user: 'robot' },
    props: { note: 'Which one?', options: ['A', 'B'] },
  }
  const notice: Message = {
    seq: 11,
    rev: 11,
    type: 'session',
    at: '2026-03-01T10:00:00Z',
    author: { user: 'system', system: true },
    triggered_by: 'session:run',
    props: { from: 'working', to: 'asking', generation: 5, cause: 10 },
  }
  const h = ref<ChannelHead | undefined>({
    channel: {
      state: 'open',
      ...(withSession
        ? {
            session: {
              state: 'asking',
              status: 'input_required',
              generation: 5,
              entered_at: '2026-03-01T10:00:00Z',
              transitions: [{ message_type: 'answer', to: 'working' }],
            },
          }
        : {}),
    },
    _actions: { post: ['answer', 'ask'], transitions: withSession ? ['answer'] : [] },
  } as unknown as ChannelHead)
  const messages = ref<Message[]>(withSession ? [question, notice] : [question])
  const ch = {
    head: h,
    messages,
    hasNewer: ref(false),
    actionsGeneration: ref<number | undefined>(withSession ? 5 : undefined),
    refreshHead: vi.fn(async () => {}),
    refetch: vi.fn(async () => {}),
  } as unknown as DecisionChannel
  const deps = {
    channelType: () => cct,
    channelKey: () => 'a',
    ch,
    messageType: (n: string) => cct.message_types.find((m) => m.name === n),
    post: vi.fn(async () => {}),
    setPending: vi.fn(),
    submitting: ref(false),
    composer: {
      editing: vi.fn(() => false),
      discardEdit: vi.fn(),
      endReply: vi.fn(),
      reply: vi.fn(),
      select: vi.fn(() => true),
      showError: vi.fn(),
    },
    ui: {
      confirm: vi.fn(async () => true),
      toast: vi.fn(),
      handle: vi.fn((): string | undefined => undefined),
      focusRow: vi.fn(() => true),
      focusHeader: vi.fn(),
    },
  } satisfies DecisionDeps
  return { deps, messages, question, d: useSessionDecisions(deps) }
}

describe('useSessionDecisions choices', () => {
  it('builds one button per option and Other on the question', () => {
    const s = choiceSetup(true)
    expect(s.d.choices.value.get(10)?.map((v) => [v.key, v.label, !!v.opensForm])).toEqual([
      ['choice:10:0', 'A', false],
      ['choice:10:1', 'B', false],
      ['choice:10:other', 'Other answer…', true],
    ])
  })

  it('the Other label follows a language switch', () => {
    const s = choiceSetup(false)
    const other = () => s.d.choices.value.get(10)?.find((v) => v.opensForm)?.label
    expect(other()).toBe('Other answer…')
    const before = i18n.global.locale.value
    try {
      i18n.global.locale.value = 'nl'
      expect(other()).toBe('Ander antwoord…')
    } finally {
      i18n.global.locale.value = before
    }
  })

  it('a click posts the answer as a reply, with the head generation, and focuses the question', async () => {
    const s = choiceSetup(true)
    await s.d.onChoose('choice:10:1')
    expect(s.deps.post).toHaveBeenCalledWith('answer', { body: 'B' }, 10, 5)
    expect(s.deps.setPending).toHaveBeenCalledWith('decision:choice:10:1', true)
    expect(s.deps.ui.focusRow).toHaveBeenCalledWith(10)
  })

  it('without a session the click sends no generation', async () => {
    const s = choiceSetup(false)
    await s.d.onChoose('choice:10:0')
    expect(s.deps.post).toHaveBeenCalledWith('answer', { body: 'A' }, 10, undefined)
  })

  it('a double click posts once', async () => {
    const s = choiceSetup(true)
    const p = deferred<void>()
    s.deps.post.mockReturnValueOnce(p.promise)
    const first = s.d.onChoose('choice:10:0')
    await s.d.onChoose('choice:10:0')
    await s.d.onDecide('answer', 'header')
    p.resolve()
    await first
    expect(s.deps.post).toHaveBeenCalledTimes(1)
  })

  it('session_moved goes through the panel mapping and opens no form', async () => {
    const s = choiceSetup(true)
    const err = problem('session_moved')
    s.deps.post.mockRejectedValueOnce(err)
    await s.d.onChoose('choice:10:0')
    expect(s.deps.ui.handle).toHaveBeenCalledWith(err, true)
    expect(s.deps.composer.select).not.toHaveBeenCalled()
  })

  it('invalid_message opens the composer with the answer type and shows the message', async () => {
    const s = choiceSetup(true)
    s.deps.post.mockRejectedValueOnce(problem('invalid_message'))
    s.deps.ui.handle.mockReturnValueOnce('too long')
    await s.d.onChoose('choice:10:0')
    expect(s.deps.composer.select).toHaveBeenCalledWith('answer')
    expect(s.deps.composer.showError).toHaveBeenCalledWith('too long')
  })

  it('other with session: opens the composer as a reply, and its Send carries the generation', async () => {
    const s = choiceSetup(true)
    await s.d.onChoose('choice:10:other')
    expect(s.deps.composer.select).toHaveBeenCalledWith('answer')
    expect(s.deps.composer.reply).toHaveBeenCalledWith(10)
    expect(s.deps.post).not.toHaveBeenCalled()
    await s.d.sendComposer('answer', { body: 'C' }, 10)
    expect(s.deps.post).toHaveBeenCalledWith('answer', { body: 'C' }, 10, 5)
  })

  it('other without session sends no generation', async () => {
    const s = choiceSetup(false)
    await s.d.onChoose('choice:10:other')
    expect(s.deps.composer.reply).toHaveBeenCalledWith(10)
    expect(s.d.formDecision.value).toBeUndefined()
    await s.d.sendComposer('answer', { body: 'C' }, 10)
    expect(s.deps.post).toHaveBeenCalledWith('answer', { body: 'C' }, 10, undefined)
  })

  it('other during an edit asks first', async () => {
    const s = choiceSetup(false)
    s.deps.composer.editing.mockReturnValue(true)
    s.deps.ui.confirm.mockResolvedValueOnce(false)
    await s.d.onChoose('choice:10:other')
    expect(s.deps.ui.confirm).toHaveBeenCalled()
    expect(s.deps.composer.reply).not.toHaveBeenCalled()
  })

  it('an arriving answer hides the buttons; only the messages changed', () => {
    const s = choiceSetup(false)
    expect(s.d.choices.value.size).toBe(1)
    s.messages.value = [
      ...s.messages.value,
      {
        seq: 12,
        rev: 12,
        type: 'answer',
        at: '2026-03-01T10:01:00Z',
        author: { user: 'alice' },
        props: { body: 'A' },
        reply_to: 10,
      },
    ]
    expect(s.d.choices.value.size).toBe(0)
  })

  it.each([
    { withSession: true, text: STALE },
    { withSession: false, text: 'This question is no longer open.' },
  ])(
    'a key that is no longer offered (session: $withSession) says so',
    async ({ withSession, text }) => {
      const s = choiceSetup(withSession)
      await s.d.onChoose('choice:99:0')
      expect(s.deps.ui.toast).toHaveBeenCalledWith(text)
      expect(s.deps.post).not.toHaveBeenCalled()
    }
  )

  it('without a session, Other the composer no longer offers says the question is closed', async () => {
    const s = choiceSetup(false)
    s.deps.composer.select.mockReturnValueOnce(false)
    await s.d.onChoose('choice:10:other')
    expect(s.deps.ui.toast).toHaveBeenCalledWith('This question is no longer open.')
    expect(s.deps.composer.reply).not.toHaveBeenCalled()
  })

  it('session_wrong_state says the session moved on, refetches and opens no form', async () => {
    const s = choiceSetup(true)
    s.deps.post.mockRejectedValueOnce(problem('session_wrong_state'))
    await s.d.onChoose('choice:10:0')
    expect(s.deps.ui.toast).toHaveBeenCalledWith(STALE)
    expect(s.deps.ch.refetch).toHaveBeenCalledTimes(1)
    expect(s.deps.ui.handle).not.toHaveBeenCalled()
    expect(s.deps.composer.select).not.toHaveBeenCalled()
    expect(s.deps.ui.focusRow).not.toHaveBeenCalled()
  })
})
