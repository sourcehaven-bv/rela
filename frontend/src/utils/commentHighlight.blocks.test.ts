// @vitest-environment jsdom
//
// These cases render through renderMarkdown, whose DOMPurify pass needs
// browser-accurate serialization; happy-dom mangles adjacent block elements
// (BUG-SQSV6V), so this file opts into jsdom like markdown.test.ts.
import { describe, it, expect } from 'vitest'
import { applyHighlights, type HighlightRange } from './commentHighlight'
import { renderMarkdown } from './markdown'
import type { ByteSpan } from '@/api/comments'

const enc = new TextEncoder()

/** Byte span of the first occurrence of `needle` at or after byte `from`. */
function spanOf(body: string, needle: string, from = 0): ByteSpan {
  const i = body.indexOf(needle, new TextDecoder().decode(enc.encode(body).slice(0, from)).length)
  if (i < 0) throw new Error(`needle not found: ${needle}`)
  const start = enc.encode(body.slice(0, i)).length
  return { start, end: start + enc.encode(needle).length }
}

/**
 * A range from the start of the first segment to the end of the last, with
 * those segments, as the server sends a cross-block anchor.
 */
function crossBlock(body: string, segments: string[], id = 'c1'): HighlightRange {
  const spans: ByteSpan[] = []
  let at = 0
  for (const s of segments) {
    const sp = spanOf(body, s, at)
    spans.push(sp)
    at = sp.end
  }
  return { id, start: spans[0].start, end: spans[spans.length - 1].end, segments: spans }
}

function render(body: string, ranges: HighlightRange[]): HTMLElement {
  const el = document.createElement('div')
  el.innerHTML = renderMarkdown(applyHighlights(body, ranges))
  return el
}

/** The text of every mark for a comment id, in document order. */
function marked(el: HTMLElement, id = 'c1'): string[] {
  return [...el.querySelectorAll(`mark[data-comment-id="${id}"]`)].map((m) => m.textContent ?? '')
}

describe('applyHighlights across blocks', () => {
  it('marks a heading and its body separately', () => {
    const body = '## Configuration\n\nConfigure the package by creating a file.\n'
    const el = render(body, [crossBlock(body, ['Configuration', 'Configure the package'])])

    expect(marked(el)).toEqual(['Configuration', 'Configure the package'])
    expect(el.querySelector('h2 mark')).not.toBeNull()
    expect(el.querySelector('p mark')).not.toBeNull()
    expect(el.textContent).not.toContain('<mark')
  })

  it('marks each item of a tight list', () => {
    const body = '- first item\n- second item\n- third item\n'
    const el = render(body, [crossBlock(body, ['item', 'second item', 'third'])])

    expect(marked(el)).toEqual(['item', 'second item', 'third'])
    expect(el.querySelectorAll('li mark')).toHaveLength(3)
  })

  it('marks across paragraphs of a blockquote line by line', () => {
    const body = 'Intro.\n\n> quote line\n> second line\n'
    const el = render(body, [crossBlock(body, ['Intro.', 'quote line\n> second'])])

    expect(marked(el)).toEqual(['Intro.', 'quote line\nsecond'])
    expect(el.querySelector('blockquote mark')).not.toBeNull()
  })

  it('marks table cells separately', () => {
    const body = '| a | b |\n|---|---|\n| one | two |\n'
    const el = render(body, [crossBlock(body, ['one', 'two'])])

    expect(marked(el)).toEqual(['one', 'two'])
    expect(el.querySelectorAll('td mark')).toHaveLength(2)
  })

  it('keeps task-list checkboxes working', () => {
    const body = '- [ ] write tests\n- [x] ship it\n'
    const el = render(body, [crossBlock(body, ['tests', 'ship'])])

    expect(marked(el)).toEqual(['tests', 'ship'])
    expect(el.querySelectorAll('input[type="checkbox"]')).toHaveLength(2)
  })

  it('keeps a covered block with edge emphasis balanced', () => {
    const body = 'Intro text.\n\n**Bold** start and *end*\n\nOutro text.\n'
    const el = render(body, [crossBlock(body, ['text.', '**Bold** start and *end*', 'Outro'])])

    expect(marked(el)).toEqual(['text.', 'Bold start and end', 'Outro'])
    expect(el.querySelector('mark strong')).not.toBeNull()
    expect(el.querySelector('mark em')).not.toBeNull()
  })

  it('marks nothing for an empty segment list', () => {
    const body = 'Intro.\n\n```\nmake test\n```\n'
    const r = { ...spanOf(body, 'make test'), id: 'c1', segments: [] }
    expect(applyHighlights(body, [r])).toBe(body)
  })

  it('trusts server segments over the approximate code scan', () => {
    // Four-space indentation reads as code to the scan, but here it is a
    // nested list item; the server parsed it and sent a segment for it.
    const body = '- outer item\n    - inner item\n- last item\n'
    const el = render(body, [crossBlock(body, ['outer item', 'inner item', 'last'])])
    expect(marked(el)).toEqual(['outer item', 'inner item', 'last'])
  })

  it('still skips a segment-less range that touches code', () => {
    const body = 'Intro text.\n\nRun `make` now.\n'
    const r = { ...spanOf(body, 'Run `make` now'), id: 'c1' }
    expect(applyHighlights(body, [r])).toBe(body)
  })

  it('drops a segment overlapping the one before it', () => {
    const body = 'First para.\n\nSecond para.\n'
    const r = crossBlock(body, ['First', 'Second'])
    r.segments!.push(spanOf(body, 'Second'))
    expect(marked(render(body, [r]))).toEqual(['First', 'Second'])
  })

  it('keeps a segment that starts before the range', () => {
    // The server lifts a start inside "**important**" to the opening "**".
    const body = 'Read the **important** text.\n\nNext paragraph.\n'
    const r = {
      ...crossBlock(body, ['**important** text.', 'Next']),
      start: spanOf(body, 'portant').start,
    }
    expect(marked(render(body, [r]))).toEqual(['important text.', 'Next'])
  })

  it('keeps a short comment inside a long cross-block one', () => {
    const body = '## Setup\n\nInstall the tools first.\n'
    const outer = crossBlock(body, ['Setup', 'Install the tools first.'], 'outer')
    const inner = { ...spanOf(body, 'the tools'), id: 'inner' }
    const el = render(body, [outer, inner])
    expect(marked(el, 'inner')).toEqual(['the tools'])
    expect(marked(el, 'outer')).toEqual([])
  })

  it('puts one chip after the first segment with a link', () => {
    const body = 'See [one](https://a.example) here.\n\nAnd [two](https://b.example) too.\n'
    const r = crossBlock(body, [
      'See [one](https://a.example) here.',
      'And [two](https://b.example)',
    ])
    const el = render(body, [r])

    const chips = el.querySelectorAll('[data-comment-chip]')
    expect(chips).toHaveLength(1)
    expect(chips[0].closest('p')?.textContent).toContain('one')
  })

  it('opens the same thread from every segment', () => {
    const body = '## Title\n\nBody text here.\n'
    const el = render(body, [crossBlock(body, ['Title', 'Body text'], 'thread-7')])
    const ids = [...el.querySelectorAll('mark')].map((m) => m.getAttribute('data-comment-id'))
    expect(ids).toEqual(['thread-7', 'thread-7'])
  })

  it('still marks a whole range when the server sent no segments', () => {
    const body = 'One phrase in a paragraph.\n'
    const r = { ...spanOf(body, 'phrase in a'), id: 'c1' }
    expect(marked(render(body, [r]))).toEqual(['phrase in a'])
  })
})
