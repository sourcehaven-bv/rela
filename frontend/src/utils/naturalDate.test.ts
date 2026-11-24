import { describe, it, expect } from 'vitest'
import { parseNaturalDate } from './naturalDate'

// Tuesday 24 November 2026, local noon.
const now = new Date(2026, 10, 24, 12)

describe('parseNaturalDate', () => {
  it.each([
    ['2026-12-01', '2026-12-01'],
    ['today', '2026-11-24'],
    ['Vandaag', '2026-11-24'],
    ['tomorrow', '2026-11-25'],
    ['morgen', '2026-11-25'],
    ['overmorgen', '2026-11-26'],
    ['yesterday', '2026-11-23'],
    ['gisteren', '2026-11-23'],
    ['friday', '2026-11-27'],
    ['vrijdag', '2026-11-27'],
    ['tuesday', '2026-11-24'],
    ['next tuesday', '2026-12-01'],
    ['volgende dinsdag', '2026-12-01'],
    ['last friday', '2026-11-20'],
    ['vorige vrijdag', '2026-11-20'],
    ['in 3 days', '2026-11-27'],
    ['over 2 weken', '2026-12-08'],
    ['+3d', '2026-11-27'],
    ['-1w', '2026-11-17'],
    ['3 days ago', '2026-11-21'],
    ['3 dagen geleden', '2026-11-21'],
    ['in 1 month', '2026-12-24'],
    ['next week', '2026-12-01'],
    ['volgende maand', '2026-12-24'],
    ['5 march', '2027-03-05'],
    ['5 maart', '2027-03-05'],
    ['5 mrt 2026', '2026-03-05'],
    ['march 5', '2027-03-05'],
    ['december 1', '2026-12-01'],
    ['1 dec', '2026-12-01'],
    ['24/11', '2026-11-24'],
    ['5-3-2027', '2027-03-05'],
    ['end of month', '2026-11-30'],
    ['einde van de maand', '2026-11-30'],
  ])('%s → %s', (text, want) => {
    expect(parseNaturalDate(text, now)).toBe(want)
  })

  it.each(['', 'soon', '31 february', '2026-02-30', 'in 3 lightyears', '32/1', 'friday 13'])(
    'refuses %j',
    (text) => {
      expect(parseNaturalDate(text, now)).toBeNull()
    }
  )

  it('clamps a month offset to the last day', () => {
    expect(parseNaturalDate('in 1 month', new Date(2027, 0, 31))).toBe('2027-02-28')
  })
})

describe('parseNaturalDate review fixes', () => {
  it.each([
    ['29 feb', '2028-02-29'],
    ['komende week', '2026-12-01'],
    ['+3', '2026-11-27'],
    ['-1', '2026-11-23'],
    ['5/3/27', '2027-03-05'],
    ['5-3-2027', '2027-03-05'],
  ])('%s', (text, want) => {
    expect(parseNaturalDate(text, now)).toBe(want)
  })
  it.each(['over 9999 years', '26-11-24', 'x'.repeat(65), '29 feb 2027'])('rejects %s', (text) => {
    expect(parseNaturalDate(text, now)).toBeNull()
  })
})
