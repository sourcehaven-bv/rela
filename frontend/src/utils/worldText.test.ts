import { describe, it, expect } from 'vitest'
import { worldText } from './worldText'

describe('worldText', () => {
  it('renders nothing for an undeclared template', () => {
    expect(worldText(undefined, { face: 'Vastgesteld' })).toBe('')
    expect(worldText('', { face: 'Vastgesteld' })).toBe('')
  })

  it('substitutes the allowlisted placeholders', () => {
    expect(
      worldText('Je kijkt naar {face} van {title}; bewerken doe je in {bare_face} ({world}).', {
        face: 'Vastgesteld',
        bare_face: 'Concept',
        world: 'actueel',
        title: 'Toegangsbeleid',
      }),
    ).toBe('Je kijkt naar Vastgesteld van Toegangsbeleid; bewerken doe je in Concept (actueel).')
  })

  it('leaves an unknown placeholder as written', () => {
    // A typo in the operator's text should show up on screen, not vanish.
    expect(worldText('{fase} is klaar', { face: 'x' })).toBe('{fase} is klaar')
  })

  it('substitutes every occurrence', () => {
    expect(worldText('{face}/{face}', { face: 'nl' })).toBe('nl/nl')
  })
})
