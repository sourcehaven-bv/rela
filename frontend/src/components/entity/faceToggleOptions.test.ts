import { describe, expect, it } from 'vitest'
import { faceToggleOptions } from './faceToggleOptions'

const declared = { concept: { label: 'Concept' }, vastgesteld: { label: 'Vastgesteld' }, en: {} }

describe('faceToggleOptions', () => {
  it('lists the face on screen and the others, in declaration order, by label', () => {
    const got = faceToggleOptions('VW-1', 'vastgesteld', [{ face: 'concept', ref: 'VW-1@concept' }], declared)
    expect(got).toEqual([
      { face: 'concept', label: 'Concept', ref: 'VW-1@concept' },
      { face: 'vastgesteld', label: 'Vastgesteld', ref: 'VW-1@vastgesteld' },
    ])
  })

  it('falls back to the declared name when a face has no label', () => {
    const got = faceToggleOptions('VW-1', 'concept', [{ face: 'en' }], declared)
    expect(got?.map((o) => [o.label, o.ref])).toEqual([
      ['Concept', 'VW-1@concept'],
      ['en', 'VW-1@en'],
    ])
  })

  it('applies to two or three faces only', () => {
    expect(faceToggleOptions('VW-1', 'concept', [], declared)).toBeNull()
    expect(
      faceToggleOptions('VW-1', 'concept', [{ face: 'a' }, { face: 'b' }, { face: 'c' }], declared),
    ).toBeNull()
    expect(faceToggleOptions('VW-1', 'concept', [{ face: 'a' }, { face: 'b' }], declared)).toHaveLength(3)
  })

  it('does not apply when a face has no stored name', () => {
    expect(faceToggleOptions('VW-1', '', [{ face: 'concept' }], declared)).toBeNull()
    expect(faceToggleOptions('VW-1', 'concept', [{ face: '' }], declared)).toBeNull()
  })
})
