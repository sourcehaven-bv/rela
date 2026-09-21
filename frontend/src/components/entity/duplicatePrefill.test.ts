import { describe, it, expect } from 'vitest'
import {
  buildDuplicatePrefill,
  relationChoices,
  defaultSelection,
} from './duplicatePrefill'
import type { Entity, RelationEntry } from '@/types/entity'
import type { EntityType } from '@/types/schema'

function entity(over: Partial<Entity> = {}): Entity {
  return {
    id: 'TKT-001',
    type: 'ticket',
    properties: { title: 'Original' },
    ...over,
  } as Entity
}

const ticketType = {
  label: 'Ticket',
  properties: {
    title: { type: 'string' },
    count: { type: 'integer' },
    done: { type: 'boolean' },
    due: { type: 'date' },
    tags: { type: 'string', list: true },
    status: { type: 'enum', values: ['open', 'closed'] },
    screenshot: { type: 'file' },
  },
} as unknown as EntityType

describe('buildDuplicatePrefill', () => {
  // AC15: the failure mode that ruled out the URL transport. A list property
  // must arrive as an array, a number as a number, a boolean as a boolean.
  it('carries non-string property types with their values intact', () => {
    const src = entity({
      properties: {
        title: 'Original',
        count: 42,
        done: false,
        due: '2026-01-01',
        tags: ['a', 'b'],
      },
    })
    const p = buildDuplicatePrefill(src, ticketType, {}, [], undefined)
    expect(p.properties.count).toBe(42)
    expect(p.properties.done).toBe(false)
    expect(p.properties.due).toBe('2026-01-01')
    expect(p.properties.tags).toEqual(['a', 'b'])
    // `false` must survive: a truthiness filter would drop it.
    expect(Object.keys(p.properties)).toContain('done')
  })

  // AC16: create mode has no content prefill channel today, so this is new.
  it('carries the markdown content', () => {
    const p = buildDuplicatePrefill(
      entity({ content: '# Body\n\ntext' }),
      ticketType,
      {},
      [],
      undefined
    )
    expect(p.content).toBe('# Body\n\ntext')
  })

  it('defaults content to empty string when the source has none', () => {
    expect(buildDuplicatePrefill(entity(), ticketType, {}, [], undefined).content).toBe('')
  })

  // AC10b: a create is an ENTRY, not a transition. Carrying the source's
  // status would be silently replaced by the server's entry value.
  it('never carries a state-machine property, and reports it', () => {
    const src = entity({
      properties: { title: 'Original', status: 'closed' },
      _transitions: { status: [] },
    })
    const p = buildDuplicatePrefill(src, ticketType, {}, [], undefined)
    expect(p.properties).not.toHaveProperty('status')
    expect(p.omitted).toContainEqual({ property: 'status', reason: 'state-machine' })
  })

  // A plain enum has no _transitions key, so it must not be over-excluded.
  it('carries a plain enum that is not a state machine', () => {
    const src = entity({ properties: { title: 'Original', status: 'open' } })
    const p = buildDuplicatePrefill(src, ticketType, {}, [], undefined)
    expect(p.properties.status).toBe('open')
  })

  // AC19b: attachments live under the SOURCE's id, so the filename would be a
  // dangling path on the copy.
  it('never carries a file property, and reports it', () => {
    const src = entity({ properties: { title: 'Original', screenshot: 'shot.png' } })
    const p = buildDuplicatePrefill(src, ticketType, {}, [], undefined)
    expect(p.properties).not.toHaveProperty('screenshot')
    expect(p.omitted).toContainEqual({ property: 'screenshot', reason: 'file' })
  })

  // AC10a: redaction is reported, never silently short-copied. Absence alone
  // cannot be used to detect it — an unset property is absent too.
  it('reports redacted properties', () => {
    const src = entity({ properties: { title: 'Original' }, _redacted: ['salary'] })
    const p = buildDuplicatePrefill(src, ticketType, {}, [], undefined)
    expect(p.omitted).toContainEqual({ property: 'salary', reason: 'redacted' })
  })

  // AC9: the operator allowlist narrows; unnamed properties are omitted rather
  // than blanked, so they fall back to metamodel/template defaults.
  it('honours a duplicate.properties allowlist', () => {
    const src = entity({ properties: { title: 'Original', count: 7, due: '2026-01-01' } })
    const p = buildDuplicatePrefill(src, ticketType, {}, [], {
      properties: ['title', 'count'],
    })
    expect(Object.keys(p.properties).sort()).toEqual(['count', 'title'])
    expect(p.omitted).toContainEqual({ property: 'due', reason: 'not-configured' })
  })

  it('carries everything when no allowlist is configured', () => {
    const src = entity({ properties: { title: 'Original', count: 7 } })
    const p = buildDuplicatePrefill(src, ticketType, {}, [], undefined)
    expect(Object.keys(p.properties).sort()).toEqual(['count', 'title'])
  })

  describe('relations', () => {
    const rels: Record<string, RelationEntry[]> = {
      'belongs-to': [{ id: 'CAT-1', type: 'category', direction: 'outgoing' }],
      blocks: [
        { id: 'TKT-2', type: 'ticket', direction: 'outgoing' },
        { id: 'TKT-3', type: 'ticket', direction: 'outgoing' },
      ],
      blocked_by: [{ id: 'TKT-9', type: 'ticket', direction: 'incoming' }],
    }

    // AC6: selected types carry every visible peer.
    it('carries peers of selected keys', () => {
      const p = buildDuplicatePrefill(entity(), ticketType, rels, ['blocks'], undefined)
      expect(p.relations).toEqual({
        blocks: [
          { id: 'TKT-2', type: 'ticket' },
          { id: 'TKT-3', type: 'ticket' },
        ],
      })
    })

    // AC7: unchecking a type means none of its edges exist on the copy.
    it('omits unselected keys entirely', () => {
      const p = buildDuplicatePrefill(entity(), ticketType, rels, ['blocks'], undefined)
      expect(p.relations).not.toHaveProperty('belongs-to')
      expect(p.relations).not.toHaveProperty('blocked_by')
    })

    // Incoming keys pass through under their INVERSE name; the create body
    // resolves them server-side, so translating here would double-invert.
    it('passes an incoming inverse key through untranslated', () => {
      const p = buildDuplicatePrefill(entity(), ticketType, rels, ['blocked_by'], undefined)
      expect(p.relations).toEqual({ blocked_by: [{ id: 'TKT-9', type: 'ticket' }] })
    })

    it('selects nothing when no keys are chosen', () => {
      expect(buildDuplicatePrefill(entity(), ticketType, rels, [], undefined).relations).toEqual({})
    })
  })
})

describe('relationChoices', () => {
  // AC2: counts are derived per group, over VISIBLE peers only.
  it('reports one row per non-empty group with its edge count', () => {
    const choices = relationChoices({
      blocks: [
        { id: 'TKT-2', type: 'ticket', direction: 'outgoing' },
        { id: 'TKT-3', type: 'ticket', direction: 'outgoing' },
      ],
      blocked_by: [{ id: 'TKT-9', type: 'ticket', direction: 'incoming' }],
    })
    expect(choices).toEqual([
      { key: 'blocked_by', direction: 'incoming', count: 1 },
      { key: 'blocks', direction: 'outgoing', count: 2 },
    ])
  })

  // AC2: a type whose every peer is hidden arrives as an empty group and must
  // not render as a zero row — the user cannot act on it and it implies the
  // type genuinely has no edges.
  it('omits a group with no visible edges', () => {
    expect(relationChoices({ blocks: [] })).toEqual([])
  })

  it('returns nothing for an entity with no relations', () => {
    expect(relationChoices({})).toEqual([])
  })
})

describe('defaultSelection', () => {
  // AC3: outgoing checked, incoming unchecked.
  it('checks outgoing types only', () => {
    const choices = relationChoices({
      blocks: [{ id: 'TKT-2', type: 'ticket', direction: 'outgoing' }],
      blocked_by: [{ id: 'TKT-9', type: 'ticket', direction: 'incoming' }],
    })
    expect(defaultSelection(choices)).toEqual(['blocks'])
  })
})

describe('self-loops and untyped peers', () => {
  // A self-loop arrives TWICE — once per direction — because it matches the
  // filter at both endpoints. Emitting either key links the copy to the
  // SOURCE rather than to itself; emitting both makes them point at each
  // other. Reproducing it needs the copy's id, which does not exist yet.
  it('drops a self-loop and reports it once, though it arrives under two keys', () => {
    const p = buildDuplicatePrefill(
      entity(),
      ticketType,
      {
        blocks: [{ id: 'TKT-001', type: 'ticket', direction: 'outgoing' }],
        blocked_by: [{ id: 'TKT-001', type: 'ticket', direction: 'incoming' }],
      },
      ['blocks', 'blocked_by'],
      undefined
    )

    expect(p.relations).toEqual({})
    expect(p.omitted.filter((o) => o.reason === 'self-loop')).toHaveLength(1)
  })

  it('keeps other peers of a relation that also self-loops', () => {
    const p = buildDuplicatePrefill(
      entity(),
      ticketType,
      {
        blocks: [
          { id: 'TKT-001', type: 'ticket', direction: 'outgoing' },
          { id: 'TKT-2', type: 'ticket', direction: 'outgoing' },
        ],
      },
      ['blocks'],
      undefined
    )

    expect(p.relations).toEqual({ blocks: [{ id: 'TKT-2', type: 'ticket' }] })
  })

  // An edge is more consequential than a property, so dropping one silently is
  // worse than the thing this module exists to prevent.
  it('reports a peer whose type the server could not resolve', () => {
    const p = buildDuplicatePrefill(
      entity(),
      ticketType,
      { blocks: [{ id: 'TKT-2', type: '', direction: 'outgoing' }] },
      ['blocks'],
      undefined
    )

    expect(p.relations).toEqual({})
    expect(p.omitted).toContainEqual({ property: 'blocks → TKT-2', reason: 'untyped-peer' })
  })
})
