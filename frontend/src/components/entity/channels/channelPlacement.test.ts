import { describe, it, expect } from 'vitest'
import type { ChannelPresentation } from '@/types'
import {
  ABOVE_RELATIONS_KEY,
  aboveRelationsIndex,
  placementOf,
  typesAt,
  withAboveRelationsBlock,
} from './channelPlacement'

describe('placementOf', () => {
  it.each<[ChannelPresentation | undefined, string]>([
    [undefined, 'end'],
    [{}, 'end'],
    [{ placement: '' }, 'end'],
    [{ placement: 'above_relations' }, 'above_relations'],
    // A value from a newer server keeps the default.
    [{ placement: 'tab' }, 'end'],
    [{ placement: 'Above_Relations' }, 'end'],
  ])('%j is %s', (p, want) => {
    expect(placementOf(p)).toBe(want)
  })
})

describe('typesAt', () => {
  const presentations: Record<string, ChannelPresentation> = {
    run: { placement: 'above_relations' },
    review: { placement: 'above_relations', session: { display: 'compact' } },
    chat: { session: { display: 'compact' } },
  }
  const at = (name: string) => presentations[name]
  const names = ['chat', 'run', 'notes', 'review']

  it('keeps the given order per block', () => {
    expect(typesAt(names, at, 'above_relations')).toEqual(['run', 'review'])
    expect(typesAt(names, at, 'end')).toEqual(['chat', 'notes'])
  })

  it('puts every type at the end without placements', () => {
    expect(typesAt(names, () => undefined, 'end')).toEqual(names)
    expect(typesAt(names, () => undefined, 'above_relations')).toEqual([])
  })
})

describe('aboveRelationsIndex', () => {
  it.each<[string, { source?: string }[], number]>([
    [
      'generated view: properties, body, relations',
      [{ source: 'entry' }, { source: 'entry' }, { source: 'deps' }, { source: 'subs' }],
      2,
    ],
    ['custom view starting with a relation table', [{ source: 'subs' }, { source: 'entry' }], 0],
    ['only entry sections', [{ source: 'entry' }, { source: 'entry' }], 2],
    ['no sections', [], 0],
    ['a collection source is not the entry', [{ source: 'entry' }, { source: 'subs' }], 1],
    ['a section without a source is not the entry', [{ source: 'entry' }, {}], 1],
  ])('%s', (_name, sections, want) => {
    expect(aboveRelationsIndex(sections)).toBe(want)
  })
})

describe('withAboveRelationsBlock', () => {
  const sec = (sectionId: string, entry = false) => ({
    sectionId,
    source: entry ? 'entry' : 'subs',
  })

  it('puts the block at the index, keyed apart from the sections', () => {
    const items = withAboveRelationsBlock([sec('props', true), sec('subs'), sec('deps')])
    expect(items.map((i) => i.key)).toEqual(['props', ABOVE_RELATIONS_KEY, 'subs', 'deps'])
    expect(items.map((i) => i.sectionIndex)).toEqual([0, -1, 1, 2])
    expect(items[1]!.section).toBeNull()
  })

  it('keeps the block key when the index moves', () => {
    const a = withAboveRelationsBlock([sec('subs')])
    const b = withAboveRelationsBlock([sec('props', true), sec('subs')])
    expect(a.find((i) => i.section === null)!.key).toBe(b.find((i) => i.section === null)!.key)
  })

  it('appends the block when there are no sections', () => {
    expect(withAboveRelationsBlock([]).map((i) => i.key)).toEqual([ABOVE_RELATIONS_KEY])
  })
})
