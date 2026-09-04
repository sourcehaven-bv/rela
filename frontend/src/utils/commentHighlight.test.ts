import { describe, it, expect } from 'vitest'
import { applyHighlights, type HighlightRange } from './commentHighlight'

/** Byte offsets of `needle` within `body`, as the Go server would report them. */
function byteRange(body: string, needle: string, id = 'c1'): HighlightRange {
  const bytes = new TextEncoder().encode(body)
  const target = new TextEncoder().encode(needle)
  outer: for (let i = 0; i + target.length <= bytes.length; i++) {
    for (let j = 0; j < target.length; j++) {
      if (bytes[i + j] !== target[j]) continue outer
    }
    return { id, start: i, end: i + target.length }
  }
  throw new Error(`needle not found: ${needle}`)
}

describe('applyHighlights', () => {
  const body = 'The quick brown fox jumps over the lazy dog.'

  it('wraps a single range', () => {
    const out = applyHighlights(body, [byteRange(body, 'brown fox')])

    expect(out).toBe(
      'The quick <mark data-comment-id="c1">brown fox</mark> jumps over the lazy dog.'
    )
  })

  it('wraps several ranges without shifting each other', () => {
    // The bug this guards: applying front-to-back makes every insertion shift
    // the offsets of the ranges after it, so the second mark lands mid-word.
    const out = applyHighlights(body, [
      byteRange(body, 'The quick', 'a'),
      byteRange(body, 'lazy dog', 'b'),
    ])

    expect(out).toContain('<mark data-comment-id="a">The quick</mark>')
    expect(out).toContain('<mark data-comment-id="b">lazy dog</mark>')
  })

  it('marks uncertain ranges distinctly', () => {
    const r = { ...byteRange(body, 'brown fox'), uncertain: true }
    const out = applyHighlights(body, [r])

    expect(out).toContain('data-comment-uncertain="true"')
  })

  it('slices correctly across multi-byte characters', () => {
    // JS strings are UTF-16 and the server sends BYTE offsets, so slicing the
    // string directly would cut in the wrong place after any non-ASCII text.
    const utf = 'Le café serveert góéde köffie vandaag.'
    const out = applyHighlights(utf, [byteRange(utf, 'góéde köffie')])

    expect(out).toContain('<mark data-comment-id="c1">góéde köffie</mark>')
    // The text either side must be preserved intact.
    expect(out).toContain('Le café serveert ')
    expect(out).toContain(' vandaag.')
  })

  it('drops overlapping ranges rather than nesting them', () => {
    const a = byteRange(body, 'quick brown', 'a')
    const b = byteRange(body, 'brown fox', 'b') // overlaps a

    const out = applyHighlights(body, [a, b])

    expect(out).toContain('<mark data-comment-id="a">quick brown</mark>')
    expect(out).not.toContain('data-comment-id="b"')
    // Exactly one opening and one closing tag — no interleaving.
    expect(out.match(/<mark /g)).toHaveLength(1)
    expect(out.match(/<\/mark>/g)).toHaveLength(1)
  })

  it('drops ranges outside the body', () => {
    // Offsets resolved against a body that has since changed.
    const out = applyHighlights(body, [
      { id: 'past-end', start: 10, end: 9999 },
      { id: 'negative', start: -5, end: 4 },
      { id: 'empty', start: 4, end: 4 },
    ])

    expect(out).toBe(body)
  })

  it('escapes a quote in the id rather than breaking out of the attribute', () => {
    const out = applyHighlights(body, [{ ...byteRange(body, 'fox'), id: 'a"><script>x' }])

    expect(out).not.toContain('"><script>')
    expect(out).toContain('&quot;')
  })

  it('returns the body unchanged when there is nothing to mark', () => {
    expect(applyHighlights(body, [])).toBe(body)
    expect(applyHighlights('', [{ id: 'x', start: 0, end: 1 }])).toBe('')
  })
})
