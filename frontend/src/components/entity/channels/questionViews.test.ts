// @vitest-environment jsdom
//
// Structured questions (TKT-GR1MXL5): the questions of the head's `asked`,
// the batch grouping the library draws, and `display: cards`.
import { describe, it, expect } from 'vitest'
import type {
  ChannelAsked,
  ChannelHead,
  ChannelMessageTypeSchema,
  ChannelTypeSchema,
  Message,
} from '@/types/channels'
import type { ChannelMessageView } from 'rela-components/components/channel/types'
import type { ChannelRow } from 'rela-components/components/channel/channelFold'
import { formatDate, formatDatetime } from '@/utils/format'
import { groupQuestions } from 'rela-components/components/channel/channelQuestions'
import {
  choiceViews,
  messageViews,
  presentationView,
  questionStates,
  questionViews,
} from './channelView'

const at = '2026-10-11T09:00:00Z'

const ask: ChannelMessageTypeSchema = {
  name: 'ask',
  label: 'Ask',
  persist: 'log',
  reply: true,
  actions: [],
  properties: [
    { name: 'note', label: 'Note', type: 'string' },
    { name: 'options', label: 'Options', type: 'string', list: true, max: 80 },
  ],
  choices: { from: 'options', answer: 'answer', into: 'body', other: false },
}
const answer: ChannelMessageTypeSchema = {
  name: 'answer',
  label: 'Answer',
  persist: 'log',
  reply: true,
  actions: [],
  properties: [{ name: 'body', label: 'Body', type: 'string' }],
}
const run: ChannelTypeSchema = {
  name: 'run',
  label: 'Run',
  attach_to: ['ticket'],
  anchor: 'family',
  cardinality: 'one',
  remove_reason_required: false,
  typing: false,
  lifecycle: [],
  message_types: [ask, answer],
  session: {
    label: 'Run',
    initial: 'working',
    states: [
      { name: 'working', label: 'Working', status: 'working' },
      { name: 'asking', label: 'Asking', status: 'input_required' },
    ],
  },
}

const q = (seq: number, over: Partial<Message> = {}): Message => ({
  seq,
  rev: seq,
  type: 'ask',
  author: { user: 'robot' },
  at,
  props: { note: `Question ${seq}?`, options: ['A', 'B'] },
  batch: 10,
  ...over,
})
const reply = (seq: number, to: number, body: string, over: Partial<Message> = {}): Message => ({
  seq,
  rev: seq,
  type: 'answer',
  author: { user: 'ann' },
  at,
  props: { body },
  reply_to: to,
  ...over,
})
const head = (
  asked: ChannelAsked[] | undefined,
  over: { state?: 'open' | 'closed' } = {}
): ChannelHead =>
  ({
    channel: {
      state: over.state ?? 'open',
      session: { state: 'asking', status: 'input_required', generation: 3, transitions: [] },
      ...(asked ? { asked } : {}),
    },
    _actions: { post: ['ask', 'answer'] },
  }) as unknown as ChannelHead
const asked = (seq: number, answers = 0): ChannelAsked => ({ seq, answer_type: 'answer', answers })

describe('questionViews', () => {
  it('offers buttons on every open question of asked, not only the cause', () => {
    const messages = [q(10), q(11), q(12)]
    const got = questionViews(run, head([asked(10), asked(11), asked(12)]), 3, false, messages)
    expect([...got.keys()]).toEqual([10, 11, 12])
    expect(got.get(11)!.choices.map((c) => c.key)).toEqual(['choice:11:0', 'choice:11:1'])
    expect(got.get(11)!.answered).toBe(false)
  })

  it('keeps an answered question with its buttons and latest answer while it is asked', () => {
    const messages = [q(10), q(11), reply(13, 10, 'A'), reply(14, 10, 'B')]
    const got = questionViews(run, head([asked(10, 2), asked(11)]), 3, false, messages)
    expect(got.get(10)).toMatchObject({ answered: true, answer: 'B', answerSeq: 14 })
    expect(got.get(10)!.choices).toHaveLength(2)
    expect(questionStates(got).get(10)).toEqual({ answered: true, answer: 'B', answerSeq: 14 })
    expect(questionStates(got).get(11)).toEqual({ answered: false })
    // choiceViews keeps only the unanswered ones.
    expect([
      ...choiceViews(run, head([asked(10, 2), asked(11)]), 3, false, messages).keys(),
    ]).toEqual([11])
  })

  it('reads answered from the count when the answer is not held', () => {
    const got = questionViews(run, head([asked(10, 1)]), 3, true, [q(10)])
    expect(got.get(10)).toMatchObject({ answered: true })
    expect(got.get(10)!.answer).toBeUndefined()
  })

  it('a removed answer does not name the answer', () => {
    const messages = [
      q(10),
      reply(13, 10, 'A', { removed: { kind: 'author' } } as Partial<Message>),
    ]
    const got = questionViews(run, head([asked(10)]), 3, false, messages)
    expect(got.get(10)).toMatchObject({ answered: false })
  })

  it('leaves out a question no longer asked, and asked seqs that are not held', () => {
    const got = questionViews(run, head([asked(11), asked(99)]), 3, false, [q(10), q(11)])
    expect([...got.keys()]).toEqual([11])
  })

  it('leaves out an asked entry of another answer type', () => {
    const entry = { seq: 10, answer_type: 'other', answers: 0 }
    expect(questionViews(run, head([entry]), 3, false, [q(10)]).size).toBe(0)
  })

  it('offers nothing while the actions are of another generation or the channel is closed', () => {
    expect(questionViews(run, head([asked(10)]), 2, false, [q(10)]).size).toBe(0)
    expect(questionViews(run, head([asked(10)], { state: 'closed' }), 3, false, [q(10)]).size).toBe(
      0
    )
  })

  it('without asked, falls back to the cause and drops an answered question', () => {
    expect([...questionViews(run, head(undefined), 3, false, [q(10), q(11)], 11).keys()]).toEqual([
      11,
    ])
    expect(
      questionViews(run, head(undefined), 3, false, [q(11), reply(12, 11, 'A')], 11).size
    ).toBe(0)
    expect(questionViews(run, head(undefined), 3, true, [q(11)], 11).size).toBe(0)
  })

  it('a caller-known cause (the inbox) ignores asked', () => {
    const got = questionViews(run, head([asked(10), asked(11)]), 3, false, [q(10), q(11)], 11)
    expect([...got.keys()]).toEqual([11])
  })
})

describe('groupQuestions', () => {
  const view = (seq: number, over: Partial<ChannelMessageView> = {}): ChannelMessageView => ({
    seq,
    rev: seq,
    type: 'ask',
    batch: 10,
    ...over,
  })
  const rows = (...views: ChannelMessageView[]): ChannelRow[] =>
    views.map((message) => ({ kind: 'message', message }))
  const isQuestion = (m: ChannelMessageView) => m.type === 'ask'
  const shape = (r: ReturnType<typeof groupQuestions>) =>
    r.map((x) =>
      x.kind === 'questions'
        ? x.messages.map((m) => m.seq)
        : x.kind === 'message'
          ? x.message.seq
          : 'fold'
    )

  it('groups consecutive questions of one batch', () => {
    const got = groupQuestions(
      rows(view(9, { batch: undefined }), view(10), view(11), view(12)),
      isQuestion
    )
    expect(shape(got)).toEqual([9, [10, 11, 12]])
    expect(got[1]).toMatchObject({ kind: 'questions', key: 'questions-10', batch: 10 })
  })

  it('leaves a lone question, and a question without a batch, as a message row', () => {
    expect(shape(groupQuestions(rows(view(10), view(11, { batch: 11 })), isQuestion))).toEqual([
      10, 11,
    ])
    expect(shape(groupQuestions(rows(view(10, { batch: undefined })), isQuestion))).toEqual([10])
  })

  it('a statement in the batch, a removed question or another batch ends the run', () => {
    const got = groupQuestions(
      rows(
        view(10),
        view(11),
        view(12, { type: 'text' }),
        view(13),
        view(14, { removed: { kind: 'author' } }),
        view(15),
        view(16),
        view(17, { batch: 17 }),
        view(18, { batch: 17 })
      ),
      isQuestion
    )
    expect(shape(got)).toEqual([[10, 11], 12, 13, 14, [15, 16], [17, 18]])
  })

  it('a fold ends the run', () => {
    const fold: ChannelRow = {
      kind: 'fold',
      key: 'f',
      first: 12,
      last: 12,
      seqs: [12],
      messages: [],
      count: 1,
      moves: 0,
      labels: [],
      expanded: false,
    }
    const got = groupQuestions([...rows(view(10), view(11)), fold, ...rows(view(13))], isQuestion)
    expect(shape(got)).toEqual([[10, 11], 'fold', 13])
  })

  it('a notice after the first question does not split the batch', () => {
    const notice = { fromLabel: 'Answered', toLabel: 'Asking', reentry: false }
    const got = groupQuestions(
      rows(view(10), view(11, { type: 'session', batch: undefined, notice }), view(12), view(13)),
      isQuestion
    )
    expect(shape(got)).toEqual([[10, 12, 13], 11])
  })

  it('a fold of notices only does not split the batch', () => {
    const fold: ChannelRow = {
      kind: 'fold',
      key: 'f',
      first: 11,
      last: 11,
      seqs: [11],
      messages: [],
      count: 0,
      moves: 1,
      labels: [],
      expanded: false,
    }
    const got = groupQuestions([...rows(view(10)), fold, ...rows(view(12), view(13))], isQuestion)
    expect(shape(got)).toEqual([[10, 12, 13], 'fold'])
  })

  it('a notice that ends a run of one keeps its place', () => {
    const notice = { fromLabel: 'A', toLabel: 'B', reentry: false }
    const got = groupQuestions(
      rows(
        view(10),
        view(11, { type: 'session', batch: undefined, notice }),
        view(12, { type: 'text' })
      ),
      isQuestion
    )
    expect(shape(got)).toEqual([10, 11, 12])
  })
})

describe('display: cards', () => {
  it('presentationView passes cards only with display: cards and items', () => {
    expect(
      presentationView({
        messages: {
          pick: {
            display: 'cards',
            cards: { items: 'options', title: 'label', plus: 'pros', minus: '', badge: 'best' },
          },
          stray: { cards: { items: 'options' } },
          empty: { display: 'cards', cards: { items: '' } },
        },
      })
    ).toEqual({
      messages: {
        pick: {
          fields: undefined,
          cards: { items: 'options', title: 'label', plus: 'pros', badge: 'best' },
        },
        stray: { fields: undefined },
        empty: { fields: undefined },
      },
    })
  })

  it('messageView formats the date fields of object list items, and carries batch and an excerpt', () => {
    const pick: ChannelMessageTypeSchema = {
      ...ask,
      name: 'pick',
      properties: [
        { name: 'note', label: 'Note', type: 'string' },
        {
          name: 'options',
          label: 'Options',
          type: 'object',
          list: true,
          fields: [
            { name: 'label', label: 'Label', type: 'string' },
            { name: 'day', label: 'Day', type: 'date' },
            { name: 'at', label: 'At', type: 'datetime' },
          ],
        },
      ],
      choices: { from: 'options', label: 'label', answer: 'answer', into: 'body', other: false },
    }
    const m = q(10, {
      type: 'pick',
      props: {
        note: 'Which day?',
        options: [{ label: 'A', day: '2026-11-14', at: '2026-11-14T09:30:00Z' }, { label: 'B' }],
      },
    })
    const [v] = messageViews([m], () => pick, { tz: 'UTC' })
    expect(v!.batch).toBe(10)
    expect(v!.excerpt).toBe('Which day?')
    const items = v!.itemText!.options!
    expect(items).toHaveLength(2)
    expect(items[1]).toBeUndefined()
    expect(items[0]!.day).toBe(formatDate('2026-11-14'))
    expect(items[0]!.at).toBe(formatDatetime('2026-11-14T09:30:00Z', 'UTC'))
    expect(items[0]!.label).toBeUndefined()

    // No date field: no itemText, and a single post has no batch.
    const [w] = messageViews([q(11, { batch: undefined })], () => ask, { tz: 'UTC' })
    expect(w!.itemText).toBeUndefined()
    expect(w!.batch).toBeUndefined()
  })
})
