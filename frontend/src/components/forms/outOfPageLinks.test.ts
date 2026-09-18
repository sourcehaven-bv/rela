import { describe, it, expect } from 'vitest'
import { missingIds, mergeResolvedLinks, indexKnownEntities, resolveSelected } from './outOfPageLinks'
import type { Entity, RelationEntry } from '@/types'

function entity(id: string, type = 'ticket'): Entity {
  return { id, type, properties: {}, _title: id }
}

function edge(id: string, type = 'ticket'): RelationEntry {
  return { id, type, direction: 'outgoing' }
}

// BUG-LSCDJK. These helpers back the RelationPicker's out-of-page resolution;
// the picker's own tests cover the wiring, these cover the folding rules.
describe('missingIds', () => {
  it('returns only the ids no known entity covers', () => {
    const known = indexKnownEntities([], [entity('TKT-001')])
    expect(missingIds(['TKT-001', 'TKT-999'], known)).toEqual(['TKT-999'])
  })

  it('returns empty when everything is known, so the caller can skip the request', () => {
    const known = indexKnownEntities([entity('TKT-999')], [entity('TKT-001')])
    expect(missingIds(['TKT-001', 'TKT-999'], known)).toEqual([])
  })
})

describe('mergeResolvedLinks', () => {
  it('keeps links resolved by an earlier pass', () => {
    // A later call asks only for what is missing now, so replacing instead of
    // merging would drop the first pass's work and re-break its save.
    const first = mergeResolvedLinks([], [edge('TKT-999')], new Set(['TKT-999']))
    const second = mergeResolvedLinks(first, [edge('TKT-998')], new Set(['TKT-998']))

    expect(second.map((e) => e.id).sort()).toEqual(['TKT-998', 'TKT-999'])
  })

  it('takes only wanted ids', () => {
    const out = mergeResolvedLinks([], [edge('TKT-999'), edge('TKT-001')], new Set(['TKT-999']))
    expect(out.map((e) => e.id)).toEqual(['TKT-999'])
  })

  it('skips an edge with no type, which cannot form a resource identifier', () => {
    const untyped: RelationEntry = { id: 'TKT-999', type: '' }
    expect(mergeResolvedLinks([], [untyped], new Set(['TKT-999']))).toEqual([])
  })

  it('sets _title to the id so the record satisfies the total-_title contract', () => {
    const [resolved] = mergeResolvedLinks([], [edge('TKT-999')], new Set(['TKT-999']))
    expect(resolved._title).toBe('TKT-999')
    expect(resolved.type).toBe('ticket')
  })
})

describe('indexKnownEntities', () => {
  it('lets a candidate win over a resolved link of the same id', () => {
    // Reachable after a newly created entity is pushed into candidates, or
    // after a candidate reload: there the list read is the fresher of the two.
    const known = indexKnownEntities([entity('TKT-1', 'stale')], [entity('TKT-1', 'fresh')])
    expect(known.get('TKT-1')?.type).toBe('fresh')
  })
})

describe('resolveSelected', () => {
  it('returns entities in value order, not index order', () => {
    const known = indexKnownEntities([], [entity('TKT-1'), entity('TKT-2')])
    expect(resolveSelected(['TKT-2', 'TKT-1'], known).map((e) => e.id)).toEqual(['TKT-2', 'TKT-1'])
  })

  it('emits one entity per repeated id', () => {
    const known = indexKnownEntities([], [entity('TKT-1')])
    expect(resolveSelected(['TKT-1', 'TKT-1'], known).map((e) => e.id)).toEqual(['TKT-1'])
  })

  it('drops ids that resolve to nothing rather than emitting a hole', () => {
    const known = indexKnownEntities([], [entity('TKT-1')])
    expect(resolveSelected(['TKT-1', 'TKT-unknown'], known).map((e) => e.id)).toEqual(['TKT-1'])
  })
})
