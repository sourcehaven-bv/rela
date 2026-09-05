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

describe('worldText: what a missing value means', () => {
  it('leaves the placeholder as written when the surface has no such fact', () => {
    // A list has no single title; `{title}` in a projection note is the
    // operator asking for something this surface cannot say, so it shows.
    expect(worldText('{title} in {world}', { world: 'actueel' })).toBe('{title} in actueel')
  })

  it('substitutes an EMPTY value to nothing', () => {
    // The fact exists and is empty — a face with no declared label. Printing
    // `{face}` there would put a rela word on screen.
    expect(worldText('Nog {face}.', { face: '' })).toBe('Nog .')
  })

  it('never re-scans a substituted value', () => {
    // A title that happens to contain a placeholder is data, not template.
    expect(worldText('{title}', { title: '{world}', world: 'x' })).toBe('{world}')
  })
})
