import { describe, it, expect } from 'vitest'
import { offeredDetailActions } from './detailActions'
import type { ActionConfig } from '@/types'

function lookup(actions: Record<string, ActionConfig>) {
  return (id: string) => actions[id]
}

describe('offeredDetailActions', () => {
  const cases: {
    name: string
    affordances: Record<string, boolean> | undefined
    actions: Record<string, ActionConfig>
    want: string[]
  }[] = [
    {
      name: 'no affordances',
      affordances: undefined,
      actions: { a: { label: 'A' } },
      want: [],
    },
    {
      name: 'only true action keys with a declared action',
      affordances: { update: true, 'action:a': true, 'action:b': false, 'action:gone': true },
      actions: { a: { label: 'A' }, b: { label: 'B' } },
      want: ['a'],
    },
    {
      name: 'by label without order',
      affordances: { 'action:archive': true, 'action:activate': true, 'action:suspend': true },
      actions: {
        archive: { label: 'Archive' },
        activate: { label: 'Make active' },
        suspend: { label: 'Suspend' },
      },
      want: ['archive', 'activate', 'suspend'],
    },
    {
      name: 'order first, absent counts as 0',
      affordances: {
        'action:archive': true,
        'action:activate': true,
        'action:suspend': true,
        'action:first': true,
      },
      actions: {
        archive: { label: 'Archive', order: 90 },
        activate: { label: 'Make active', order: 10 },
        suspend: { label: 'Suspend' },
        first: { label: 'Zzz', order: -1 },
      },
      want: ['first', 'suspend', 'activate', 'archive'],
    },
    {
      name: 'equal order and label falls back to the id',
      affordances: { 'action:b': true, 'action:a': true },
      actions: { b: { label: 'Same' }, a: { label: 'Same' } },
      want: ['a', 'b'],
    },
    {
      name: 'the id labels an action without a label',
      affordances: { 'action:zed': true, 'action:alpha': true },
      actions: { zed: {}, alpha: { label: 'Omega' } },
      want: ['alpha', 'zed'],
    },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(offeredDetailActions(c.affordances, lookup(c.actions)).map((a) => a.id)).toEqual(
        c.want
      )
    })
  }
})
