import { describe, expect, it } from 'vitest'
import type { EntityType } from '@/types/schema'
import { entityTypeForId } from './entityIdType'

function def(prefix: { id_prefix?: string; id_prefixes?: string[] }): EntityType {
  return { label: 'x', properties: {}, ...prefix }
}

const types = new Map<string, EntityType>([
  ['feature', def({ id_prefix: 'FEAT' })],
  ['project', def({ id_prefix: 'PROJ-' })],
  ['task', def({ id_prefixes: ['TASK-'] })],
  ['decision', def({ id_prefixes: ['DEC-', 'ADR-'] })],
  ['spec', def({ id_prefix: 'S-' })],
  ['specification', def({ id_prefix: 'SPEC-' })],
  ['testcase', def({ id_prefix: 'TC' })],
  ['category', def({})],
])

describe('entityTypeForId', () => {
  it('matches a dashless id_prefix', () => {
    expect(entityTypeForId('FEAT-001', types)).toBe('feature')
  })

  it('matches an id_prefix that includes the dash', () => {
    expect(entityTypeForId('PROJ-7K2M', types)).toBe('project')
  })

  it('matches id_prefixes', () => {
    expect(entityTypeForId('TASK-7', types)).toBe('task')
    expect(entityTypeForId('ADR-3', types)).toBe('decision')
  })

  it('ignores case', () => {
    expect(entityTypeForId('proj-abc', types)).toBe('project')
  })

  it('prefers the longest matching prefix', () => {
    expect(entityTypeForId('SPEC-1', types)).toBe('specification')
    expect(entityTypeForId('S-1', types)).toBe('spec')
  })

  it('does not let a dashless prefix claim a longer word', () => {
    expect(entityTypeForId('TCX-1', types)).toBeUndefined()
    expect(entityTypeForId('TC-1', types)).toBe('testcase')
    expect(entityTypeForId('TC1', types)).toBe('testcase')
  })

  it('returns undefined for an id without a known prefix', () => {
    expect(entityTypeForId('backend', types)).toBeUndefined()
    expect(entityTypeForId('', types)).toBeUndefined()
  })
})
