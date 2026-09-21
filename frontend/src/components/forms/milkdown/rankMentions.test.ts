import { describe, it, expect } from 'vitest'
import { rankMentions, rankTypeNames } from './rankMentions'
import type { Entity } from '@/types'

/**
 * Builds a candidate in the shape `/_search` actually returns.
 *
 * The display name lives in `_title` — the metamodel-aware value the backend
 * computes and redacts — not in `properties.title`, which is only correct for
 * types whose `display_property` happens to be `title` (BUG-1P88YM). Putting
 * the title on the wrong field makes every haystack collapse to the bare ID,
 * so a title-matching test passes for the wrong reason.
 */
function ent(id: string, title?: string, type = 'ticket'): Entity {
  return { id, type, _title: title ?? id, properties: {} } as unknown as Entity
}

describe('rankMentions', () => {
  it('leaves the order alone for an empty query', () => {
    const items = [ent('B'), ent('A')]
    expect(rankMentions(items, '')).toBe(items)
  })

  it('puts an exact id match first, then prefix, then the rest', () => {
    const items = [ent('OTHER', 'TKT-AB mentioned here'), ent('TKT-ABCD'), ent('TKT-AB')]
    expect(rankMentions(items, 'TKT-AB').map((e) => e.id)).toEqual(['TKT-AB', 'TKT-ABCD', 'OTHER'])
  })

  it('ranks an id prefix above a loose title match', () => {
    const items = [ent('ZZZ-1', 'a ticket about tkt things'), ent('TKT-ABCD', 'unrelated title')]
    expect(rankMentions(items, 'TKT-AB')[0].id).toBe('TKT-ABCD')
  })

  it('matches a hyphenated query across title words', () => {
    const items = [ent('FR-002', 'Unrelated summary'), ent('FR-001', 'FancyReport ranking fix')]
    expect(rankMentions(items, 'fancy-ranking')[0].id).toBe('FR-001')
  })

  it('matches a run-together query against a camel-case title', () => {
    const items = [ent('X-1', 'Plain title'), ent('FR-001', 'FancyReport ranking fix')]
    expect(rankMentions(items, 'fancyreport').map((e) => e.id)).toEqual(['FR-001'])
  })

  it('orders title matches by quality rather than dumping them in one tier', () => {
    const items = [
      ent('A-1', 'a ranking system for something else entirely'),
      ent('A-2', 'ranking'),
    ]
    expect(rankMentions(items, 'ranking').map((e) => e.id)).toEqual(['A-2', 'A-1'])
  })

  it('returns an empty list when nothing matches, rather than everything', () => {
    const items = [ent('A-1', 'alpha'), ent('A-2', 'beta')]
    expect(rankMentions(items, 'zzzzqqq')).toEqual([])
  })

  it('never invents a row that was not in the response', () => {
    const items = [ent('A-1', 'alpha'), ent('A-2', 'beta')]
    for (const q of ['a', 'al', 'alpha', 'b', 'zzz', '---', '日本']) {
      const out = rankMentions(items, q)
      expect(items).toEqual(expect.arrayContaining(out))
      expect(out.length).toBeLessThanOrEqual(items.length)
    }
  })

  it('ranks an entity with no title by its id alone', () => {
    const untitled = { id: 'TKT-XYZ', type: 'ticket', properties: {} } as unknown as Entity
    expect(rankMentions([untitled], 'TKT-XY').map((e) => e.id)).toEqual(['TKT-XYZ'])
  })

  // RR-NNFWGP. The floor that shipped in the first plan rejected every prefix,
  // because uFuzzy counts `fancy-` as zero COMPLETE terms. A complete-word
  // fixture passes happily while the menu blanks on every real keystroke, so
  // the guard has to be a matrix over every prefix.
  describe('prefix matrix: typing toward a match never blanks the menu', () => {
    const target = ent('FR-001', 'FancyReport ranking fix')
    const items = [ent('OTHER-1', 'completely unrelated'), target]
    const query = 'fancy-ranking'

    for (let i = 1; i <= query.length; i++) {
      const prefix = query.slice(0, i)
      it(`keeps the target visible for ${JSON.stringify(prefix)}`, () => {
        expect(rankMentions(items, prefix).map((e) => e.id)).toContain('FR-001')
      })
    }
  })

  // uFuzzy's tokenizer silently discards what its Latin-oriented splitter does
  // not match, so a query it only PARTLY understands would otherwise be ranked
  // on the fragment — dropping rows the server matched on the discarded part.
  // Note this is not only a CJK problem: `café` splits to `["caf"]`.
  describe('a partly-tokenizable query passes the server order through', () => {
    const items = [
      ent('JP-1', '日本語のタイトル'),
      ent('AL-1', 'alpha'),
      ent('CA-1', 'café review'),
    ]

    for (const q of ['日本語-jp', 'jp-日本語', 'a-日本語', 'café', 'naïve', 'Ω-omega']) {
      it(`returns every row for ${JSON.stringify(q)}`, () => {
        expect(rankMentions(items, q)).toBe(items)
      })
    }

    it('still ranks a fully-tokenizable hyphenated query', () => {
      // The guard must not swallow the normal case: `fancy-rank` is fully
      // covered even though the hyphen itself is absent from the terms.
      const fixture = [ent('X-1', 'unrelated'), ent('FR-1', 'FancyReport ranking')]
      expect(rankMentions(fixture, 'fancy-ranking').map((e) => e.id)).toEqual(['FR-1'])
    })
  })

  describe('queries uFuzzy cannot tokenize pass the server order through', () => {
    // The server matched these rows; uFuzzy has no Latin terms to re-rank by.
    // Dropping them would report "No matches" over a good response (RR-66JTAL,
    // RR-9B2QSS) and an unguarded uf.info() would throw into the catch and
    // surface as a misleading "Search failed".
    const items = [ent('JP-1', '日本語のタイトル'), ent('RU-1', 'Заголовок')]

    for (const q of ['日本語', 'Заголовок', '---', '-', '...']) {
      it(`returns every row unchanged for ${JSON.stringify(q)}`, () => {
        expect(rankMentions(items, q)).toBe(items)
      })
    }
  })

  // The slice to MAX_RESULTS happens in the caller, AFTER this tier sort.
  // Slicing first would let an exact ID match beyond the cut be dropped before
  // the tier could promote it, so the order of those two steps is load-bearing.
  it('promotes an exact id match from the tail of a long response', () => {
    const rows = Array.from({ length: 30 }, (_, i) => ent(`Z-${i}`, `target thing ${i}`))
    rows.push(ent('TARGET', 'target thing last'))
    expect(rankMentions(rows, 'TARGET')[0].id).toBe('TARGET')
  })

  // A COST assertion, not a correctness one. `intraIns: 1` (which looks like a
  // free typo-tolerance win) makes matching exponential when a repeated-char run
  // in the needle meets one in the haystack: measured at 27s for a 24-char
  // needle and 55s for 64 chars, synchronously on the main thread. Every
  // correctness fixture passes either way, so only a timing bound catches it.
  // If this test hangs rather than fails, that IS the regression.
  it('matches a worst-case repeated-run needle in well under a second', () => {
    const rows = [ent('R-1', 'a'.repeat(80)), ent('R-2', 'a'.repeat(200))]
    const needle = 'a'.repeat(63) + 'b' // at MAX_QUERY_LENGTH
    const started = Date.now()
    rankMentions(rows, needle)
    expect(Date.now() - started).toBeLessThan(500)
  })

  it('handles an empty response without throwing', () => {
    expect(rankMentions([], 'anything')).toEqual([])
  })
})

describe('rankTypeNames', () => {
  const types = ['ticket', 'research', 'review-checklist', 'review-response', 'decision']

  it('keeps the schema order for an empty query', () => {
    expect(rankTypeNames(types, '')).toBe(types)
  })

  it('offers every type matching a short prefix', () => {
    const out = rankTypeNames(types, 're')
    expect(out).toContain('research')
    expect(out).toContain('review-checklist')
    expect(out).toContain('review-response')
    expect(out).not.toContain('decision')
  })

  it('ranks the closest name first', () => {
    expect(rankTypeNames(types, 'ticket')[0]).toBe('ticket')
  })

  it('drops non-matching types so suggestions stay relevant', () => {
    expect(rankTypeNames(types, 'zzzq')).toEqual([])
  })

  it('handles an empty type list and a single-type schema', () => {
    expect(rankTypeNames([], 'ticket')).toEqual([])
    expect(rankTypeNames(['ticket'], 'tick')).toEqual(['ticket'])
  })

  it('passes the list through for a query it cannot tokenize', () => {
    expect(rankTypeNames(types, '日本語')).toBe(types)
  })
})
