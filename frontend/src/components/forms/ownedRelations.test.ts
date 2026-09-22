import { describe, it, expect } from 'vitest'
import { ownedRelationKeys, filterOwnedRelations, untypedRelationKeys } from './ownedRelations'

describe('ownedRelationKeys', () => {
  it('owns a rendered outgoing relation', () => {
    expect(ownedRelationKeys([{ relation: 'gaat_over', direction: 'outgoing' }])).toEqual(
      new Set(['gaat_over'])
    )
  })

  it('treats a directionless field as outgoing, matching the form default', () => {
    expect(ownedRelationKeys([{ relation: 'gaat_over' }])).toEqual(new Set(['gaat_over']))
  })

  // An incoming picker loads its own edges and delivers them via
  // pendingCardChanges under the inverse body key. The inverse key the entity
  // GET supplies is a stale, untyped duplicate — owning it is what let
  // `onderdeel_van` reach the payload and abort the save.
  it('does not own the inverse key of an incoming field', () => {
    const owned = ownedRelationKeys([{ relation: 'bestaat_uit', direction: 'incoming' }])

    expect(owned.has('onderdeel_van')).toBe(false)
  })

  it('does not own a relation no field names', () => {
    const owned = ownedRelationKeys([{ relation: 'gaat_over', direction: 'outgoing' }])

    expect(owned.has('onderdeel_van')).toBe(false)
  })

  // A form may render both directions of one relation. The outgoing half must
  // stay owned regardless of the order the fields appear in.
  it('owns the outgoing half when both directions are rendered', () => {
    const incomingFirst = ownedRelationKeys([
      { relation: 'blocks', direction: 'incoming' },
      { relation: 'blocks', direction: 'outgoing' },
    ])
    const outgoingFirst = ownedRelationKeys([
      { relation: 'blocks', direction: 'outgoing' },
      { relation: 'blocks', direction: 'incoming' },
    ])

    expect(incomingFirst).toEqual(new Set(['blocks']))
    expect(outgoingFirst).toEqual(new Set(['blocks']))
  })

  // A `symmetric: true` relation whose inverse.id is its own name is legal —
  // the one permitted name overlap in the metamodel — so the GET keys BOTH
  // directions under that single name. An outgoing field must therefore own it,
  // or the form would drop its own edges. The direction rule gets this right
  // without a special case; pinned here because `prefillRouting.ts` pins the
  // analogous case and the property is easy to break silently.
  it('owns a symmetric self-inverse relation rendered outgoing', () => {
    expect(ownedRelationKeys([{ relation: 'mirrors', direction: 'outgoing' }])).toEqual(
      new Set(['mirrors'])
    )
  })

  it('ignores property fields, which name no relation', () => {
    expect(ownedRelationKeys([{}, { relation: 'gaat_over' }, {}])).toEqual(new Set(['gaat_over']))
  })
})

describe('filterOwnedRelations', () => {
  // The BUG-KQSOJ2 shape: TASK-7F8K's GET carries `onderdeel_van`, which the
  // edit_taak form renders only as the incoming side of `bestaat_uit` — so it
  // has no picker and no type, and sending it aborted the whole payload.
  it('drops a relation the form does not own', () => {
    const out = filterOwnedRelations(
      { gaat_over: ['NCR-001'], onderdeel_van: ['PROJ-DP5C'] },
      new Set(['gaat_over'])
    )

    expect(out).toEqual({ gaat_over: ['NCR-001'] })
  })

  it('keeps every owned relation, including empty ones that clear edges', () => {
    const out = filterOwnedRelations(
      { gaat_over: [], spawnt: ['TASK-1'] },
      new Set(['gaat_over', 'spawnt'])
    )

    expect(out).toEqual({ gaat_over: [], spawnt: ['TASK-1'] })
  })

  // A `link_as: to` create button may pre-link a relation this form does not
  // render. That path registers its own pickerTypes entry, so the edge is typed
  // and safe — dropping it would trip the submit-site post-condition check and
  // turn a working create into a hard error.
  it('keeps an explicitly allowed relation the form does not own', () => {
    const out = filterOwnedRelations(
      { onderdeel_van: ['PROJ-DP5C'] },
      new Set(['gaat_over']),
      new Set(['onderdeel_van'])
    )

    expect(out).toEqual({ onderdeel_van: ['PROJ-DP5C'] })
  })

  it('returns an empty record when nothing is owned', () => {
    const out = filterOwnedRelations({ onderdeel_van: ['PROJ-DP5C'] }, new Set())

    expect(out).toEqual({})
  })
})

describe('untypedRelationKeys', () => {
  it('names the relation holding an untyped id', () => {
    const bad = untypedRelationKeys(
      { gaat_over: ['NCR-001'], spawnt: ['TASK-1'] },
      { gaat_over: new Map([['NCR-001', 'ncr']]) }
    )

    expect(bad).toEqual(['spawnt'])
  })

  it('names a relation where only some ids resolve', () => {
    const bad = untypedRelationKeys(
      { gaat_over: ['NCR-001', 'PROCEDURE-6l21'] },
      { gaat_over: new Map([['NCR-001', 'ncr']]) }
    )

    expect(bad).toEqual(['gaat_over'])
  })

  it('reports nothing when every id resolves', () => {
    const bad = untypedRelationKeys(
      { gaat_over: ['NCR-001'] },
      { gaat_over: new Map([['NCR-001', 'ncr']]) }
    )

    expect(bad).toEqual([])
  })

  // An empty list is a legitimate "clear all edges of this type" instruction
  // and types nothing, so it must not be reported as a failure.
  it('does not report an empty relation', () => {
    expect(untypedRelationKeys({ gaat_over: [] }, {})).toEqual([])
  })
})
