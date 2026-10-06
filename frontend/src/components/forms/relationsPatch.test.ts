import { describe, it, expect } from 'vitest'
import {
  mergeRelationsFields,
  addressedEntry,
  buildRelationsPatch as buildRelationsPatchRaw,
  confirmRelations,
  reshapeLegacyToModern,
  type ConfirmedEdges,
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

// buildRelationsPatch with a default empty inverseByRelation map for
// outgoing-only tests. Incoming-suffix tests must opt in by passing
// their own map.
function buildRelationsPatch(
  p: Map<string, RelationCardState>,
  inverse: Map<string, string> = new Map()
) {
  return buildRelationsPatchRaw(p, inverse)
}

describe('buildRelationsPatch', () => {
  it('emits no entry when the card was not touched (autosave/stale-Map safety)', () => {
    // The Map may legitimately contain a key for a card that the user
    // never edited (e.g., autosave wires the same Map for many cards).
    // Emitting anything here would rewrite edges the user did not touch.
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-1', type: 'label' }],
          added: [],
          removed: [],
          updated: [],
        }),
      })
    )
    expect(result).toEqual({})
  })

  it('add-one produces a single add', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-1', type: 'label' }],
          added: [{ targetId: 'L-1' }],
        }),
      })
    )
    expect(result).toEqual({ tagged: { add: [{ type: 'label', id: 'L-1' }] } })
  })

  it('removal names only the removed edge', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-2', type: 'label' }],
          removed: ['L-1'],
        }),
      })
    )
    // The loaded set is entries + removed - added; L-2 is unchanged.
    expect(result.tagged).toEqual({ remove: [{ id: 'L-1' }] })
  })

  it('clear-all removes each loaded edge, never sends data: []', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [],
          removed: ['L-1', 'L-2'],
        }),
      })
    )
    expect(result.tagged).toEqual({ remove: [{ id: 'L-1' }, { id: 'L-2' }] })
  })

  it('preserves per-edge meta', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-1', type: 'label', meta: { weight: 5 } }],
          updated: [{ targetId: 'L-1', meta: { weight: 5 } }],
        }),
      })
    )
    // An updated edge is re-sent as an add, which upserts its meta.
    expect(result.tagged).toEqual({ add: [{ type: 'label', id: 'L-1', meta: { weight: 5 } }] })
  })

  it('omits empty meta (no key) — wire shape stays minimal', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-1', type: 'label', meta: {} }],
          added: [{ targetId: 'L-1' }],
        }),
      })
    )
    expect(result.tagged).toEqual({ add: [{ type: 'label', id: 'L-1' }] })
  })

  it('preserves per-edge content when present (plumbing for future UI)', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-1', type: 'label', content: 'why this label' }],
          added: [{ targetId: 'L-1' }],
        }),
      })
    )
    expect(result.tagged).toEqual({
      add: [{ type: 'label', id: 'L-1', content: 'why this label' }],
    })
  })

  it('incoming-suffix key emits under inverse body key (TKT-GFQK)', () => {
    const result = buildRelationsPatch(
      pending({
        ['blocks' + INCOMING_SUFFIX]: card({
          entries: [{ id: 'T-1', type: 'ticket' }],
          added: [{ targetId: 'T-1' }],
        }),
      }),
      new Map([['blocks', 'blockedBy']])
    )
    // Backend resolveDirection sees `blockedBy`, swaps endpoints, and
    // writes `T-1 --blocks--> path-entity`.
    expect(result).toEqual({
      blockedBy: { add: [{ type: 'ticket', id: 'T-1' }] },
    })
  })

  it('names an incoming content edge by its source face', () => {
    // POL-1 cites this entity from two faces: two edges. Removing the draft
    // one must not touch the published one.
    const loaded = [
      addressedEntry({ id: 'POL-1', type: 'policy', face: 'draft' }),
      addressedEntry({ id: 'POL-1', type: 'policy', face: 'published' }),
    ]
    const result = buildRelationsPatch(
      pending({
        ['cites' + INCOMING_SUFFIX]: card({
          entries: [loaded[1]],
          removed: [loaded[0].id],
        }),
      }),
      new Map([['cites', 'cited-by']])
    )
    expect(result).toEqual({ 'cited-by': { remove: [{ id: 'POL-1@draft' }] } })
  })

  it('mixed canonical+inverse: both emit under their respective body keys', () => {
    const result = buildRelationsPatch(
      pending({
        ['tagged' + OUTGOING_SUFFIX]: card({
          entries: [{ id: 'L-1', type: 'label' }],
          added: [{ targetId: 'L-1' }],
        }),
        ['blocks' + INCOMING_SUFFIX]: card({
          entries: [{ id: 'T-1', type: 'ticket' }],
          added: [{ targetId: 'T-1' }],
        }),
      }),
      new Map([['blocks', 'blockedBy']])
    )
    expect(Object.keys(result).sort()).toEqual(['blockedBy', 'tagged'])
  })

  it('throws when incoming-suffix key has no inverse declared in the lookup', () => {
    // DynamicForm pre-flights this at form-load time; the throw is the
    // defensive last-line guard for the case where pre-flight failed
    // (race, bug, etc.).
    expect(() =>
      buildRelationsPatch(
        pending({
          ['blocks' + INCOMING_SUFFIX]: card({
            entries: [{ id: 'T-1', type: 'ticket' }],
            added: [{ targetId: 'T-1' }],
          }),
        }),
        new Map() // empty: no inverse known
      )
    ).toThrow(/no inverse declared/)
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
        })
      )
    ).toThrow(/missing 'type'/)
  })
})

describe('reshapeLegacyToModern', () => {
  it('returns null when ANY id has no type — caller falls back to legacy', () => {
    const result = reshapeLegacyToModern(
      { 'depends-on': ['T-1', 'C-unknown'] },
      { 'depends-on': new Map([['T-1', 'ticket']]) }
    )
    expect(result).toBeNull()
  })

  it('adds the ids the form did not load', () => {
    const result = reshapeLegacyToModern(
      { 'depends-on': ['T-1', 'BUG-1'] },
      {
        'depends-on': new Map([
          ['T-1', 'ticket'],
          ['BUG-1', 'bug'],
        ]),
      }
    )
    expect(result).toEqual({
      'depends-on': {
        add: [
          { type: 'ticket', id: 'T-1' },
          { type: 'bug', id: 'BUG-1' },
        ],
      },
    })
  })

  it('clearing a loaded list removes each loaded edge', () => {
    const result = reshapeLegacyToModern(
      { tagged: [] },
      { tagged: new Map([['L-1', 'label']]) },
      new Map(),
      { tagged: ['L-1', 'L-2'] }
    )
    expect(result).toEqual({
      tagged: { remove: [{ type: 'label', id: 'L-1' }, { id: 'L-2' }] },
    })
  })

  it('omits an unchanged list', () => {
    const result = reshapeLegacyToModern({ tagged: ['L-1'] }, { tagged: new Map() }, new Map(), {
      tagged: ['L-1'],
    })
    expect(result).toEqual({})
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
      }
    )
    const upd = result?.['depends-on']
    expect(upd && 'add' in upd ? upd.add?.map((d) => d.type) : []).toEqual([
      'ticket',
      'bug',
      'feature',
      'doc-task',
    ])
  })
})

// A card state whose `added` is populated but whose `entries` is empty emits
// nothing for that edge: `entries` is the desired end state, and the delta is
// computed from it.
//
// This is not a hypothetical. The duplicate prefill (TKT-Z8K2FS) first
// populated only `added`, and the result was a create whose relations body said
// "this relation should have no edges". Every unit test passed; the copy simply
// arrived without its relations and nothing errored. Pinned here because the
// asymmetry is invisible at the call site.
describe('buildRelationsPatch entries/added asymmetry', () => {
  it('emits no add when added is set but entries is not', () => {
    const pending = new Map<string, RelationCardState>([
      [`blocks${OUTGOING_SUFFIX}`, card({ added: [{ targetId: 'FEAT-2' }] })],
    ])

    const out = buildRelationsPatchRaw(pending, new Map())

    expect(out.blocks).toBeUndefined()
  })

  it('emits the edge when entries carries it', () => {
    const pending = new Map<string, RelationCardState>([
      [
        `blocks${OUTGOING_SUFFIX}`,
        card({
          entries: [{ id: 'FEAT-2', type: 'feature' }],
          added: [{ targetId: 'FEAT-2' }],
        }),
      ],
    ])

    const out = buildRelationsPatchRaw(pending, new Map())

    expect(out.blocks).toEqual({ add: [{ type: 'feature', id: 'FEAT-2' }] })
  })
})

// Each body is a delta against the edges the server has confirmed, not
// against the loaded state, so a sequence of autosaves stays correct.
describe('confirmed edges across saves', () => {
  it('removes an edge an earlier save added', () => {
    const confirmed: ConfirmedEdges = new Map()
    const key = `tagged${OUTGOING_SUFFIX}`
    const first = buildRelationsPatchRaw(
      new Map([
        [key, card({ entries: [{ id: 'L-1', type: 'label' }], added: [{ targetId: 'L-1' }] })],
      ]),
      new Map(),
      confirmed
    )
    expect(first).toEqual({ tagged: { add: [{ type: 'label', id: 'L-1' }] } })
    confirmRelations(confirmed, first)

    // The widget dropped L-1 from `added` and `entries`: in its eyes it never
    // existed, but the server now holds it.
    const second = buildRelationsPatchRaw(new Map([[key, card()]]), new Map(), confirmed)
    expect(second).toEqual({ tagged: { remove: [{ type: 'label', id: 'L-1' }] } })
  })

  it('does not resend a confirmed add', () => {
    const confirmed: ConfirmedEdges = new Map()
    const state = new Map([
      [
        `tagged${OUTGOING_SUFFIX}`,
        card({ entries: [{ id: 'L-1', type: 'label' }], added: [{ targetId: 'L-1' }] }),
      ],
    ])
    confirmRelations(confirmed, buildRelationsPatchRaw(state, new Map(), confirmed))
    expect(buildRelationsPatchRaw(state, new Map(), confirmed)).toEqual({})
  })

  it('a body that was not confirmed is sent again', () => {
    const confirmed: ConfirmedEdges = new Map()
    const legacy = { tagged: ['L-1'] }
    const types = { tagged: new Map([['L-1', 'label']]) }
    const first = reshapeLegacyToModern(legacy, types, confirmed, {})
    // The save failed: nothing is confirmed, so the retry carries it again.
    expect(reshapeLegacyToModern(legacy, types, confirmed, {})).toEqual(first)
  })
})

describe('mergeRelationsFields', () => {
  const feat1 = { type: 'feature', id: 'FEAT-1' }
  const feat2 = { type: 'feature', id: 'FEAT-2' }

  it('keeps both deltas when the two bodies share a key', () => {
    const merged = mergeRelationsFields(
      { contained_in: { add: [feat1] } },
      { contained_in: { add: [feat2], remove: [{ id: 'FEAT-3' }] } }
    )
    expect(merged.contained_in).toEqual({ add: [feat1, feat2], remove: [{ id: 'FEAT-3' }] })
  })

  it('keeps keys only one side names', () => {
    const merged = mergeRelationsFields({ a: { add: [feat1] } }, { b: { add: [feat2] } })
    expect(merged).toEqual({ a: { add: [feat1] }, b: { add: [feat2] } })
  })

  it('lets a full replacement win', () => {
    const merged = mergeRelationsFields({ a: { add: [feat1] } }, { a: { data: [feat2] } })
    expect(merged.a).toEqual({ data: [feat2] })
  })
})
