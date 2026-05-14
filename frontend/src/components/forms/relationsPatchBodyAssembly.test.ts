// End-to-end assertions about how DynamicForm.handleSubmit composes
// `relations` for the unified PATCH body. These cases mirror the
// `relationsPayload` selection in DynamicForm — buildRelationsPatch
// produces card edits, reshapeLegacyToModern handles the
// mixed-shape constraint, and the fallback path keeps legacy when
// any picker target's type is unresolved (TKT-ZEKO4 AC7, AC8).

import { describe, it, expect } from 'vitest'
import {
  buildRelationsPatch,
  reshapeLegacyToModern,
  OUTGOING_SUFFIX,
  type RelationCardState,
} from './relationsPatch'

function card(overrides: Partial<RelationCardState> = {}): RelationCardState {
  return { entries: [], added: [], removed: [], updated: [], ...overrides }
}

function assemble(
  pending: Map<string, RelationCardState>,
  legacy: Record<string, string[]>,
  pickerTypes: Record<string, Map<string, string>>,
): { shape: 'modern' | 'legacy' | 'mixed-blocked'; body: unknown } {
  const modern = buildRelationsPatch(pending)
  const hasModern = Object.keys(modern).length > 0
  if (!hasModern) return { shape: 'legacy', body: legacy }
  const reshaped = reshapeLegacyToModern(legacy, pickerTypes)
  if (!reshaped) return { shape: 'mixed-blocked', body: legacy }
  return { shape: 'modern', body: { ...reshaped, ...modern } }
}

describe('relations body assembly', () => {
  it('AC7: no card edits → legacy shape unchanged', () => {
    const result = assemble(
      new Map(),
      { 'depends-on': ['T-1'] },
      { 'depends-on': new Map([['T-1', 'ticket']]) },
    )
    expect(result.shape).toBe('legacy')
    expect(result.body).toEqual({ 'depends-on': ['T-1'] })
  })

  it('AC7: card edits + picker with all types known → all modern', () => {
    const pending = new Map<string, RelationCardState>([
      [
        'tagged' + OUTGOING_SUFFIX,
        card({
          entries: [{ id: 'L-1', type: 'label' }],
          added: [{ targetId: 'L-1' }],
        }),
      ],
    ])
    const result = assemble(
      pending,
      { 'depends-on': ['T-1'] },
      { 'depends-on': new Map([['T-1', 'ticket']]) },
    )
    expect(result.shape).toBe('modern')
    expect(result.body).toEqual({
      'depends-on': { data: [{ type: 'ticket', id: 'T-1' }] },
      tagged: { data: [{ type: 'label', id: 'L-1' }] },
    })
  })

  it('AC8: card edits + picker with missing type → mixed-blocked', () => {
    const pending = new Map<string, RelationCardState>([
      [
        'tagged' + OUTGOING_SUFFIX,
        card({
          entries: [{ id: 'L-1', type: 'label' }],
          added: [{ targetId: 'L-1' }],
        }),
      ],
    ])
    const result = assemble(
      pending,
      { 'depends-on': ['T-1', 'C-unknown'] },
      { 'depends-on': new Map([['T-1', 'ticket']]) },
    )
    expect(result.shape).toBe('mixed-blocked')
  })

  it('card edits only, no other relations in body → modern, no reshape needed', () => {
    const pending = new Map<string, RelationCardState>([
      [
        'tagged' + OUTGOING_SUFFIX,
        card({
          entries: [{ id: 'L-1', type: 'label' }],
          added: [{ targetId: 'L-1' }],
        }),
      ],
    ])
    const result = assemble(pending, {}, {})
    expect(result.shape).toBe('modern')
    expect(result.body).toEqual({
      tagged: { data: [{ type: 'label', id: 'L-1' }] },
    })
  })

  it('polymorphic depends-on: types preserved per row, no to[0] guess', () => {
    const pending = new Map<string, RelationCardState>([
      [
        'tagged' + OUTGOING_SUFFIX,
        card({
          entries: [{ id: 'L-1', type: 'label' }],
          added: [{ targetId: 'L-1' }],
        }),
      ],
    ])
    const result = assemble(
      pending,
      { 'depends-on': ['T-1', 'BUG-1', 'FEAT-1'] },
      {
        'depends-on': new Map([
          ['T-1', 'ticket'],
          ['BUG-1', 'bug'],
          ['FEAT-1', 'feature'],
        ]),
      },
    )
    expect(result.shape).toBe('modern')
    const body = result.body as Record<string, { data: { type: string; id: string }[] }>
    expect(body['depends-on'].data.map((d) => d.type)).toEqual(['ticket', 'bug', 'feature'])
  })
})
