import { describe, expect, it } from 'vitest'
import type { RelationType } from '@/types'
import { dependentsOf, ownerQueries, sameIds } from './scopedRelations'

const rels: Record<string, RelationType> = {
  in_project: { label: 'in_project', from: ['todo'], to: ['project'], max_outgoing: 1 },
  has_heading: { label: 'has_heading', from: ['project'], to: ['heading'], owning: true },
  under_heading: {
    label: 'under_heading',
    from: ['todo'],
    to: ['heading'],
    within: 'in_project',
    on_change: 'clear',
    scope_owners: { heading: 'has_heading' },
  },
  under_section: {
    label: 'under_section',
    from: ['todo'],
    to: ['heading'],
    within: 'in_project',
    on_change: 'reject',
    scope_owners: { heading: 'has_heading' },
  },
}

describe('ownerQueries', () => {
  it('lists the owning edges of the scope target', () => {
    expect(ownerQueries('under_heading', rels)).toEqual([
      { ownerType: 'project', owningRelation: 'has_heading', targetType: 'heading' },
    ])
  })
  it('is empty for a relation that is not scoped', () => {
    expect(ownerQueries('in_project', rels)).toEqual([])
    expect(ownerQueries('missing', rels)).toEqual([])
  })
  it('skips a target type without an owner', () => {
    const r = { ...rels, under_heading: { ...rels.under_heading!, scope_owners: {} } }
    expect(ownerQueries('under_heading', r)).toEqual([])
  })
})

describe('dependentsOf', () => {
  it('names each scoped relation with its on_change', () => {
    expect(dependentsOf('in_project', rels)).toEqual([
      { relation: 'under_heading', onChange: 'clear' },
      { relation: 'under_section', onChange: 'reject' },
    ])
    expect(dependentsOf('has_heading', rels)).toEqual([])
  })
})

describe('sameIds', () => {
  it('ignores order', () => {
    expect(sameIds(['a', 'b'], ['b', 'a'])).toBe(true)
    expect(sameIds(['a'], ['b'])).toBe(false)
    expect(sameIds([], ['a'])).toBe(false)
  })
})
