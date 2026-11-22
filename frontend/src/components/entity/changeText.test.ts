import { describe, it, expect } from 'vitest'

import {
  actorText,
  opText,
  subjectText,
  summaryText,
  triggerText,
  type ChangeSchema,
} from './changeText'
import type { ChangeRecord } from '@/api/audit'
import type { EntityType, RelationType } from '@/types'

const schema: ChangeSchema = {
  entityType: (name) =>
    (
      ({
        ticket: { label: 'Ticket', properties: { status: { type: 'string', label: 'Status' } } },
        bug: { label: 'Bug' },
      }) as unknown as Record<string, EntityType>
    )[name],
  relationType: (name) =>
    (
      ({
        affects: {
          label: 'Affects',
          from: [],
          to: [],
          inverse: { id: 'affected-by', label: 'Affected by' },
        },
        blocks: { label: 'Blocks', from: [], to: [] },
        tracks: { label: 'Tracks', from: [], to: [], inverse: { id: 'tracked-by' } },
      }) as Record<string, RelationType>
    )[name],
}

function rec(over: Partial<ChangeRecord>): ChangeRecord {
  return { seq: 1, time: '2026-01-01T00:00:00Z', op: 'update-entity', ...over }
}

const rel = (type: string, from: string, to: string) =>
  rec({
    op: 'create-relation',
    subject: { kind: 'relation', relation_type: type, from_id: from, to_id: to },
  })

describe('changeText', () => {
  it.each([
    ['create-entity', 'Created'],
    ['delete-relation', 'Unlinked'],
    ['some-new-op', 'some-new-op'],
  ])('names op %s as %s', (op, want) => {
    expect(opText(op)).toBe(want)
  })

  it.each([
    [{ user: 'alice' }, 'alice'],
    [{ user: 'system:scheduler' }, 'System'],
    [{ user: 'integration:crm' }, 'Integration crm'],
    [{ user: 'webhook:membership.created' }, 'Identity provider (membership.created)'],
    [{ user: 'system:webhook:hub' }, 'Webhook hub'],
    [{ user: 'unknown', tool: 'mcp' }, 'System'],
    [{ tool: 'cli' }, 'System'],
    [{ user: 'PSN-19' }, 'Ada Lovelace'],
    [{ user: 'PSN-20' }, 'PSN-20'],
  ])('names actor %o as %s', (over, want) => {
    expect(actorText(rec(over), { 'PSN-19': 'Ada Lovelace' })).toBe(want)
  })

  it.each([
    [
      'an outgoing relation by its label and the title',
      rel('affects', 'TKT-1', 'CON-7'),
      'Affects → Login',
    ],
    [
      'an incoming relation by its inverse label',
      rel('affects', 'BUG-3', 'TKT-1'),
      'Affected by → BUG-3',
    ],
    [
      'an incoming relation without an inverse as from its source',
      rel('blocks', 'BUG-3', 'TKT-1'),
      'Blocks, from BUG-3',
    ],
    [
      'an incoming relation whose inverse has no label as from its source',
      rel('tracks', 'CON-7', 'TKT-1'),
      'Tracks, from Login',
    ],
    ['an unknown relation type by its name', rel('refs', 'TKT-1', 'CON-7'), 'refs → Login'],
    [
      'a conversion by the type labels',
      rec({
        op: 'convert-entity',
        before: { kind: 'entity', type: 'bug', id: 'BUG-1' },
        after: { kind: 'entity', type: 'ticket', id: 'TKT-1' },
      }),
      'Bug → Ticket',
    ],
    [
      'a rename by the ids',
      rec({
        op: 'rename-entity',
        before: { kind: 'entity', type: 'ticket', id: 'TKT-0' },
        after: { kind: 'entity', type: 'ticket', id: 'TKT-1' },
      }),
      'TKT-0 → TKT-1',
    ],
    [
      'a change of the entity itself not at all',
      rec({ subject: { kind: 'entity', type: 'ticket', id: 'TKT-1' } }),
      '',
    ],
  ])('names %s', (_name, c, want) => {
    expect(subjectText(c, 'TKT-1', { 'CON-7': 'Login' }, schema)).toBe(want)
  })

  it.each([
    ['updated: status,owner', 'Changed: Status, owner'],
    ['updated', ''],
    ['created', ''],
  ])('summarises %s as %s', (summary, want) => {
    expect(
      summaryText(
        rec({ summary, subject: { kind: 'entity', type: 'ticket', id: 'TKT-1' } }),
        schema
      )
    ).toBe(want)
  })

  it.each([
    ['automation:close-on-done', 'By an automation'],
    ['schedule:nightly', 'By a schedule'],
    ['cascade', 'Automatically'],
    [undefined, ''],
  ])('names trigger %s as %s', (trigger, want) => {
    expect(triggerText(rec({ trigger }))).toBe(want)
  })
})
