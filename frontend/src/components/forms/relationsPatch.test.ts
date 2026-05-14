import { describe, it, expect } from 'vitest'
import {
  buildRelationsPatch,
  reshapeLegacyToModern,
  OUTGOING_SUFFIX,
  INCOMING_SUFFIX,
  type RelationCardState,
} from './relationsPatch'

// Test builder helpers for RelationCardState — minimize boilerplate
// per the project's test-writing guidance.
function card(overrides: Partial<RelationCardState> = {}): RelationCardState {
  return {
    entries: [],
    added: [],
    removed: [],
    updated: [],
    ...overrides,
  }
}

function pending(entries: Record<string, RelationCardState>): Map<string, RelationCardState> {
  return new Map(Object.entries(entries))
}

describe('buildRelationsPatch', () => {
  it('emits no entry when the card was not touched (autosave/stale-Map safety)', () => {
    // The Map may legitimately contain a key for a card that the user
    // never edited (e.g., autosave wires the same Map for many cards).
    // Emitting `data: []` here would WIPE every edge of that type.
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-1', type: 'label' }],
          added: [],
          removed: [],
          updated: [],
        }),
      }),
    )
    expect(result).toEqual({})
  })

  it('add-one produces single-entry data', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-1', type: 'label' }],
          added: [{ targetId: 'L-1' }],
        }),
      }),
    )
    expect(result).toEqual({ tagged: { data: [{ type: 'label', id: 'L-1' }] } })
  })

  it('removal keeps only retained rows in data', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-2', type: 'label' }],
          removed: ['L-1'],
        }),
      }),
    )
    expect(result.tagged.data).toEqual([{ type: 'label', id: 'L-2' }])
  })

  it('clear-all sends data: []', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [],
          removed: ['L-1', 'L-2'],
        }),
      }),
    )
    expect(result.tagged.data).toEqual([])
  })

  it('preserves per-edge meta', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-1', type: 'label', meta: { weight: 5 } }],
          updated: [{ targetId: 'L-1', meta: { weight: 5 } }],
        }),
      }),
    )
    expect(result.tagged.data[0]).toEqual({
      type: 'label',
      id: 'L-1',
      meta: { weight: 5 },
    })
  })

  it('omits empty meta (no key) — wire shape stays minimal', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-1', type: 'label', meta: {} }],
          added: [{ targetId: 'L-1' }],
        }),
      }),
    )
    expect(result.tagged.data[0]).toEqual({ type: 'label', id: 'L-1' })
  })

  it('preserves per-edge content when present (plumbing for future UI)', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-1', type: 'label', content: 'why this label' }],
          added: [{ targetId: 'L-1' }],
        }),
      }),
    )
    expect(result.tagged.data[0]).toEqual({
      type: 'label',
      id: 'L-1',
      content: 'why this label',
    })
  })

  it('skips incoming-suffix keys (they take the per-edge path)', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + INCOMING_SUFFIX]: card({
          entries: [{ id: 'T-1', type: 'ticket' }],
          added: [{ targetId: 'T-1' }],
        }),
      }),
    )
    expect(result).toEqual({})
  })

  it('mixed in+out keeps outgoing, skips incoming', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-1', type: 'label' }],
          added: [{ targetId: 'L-1' }],
        }),
        ['referenced-by' + INCOMING_SUFFIX]: card({
          entries: [{ id: 'T-1', type: 'ticket' }],
          added: [{ targetId: 'T-1' }],
        }),
      }),
    )
    expect(Object.keys(result)).toEqual(['tagged'])
  })

  it('throws loudly when an entry is missing type (drift surfaces, not silent corruption)', () => {
    expect(() =>
      buildRelationsPatch(
        pending({
          ['tagged' + OUTGOING_SUFFIX]: card({
            // Cast deliberately — simulates older-server payload that
            // landed in entries via a stale RelationEntry without
            // backend Step 0.
            entries: [{ id: 'L-1' } as unknown as { id: string; type: string }],
            added: [{ targetId: 'L-1' }],
          }),
        }),
      ),
    ).toThrow(/missing 'type'/)
  })
})

describe('reshapeLegacyToModern', () => {
  it('returns null when ANY id has no type — caller falls back to legacy', () => {
    const result = reshapeLegacyToModern(
      { 'depends-on': ['T-1', 'C-unknown'] },
      { 'depends-on': new Map([['T-1', 'ticket']]) },
    )
    expect(result).toBeNull()
  })

  it('reshapes when all ids have known types', () => {
    const result = reshapeLegacyToModern(
      { 'depends-on': ['T-1', 'BUG-1'] },
      {
        'depends-on': new Map([
          ['T-1', 'ticket'],
          ['BUG-1', 'bug'],
        ]),
      },
    )
    expect(result).toEqual({
      'depends-on': {
        data: [
          { type: 'ticket', id: 'T-1' },
          { type: 'bug', id: 'BUG-1' },
        ],
      },
    })
  })

  it('empty legacy lists become explicit data: [] (clear-all preserved)', () => {
    const result = reshapeLegacyToModern({ tagged: [] }, { tagged: new Map() })
    expect(result).toEqual({ tagged: { data: [] } })
  })

  it('preserves polymorphic-target type per row (no to[0] guessing)', () => {
    // Hard-coded values are the load-bearing assertion: that types come
    // from the per-id Map, not from a "first allowed target type"
    // schema fallback that would homogenize them.
    const result = reshapeLegacyToModern(
      { 'depends-on': ['T-1', 'BUG-1', 'FEAT-1', 'DT-1'] },
      {
        'depends-on': new Map([
          ['T-1', 'ticket'],
          ['BUG-1', 'bug'],
          ['FEAT-1', 'feature'],
          ['DT-1', 'doc-task'],
        ]),
      },
    )
    expect(result?.['depends-on'].data.map((d) => d.type)).toEqual([
      'ticket',
      'bug',
      'feature',
      'doc-task',
    ])
  })
})
