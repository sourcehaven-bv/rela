import { describe, it, expect } from 'vitest'
import { planPrefillRouting } from './prefillRouting'

const peers = [{ id: 'TKT-2', type: 'ticket' }]

describe('planPrefillRouting', () => {
  it('routes a card-managed canonical key as outgoing', () => {
    const { cardRoutes } = planPrefillRouting(
      { blocks: peers },
      new Set(['blocks']),
      new Map([['blocks', 'blockedBy']])
    )

    expect(cardRoutes).toEqual([{ mapKey: 'blocks-outgoing', sourceKey: 'blocks', peers }])
  })

  it('routes an inverse key as incoming, under the canonical relation', () => {
    const { cardRoutes } = planPrefillRouting(
      { blockedBy: peers },
      new Set(['blocks']),
      new Map([['blocks', 'blockedBy']])
    )

    expect(cardRoutes).toEqual([{ mapKey: 'blocks-incoming', sourceKey: 'blockedBy', peers }])
  })

  // A `symmetric: true` relation whose inverse.id is its own name is legal —
  // the one permitted name overlap in the metamodel — and maps to ITSELF. A
  // direction test of "is this key in the inverse lookup?" says yes and routes
  // the outgoing key as incoming. The body key happens to come out right
  // because the inverse of `mirrors` is `mirrors`, so the bug is invisible at
  // the payload; it is visible here, which is the point of testing the
  // decision rather than only its downstream effect.
  it('treats a symmetric self-inverse relation as outgoing', () => {
    const { cardRoutes } = planPrefillRouting(
      { mirrors: peers },
      new Set(['mirrors']),
      new Map([['mirrors', 'mirrors']])
    )

    expect(cardRoutes[0].mapKey).toBe('mirrors-outgoing')
  })

  it('ignores a relation whose field is not card-managed', () => {
    const { cardRoutes } = planPrefillRouting({ refs: peers }, new Set(['blocks']), new Map())

    expect(cardRoutes).toEqual([])
  })

  it('ignores an inverse key whose canonical relation is not card-managed', () => {
    const { cardRoutes } = planPrefillRouting(
      { blockedBy: peers },
      new Set(['refs']),
      new Map([['blocks', 'blockedBy']])
    )

    expect(cardRoutes).toEqual([])
  })

  it('routes both directions of one relation independently', () => {
    const { cardRoutes } = planPrefillRouting(
      { blocks: peers, blockedBy: [{ id: 'TKT-3', type: 'ticket' }] },
      new Set(['blocks']),
      new Map([['blocks', 'blockedBy']])
    )

    expect(cardRoutes.map((r) => r.mapKey).sort()).toEqual(['blocks-incoming', 'blocks-outgoing'])
  })

  it('returns nothing for an empty prefill', () => {
    expect(planPrefillRouting({}, new Set(['blocks']), new Map()).cardRoutes).toEqual([])
  })
})
