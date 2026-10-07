import { describe, it, expect } from 'vitest'
import { anchorSections } from './rowAnchors'
import type { ViewSection } from '@/api/views'

function section(sectionId: string, over: Partial<ViewSection>): ViewSection {
  return {
    heading: sectionId,
    sectionId,
    display: 'list',
    isEmpty: false,
    isGrouped: false,
    hasContent: false,
    ...over,
  }
}

describe('anchorSections', () => {
  it('gives each entity to the first section that shows it', () => {
    const got = anchorSections([
      section('steps', { entities: [{ id: 'S-1', type: 'step', title: 'a', hasContent: false }] }),
      section('table', { rows: [{ entityId: 'S-1' }, { entityId: 'S-2' }] as never }),
      section('tree', {
        tree: [
          {
            entity: { id: 'S-3', type: 'step', title: 'c', hasContent: false },
            children: [{ entity: { id: 'S-2', type: 'step', title: 'b', hasContent: false } }],
          },
        ] as never,
      }),
    ])
    expect(Object.fromEntries(got)).toEqual({ 'S-1': 'steps', 'S-2': 'table', 'S-3': 'tree' })
  })
})
