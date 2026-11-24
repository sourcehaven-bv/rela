import { describe, expect, it } from 'vitest'
import { DATE_PACKS, fold, packsFor, parseNaturalDate } from '.'
import type { DatePack } from '.'

// Wednesday 25 November 2026, local time.
const TODAY = new Date(2026, 10, 25)

function parse(input: string, packs: DatePack[]) {
  return parseNaturalDate(input, TODAY, packs)
}

// Every pack must resolve every shape. The phrases are built from the pack's
// own words, so a new pack is covered by listing it in locales/index.ts.
describe.each(Object.values(DATE_PACKS))('shapes for pack $code', (pack) => {
  const p = [pack]
  const fri = pack.weekdays[4][0]
  const wed = pack.weekdays[2][0]
  const unit = (u: keyof DatePack['units'], plural = false) => pack.units[u][plural ? 1 : 0]

  it.each([
    ['ISO', '2027-01-31', '2027-01-31'],
    ['weekday', fri, '2026-11-27'],
    ['this weekday is today', `${pack.this[0]} ${wed}`, '2026-11-25'],
    ['next weekday on the same day', `${pack.next[0]} ${wed}`, '2026-12-02'],
    ['last weekday', `${pack.last[0]} ${fri}`, '2026-11-20'],
    ['last weekday on the same day', `${pack.last[0]} ${wed}`, '2026-11-18'],
    ['in N days', `${pack.in[0]} 3 ${unit('day', true)}`, '2026-11-28'],
    ['N weeks', `2 ${unit('week', true)}`, '2026-12-09'],
    ['N days ago', `3 ${unit('day', true)} ${pack.ago[0]}`, '2026-11-22'],
    ['next week', `${pack.next[0]} ${unit('week')}`, '2026-12-02'],
    ['last month', `${pack.last[0]} ${unit('month')}`, '2026-10-25'],
    ['next year', `${pack.next[0]} ${unit('year')}`, '2027-11-25'],
    ['day month, upcoming', `5 ${pack.months[11][0]}`, '2026-12-05'],
    ['day month, passed', `5 ${pack.months[2][0]}`, '2027-03-05'],
    ['day month year', `5 ${pack.months[2][0]} 2030`, '2030-03-05'],
    ['month day', `${pack.months[0][0]} 2`, '2027-01-02'],
    ['case and spaces', `  ${fri.toUpperCase()}  `, '2026-11-27'],
  ])('%s: %s', (_name, input, want) => {
    expect(parse(input, p)).toBe(want)
  })

  it('maps every relative day phrase to its offset', () => {
    for (const [phrase, offset] of Object.entries(pack.relativeDays)) {
      const want = new Date(Date.UTC(2026, 10, 25 + offset)).toISOString().slice(0, 10)
      expect(parse(phrase, p), phrase).toBe(want)
    }
  })

  it.each([
    '',
    '   ',
    `31 ${pack.months[1][0]} 2027`,
    `${pack.in[0]} -3 ${unit('day', true)}`,
    `${pack.in[0]} 9999 ${unit('day', true)}`,
    '2026-02-30',
    'x'.repeat(65),
    'zzz qqq',
  ])('rejects %j', (input) => {
    expect(parse(input, p)).toBeNull()
  })

  it('writes every word in folded form, or it could never match', () => {
    const words = [
      ...Object.keys(pack.relativeDays),
      ...pack.weekdays.flat(),
      ...pack.months.flat(),
      ...pack.this,
      ...pack.next,
      ...pack.last,
      ...pack.in,
      ...pack.ago,
      ...Object.values(pack.units).flat(),
      ...Object.keys(pack.numbers ?? {}),
      ...(pack.fillers ?? []),
    ]
    for (const w of words) expect(fold(w), w).toBe(w)
  })

  it('has seven weekdays and twelve months', () => {
    expect(pack.weekdays).toHaveLength(7)
    expect(pack.months).toHaveLength(12)
  })
})

describe('language-specific phrases', () => {
  it.each([
    ['en', 'tomorrow', '2026-11-26'],
    ['en', 'the day after tomorrow', '2026-11-27'],
    ['en', 'in a week', '2026-12-02'],
    ['en', 'march 5th', '2027-03-05'],
    ['en', 'next fri', '2026-11-27'],
    ['nl', 'morgen', '2026-11-26'],
    ['nl', 'overmorgen', '2026-11-27'],
    ['nl', 'volgende week', '2026-12-02'],
    ['nl', 'over twee weken', '2026-12-09'],
    ['nl', 'op vrijdag', '2026-11-27'],
    ['nl', '5 maart', '2027-03-05'],
    ['nl', '1e januari', '2027-01-01'],
    ['nl', '1ste mei', '2027-05-01'],
    ['nl', '2de juni', '2027-06-02'],
    ['nl', 'in 3 dagen', '2026-11-28'],
    ['nl', 'acht dagen geleden', '2026-11-17'],
    ['en', 'eight days ago', '2026-11-17'],
  ])('%s: %s', (lang, input, want) => {
    expect(parse(input, packsFor(lang))).toBe(want)
  })

  it('folds diacritics in input', () => {
    expect(parse('Fríday', packsFor('en'))).toBe('2026-11-27')
  })

  it('falls back to English for another UI language', () => {
    expect(parse('tomorrow', packsFor('nl'))).toBe('2026-11-26')
    expect(parse('tomorrow', packsFor('fr'))).toBe('2026-11-26')
    expect(parse('morgen', packsFor('en'))).toBeNull()
  })

  it('accepts a region subtag', () => {
    expect(packsFor('nl-BE')[0].code).toBe('nl')
  })
})

describe('calendar edges', () => {
  it('clamps a month step to the last day', () => {
    expect(parseNaturalDate('next month', new Date(2027, 0, 31), packsFor('en'))).toBe('2027-02-28')
  })

  it('skips 29 February to the next leap year', () => {
    expect(parseNaturalDate('29 february', new Date(2026, 10, 25), packsFor('en'))).toBe(
      '2028-02-29'
    )
  })

  it('rolls over the year for a weekday', () => {
    expect(parseNaturalDate('friday', new Date(2026, 11, 31), packsFor('en'))).toBe('2027-01-01')
  })
})

// A pack that is neither English nor Dutch: modifiers after the word, a
// multi-word direction before the count, and an article to drop. It needs no
// parser change, which is the point of the pack design.
const fr: DatePack = {
  code: 'fr',
  relativeDays: { demain: 1, "aujourd'hui": 0 },
  weekdays: [['lundi'], ['mardi'], ['mercredi'], ['jeudi'], ['vendredi'], ['samedi'], ['dimanche']],
  months: [
    ['janvier'],
    ['fevrier'],
    ['mars'],
    ['avril'],
    ['mai'],
    ['juin'],
    ['juillet'],
    ['aout'],
    ['septembre'],
    ['octobre'],
    ['novembre'],
    ['decembre'],
  ],
  this: ['ce'],
  next: ['prochain', 'prochaine'],
  last: ['dernier', 'derniere'],
  in: ['dans'],
  ago: ['il y a'],
  units: {
    day: ['jour', 'jours'],
    week: ['semaine', 'semaines'],
    month: ['mois'],
    year: ['an', 'ans'],
  },
  fillers: ['la', 'le'],
  ordinals: ['er'],
}

// A pack for a script written without spaces. Its tokenizer splits on the
// pack's own words; the shapes are the parser's.
const nospace: DatePack = {
  code: 'xx',
  relativeDays: { tmrw: 1 },
  weekdays: [['mo'], ['tu'], ['we'], ['th'], ['fr'], ['sa'], ['su']],
  months: Array.from({ length: 12 }, (_, i) => [`m${i + 1}`]),
  this: [],
  next: ['nx'],
  last: [],
  in: [],
  ago: [],
  units: { day: ['d'], week: ['w'], month: ['mo_'], year: ['y'] },
  tokenize: (folded) => folded.match(/\d+|nx|fr|w/g) ?? [],
}

describe('a pack for another word order', () => {
  it.each([
    ['vendredi prochain', '2026-11-27'],
    ['la semaine prochaine', '2026-12-02'],
    ['il y a 3 jours', '2026-11-22'],
    ['dans 2 semaines', '2026-12-09'],
    ['1er mars', '2027-03-01'],
    ['Février 2', '2027-02-02'],
    ["aujourd'hui", '2026-11-25'],
  ])('%s', (input, want) => {
    expect(parse(input, [fr])).toBe(want)
  })

  it('takes a tokenizer for a language without spaces', () => {
    expect(parse('nxfr', [nospace])).toBe('2026-11-27')
    expect(parse('2w', [nospace])).toBe('2026-12-09')
  })
})
