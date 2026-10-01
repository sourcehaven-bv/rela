import { describe, it, expect } from 'vitest'
import { mergeFamilyCandidates, widenWorlds } from './familyCandidates'
import type { Entity, WorldInfo } from '@/types'

function row(id: string, face = ''): Entity {
  return {
    id,
    type: 'policy',
    properties: {},
    _self: `/api/v1/policies/${face ? `${id}@${face}` : id}`,
  }
}

function worlds(entries: Record<string, boolean>): Map<string, WorldInfo> {
  return new Map(
    Object.entries(entries).map(([name, readable]) => [name, { readable } as WorldInfo])
  )
}

describe('widenWorlds', () => {
  it('names the other readable worlds, sorted, without default or the ambient one', () => {
    const w = worlds({
      published: true,
      editorial: true,
      default: true,
      secret: false,
      archive: true,
    })
    expect(widenWorlds(w, 'published')).toEqual(['archive', 'editorial'])
  })

  it('keeps a declared default world, which serves faces like any other', () => {
    const w = worlds({ published: true, base: true })
    w.set('base', { readable: true, default: true } as WorldInfo)
    expect(widenWorlds(w, 'published')).toEqual(['base'])
  })

  it('names every readable declared world when the ambient world is the default', () => {
    expect(widenWorlds(worlds({ published: true, editorial: true }), 'default')).toEqual([
      'editorial',
      'published',
    ])
  })
})

describe('mergeFamilyCandidates', () => {
  it('keeps the ambient row for an entity both worlds serve', () => {
    const { rows, offWorld } = mergeFamilyCandidates(
      [row('POL-1', 'published')],
      [[row('POL-1', 'draft'), row('POL-2', 'draft')]]
    )
    expect(rows.map((r) => r._self)).toEqual([
      '/api/v1/policies/POL-1@published',
      '/api/v1/policies/POL-2@draft',
    ])
    expect([...offWorld]).toEqual(['POL-2'])
  })

  it('takes an off-world row from the first world that serves it', () => {
    const { rows } = mergeFamilyCandidates([], [[row('POL-2', 'draft')], [row('POL-2', 'review')]])
    expect(rows.map((r) => r._self)).toEqual(['/api/v1/policies/POL-2@draft'])
  })

  it('marks nothing off-world when the ambient world serves everything', () => {
    const { rows, offWorld } = mergeFamilyCandidates([row('CTL-1')], [])
    expect(rows).toHaveLength(1)
    expect(offWorld.size).toBe(0)
  })
})
