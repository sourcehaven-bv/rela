import { describe, it, expect } from 'vitest'
import {
  foldMessages,
  rowFor,
  shownAs,
  type ChannelRow,
  type FoldOptions,
} from 'rela-components/components/channel/channelFold'
import { CHANNEL_OPERATIONS } from 'rela-components/components/channel/channelFields'
import { MESSAGE_OPS } from '@/types/channels'
import type { ChannelMessageView } from 'rela-components/components/channel/types'

// The fold of quiet message types (TKT-03DMXH): which held messages become
// rows, fold lines or nothing. The library's vitest runs only stories, so
// the pure function is tested here.

const DISPLAY: Record<string, 'collapsed' | 'hidden'> = {
  progress: 'collapsed',
  note: 'collapsed',
  claim: 'hidden',
}

function opts(over: Partial<FoldOptions> = {}): FoldOptions {
  return {
    displayOf: (type) => DISPLAY[type],
    labelOf: (type) => type[0]!.toUpperCase() + type.slice(1),
    showAll: false,
    keep: new Set(),
    expanded: new Set(),
    ...over,
  }
}

const m = (
  seq: number,
  type: string,
  over: Partial<ChannelMessageView> = {}
): ChannelMessageView => ({
  seq,
  rev: seq,
  type,
  ...over,
})
const removed = (seq: number): ChannelMessageView => ({
  seq,
  rev: seq,
  removed: { kind: 'author' },
})
const notice = (seq: number): ChannelMessageView => ({
  seq,
  rev: seq,
  type: 'session',
  notice: { fromLabel: 'A', toLabel: 'B', reentry: false },
})

/** A compact picture of the rows: a seq for a message, `[seqs shown|expanded]` for a fold. */
function shape(rows: ChannelRow[]): (number | string)[] {
  return rows.map((r) =>
    r.kind === 'message'
      ? r.message.seq
      : `[${r.messages.map((x) => x.seq).join(',')}${r.expanded ? '|open' : ''}]`
  )
}

describe('foldMessages', () => {
  const cases: {
    name: string
    messages: ChannelMessageView[]
    over?: Partial<FoldOptions>
    want: (number | string)[]
  }[] = [
    {
      name: 'one run',
      messages: [m(1, 'text'), m(2, 'progress'), m(3, 'progress'), m(4, 'text')],
      want: [1, '[2,3]', 4],
    },
    {
      name: 'two runs split by a notice',
      messages: [m(1, 'progress'), notice(2), m(3, 'progress')],
      want: ['[1]', 2, '[3]'],
    },
    {
      name: 'collapsed, hidden, collapsed is one run',
      messages: [m(1, 'progress'), m(2, 'claim'), m(3, 'progress')],
      want: ['[1,3]'],
    },
    {
      name: 'expanding a mixed run shows only collapsed',
      messages: [m(1, 'progress'), m(2, 'claim'), m(3, 'progress')],
      over: { expanded: new Set([2]) },
      want: ['[1,3|open]'],
    },
    {
      name: 'hidden only',
      messages: [m(1, 'text'), m(2, 'claim'), m(3, 'claim'), m(4, 'text')],
      want: [1, 4],
    },
    {
      name: 'removed inside a run joins it',
      messages: [m(1, 'progress'), removed(2), m(3, 'progress')],
      want: ['[1,2,3]'],
    },
    {
      name: 'removed inside a hidden-only run is a row',
      messages: [m(1, 'text'), m(2, 'claim'), removed(3), m(4, 'claim'), m(5, 'text')],
      want: [1, 3, 5],
    },
    {
      name: 'kept removed message ends a run',
      messages: [m(1, 'progress'), removed(2), m(3, 'progress')],
      over: { keep: new Set([2]) },
      want: ['[1]', 2, '[3]'],
    },
    {
      name: 'revealed hidden member splits the fold',
      messages: [m(1, 'progress'), m(2, 'claim'), m(3, 'progress')],
      over: { keep: new Set([2]) },
      want: ['[1]', 2, '[3]'],
    },
    {
      name: 'removed outside a run is a row',
      messages: [m(1, 'text'), removed(2), m(3, 'progress'), removed(4), m(5, 'text')],
      want: [1, 2, '[3]', 4, 5],
    },
    {
      name: 'decision row kept',
      messages: [m(1, 'progress'), m(2, 'progress'), m(3, 'progress')],
      over: { keep: new Set([2]) },
      want: ['[1]', 2, '[3]'],
    },
    {
      name: 'choice row kept',
      messages: [m(1, 'claim'), m(2, 'claim')],
      over: { keep: new Set([2]) },
      want: [2],
    },
    {
      name: 'type-action row kept',
      messages: [m(1, 'progress'), m(2, 'progress', { actions: ['edit', 'record'] })],
      want: ['[1]', 2],
    },
    {
      name: 'operation-only actions not kept',
      messages: [m(1, 'progress', { actions: ['edit', 'delete', 'remove', 'release'] })],
      want: ['[1]'],
    },
    {
      name: 'revealed seq kept',
      messages: [m(1, 'claim'), m(2, 'claim'), m(3, 'claim')],
      over: { keep: new Set([2]) },
      want: [2],
    },
    {
      name: 'toggle on',
      messages: [m(1, 'progress'), m(2, 'claim'), removed(3)],
      over: { showAll: true },
      want: [1, 2, 3],
    },
    {
      name: 'no quiet types (output equals input)',
      messages: [m(1, 'text'), removed(2), notice(3), m(4, 'text')],
      over: { displayOf: () => undefined },
      want: [1, 2, 3, 4],
    },
  ]
  for (const tc of cases) {
    it(tc.name, () => {
      expect(shape(foldMessages(tc.messages, opts(tc.over)))).toEqual(tc.want)
    })
  }

  it('labels the fold with type labels in first-seen order', () => {
    const [fold] = foldMessages(
      [m(1, 'note'), m(2, 'claim'), m(3, 'progress'), m(4, 'note')],
      opts()
    )
    expect(fold).toMatchObject({ kind: 'fold', labels: ['Note', 'Progress'], first: 1, last: 4 })
    expect(fold!.kind === 'fold' && fold.seqs).toEqual([1, 2, 3, 4])
  })

  it('fold key unchanged after append', () => {
    const before = foldMessages([m(1, 'text'), m(2, 'progress')], opts())
    const after = foldMessages(
      [m(1, 'text'), m(2, 'progress'), m(3, 'claim'), m(4, 'progress')],
      opts()
    )
    expect(before[1]).toMatchObject({ kind: 'fold', key: 'fold-2' })
    expect(after[1]).toMatchObject({ kind: 'fold', key: 'fold-2', last: 4 })
  })

  it('expanded run stays expanded after append', () => {
    const expanded = new Set([2, 3])
    const rows = foldMessages(
      [m(1, 'text'), m(2, 'progress'), m(3, 'progress'), m(4, 'progress')],
      opts({ expanded })
    )
    expect(shape(rows)).toEqual([1, '[2,3,4|open]'])
  })

  it('expanded run stays expanded after prepend and trim', () => {
    const expanded = new Set([5, 6])
    const prepended = [m(3, 'progress'), m(4, 'progress'), m(5, 'progress'), m(6, 'progress')]
    expect(shape(foldMessages(prepended, opts({ expanded })))).toEqual(['[3,4,5,6|open]'])
    expect(shape(foldMessages(prepended.slice(3), opts({ expanded })))).toEqual(['[6|open]'])
  })
})

describe('rowFor and shownAs', () => {
  const messages = [
    m(1, 'text'),
    m(2, 'progress'),
    m(3, 'claim'),
    m(4, 'progress'),
    m(5, 'claim'),
    m(6, 'text'),
  ]

  it('finds the message row, the fold of a run, or the next row', () => {
    const rows = foldMessages(messages, opts())
    expect(rowFor(rows, 1)).toBe(rows[0])
    expect(rowFor(rows, 3)).toBe(rows[1])
    expect(rowFor(rows, 5)).toBe(rows[1])
    expect(rowFor(rows, 6)).toBe(rows[2])
    expect(rowFor(rows, 7)).toBeUndefined()
    const hiddenOnly = foldMessages([m(1, 'text'), m(2, 'claim'), m(3, 'text')], opts())
    expect(rowFor(hiddenOnly, 2)).toBe(hiddenOnly[1])
  })

  it('says how a message renders', () => {
    const rows = foldMessages(messages, opts())
    expect(shownAs(rows, 1)).toBe('full')
    expect(shownAs(rows, 2)).toBe('folded')
    expect(shownAs(rows, 3)).toBeUndefined()
    expect(shownAs(rows, 5)).toBeUndefined()
    const open = foldMessages(messages, opts({ expanded: new Set([2]) }))
    expect(shownAs(open, 4)).toBe('full')
    expect(shownAs(open, 3)).toBeUndefined()
  })
})

describe('CHANNEL_OPERATIONS', () => {
  it('names the same operations as MESSAGE_OPS', () => {
    expect([...CHANNEL_OPERATIONS].sort()).toEqual([...MESSAGE_OPS].sort())
  })
})
