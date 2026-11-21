import { describe, expect, it } from 'vitest'
import { collapseUnchanged, diffBlocks, hasChanges, type DiffBlock } from './renderedDiff'

const ops = (blocks: DiffBlock[]) => blocks.map((b) => b.op)

describe('diffBlocks', () => {
  it('marks identical documents as all equal', () => {
    const md = '# Title\n\nOne.\n\nTwo.'
    expect(ops(diffBlocks(md, md))).toEqual(['equal', 'equal', 'equal'])
  })

  it('marks a new block as added and a dropped block as removed', () => {
    const blocks = diffBlocks('Keep.\n\nDrop.', 'Keep.\n\nNew.')
    // The two paragraphs share nothing, so this is not an in-place edit.
    expect(ops(blocks)).toEqual(['equal', 'del', 'add'])
    expect(diffBlocks('Keep.', 'Keep.\n\nExtra.').map((b) => b.op)).toEqual(['equal', 'add'])
    expect(diffBlocks('Keep.\n\nGone.', 'Keep.').map((b) => b.op)).toEqual(['equal', 'del'])
  })

  it('highlights only the changed words inside a paragraph', () => {
    const [block] = diffBlocks('The quick brown fox', 'The quick red fox')
    expect(block.op).toBe('change')
    expect(block.html).toContain('<ins class="rl-rd__ins">red</ins>')
    expect(block.html).toContain('<del class="rl-rd__del">brown</del>')
    expect(block.html).toContain('The quick')
  })

  it('keeps list structure and diffs per item', () => {
    const [block] = diffBlocks('- a\n- b yearly\n- c', '- a\n- b monthly\n- c\n- d')
    expect(block.html.match(/<li>/g)).toHaveLength(4)
    expect(block.html).toContain('<ins class="rl-rd__ins">monthly</ins>')
    expect(block.html).toContain('<del class="rl-rd__del">yearly</del>')
    expect(block.html).toMatch(/<li>(?:<p>)?<ins class="rl-rd__ins">d<\/ins>/)
  })

  it('diffs table cells instead of replacing the table', () => {
    const before = '| L | T |\n| - | - |\n| High | 4 h |'
    const after = '| L | T |\n| - | - |\n| High | 2 h |'
    const blocks = diffBlocks(before, after)
    expect(ops(blocks)).toEqual(['change'])
    expect(blocks[0].html).toContain('<table>')
    expect(blocks[0].html).toContain('<ins class="rl-rd__ins">2</ins>')
  })

  it('shows blocks of different types as a removal and an addition', () => {
    expect(ops(diffBlocks('Some text', '```\ncode\n```'))).toEqual(['del', 'add'])
  })

  it('shows removed words exactly as written', () => {
    const [block] = diffBlocks('Say **bold** now', 'Say plain now')
    expect(block.html).toContain('<del class="rl-rd__del">**bold**</del>')
  })

  it('escapes raw html', () => {
    const html = diffBlocks('Hi', 'Hi <img src=x onerror=alert(1)>').map((b) => b.html).join('')
    expect(html).not.toContain('<img')
    expect(html).toContain('&lt;img')
  })

  it('drops links with unsafe schemes but keeps safe ones', () => {
    const html = (md: string) => diffBlocks('x', md).map((b) => b.html).join('')
    expect(html('[a](javascript:alert(1))')).not.toContain('<a')
    expect(html('[a](https://example.com)')).toContain('<a href="https://example.com"')
  })

  it('handles empty sides', () => {
    expect(diffBlocks('', '')).toEqual([])
    expect(ops(diffBlocks('', 'New.'))).toEqual(['add'])
    expect(ops(diffBlocks('Old.', ''))).toEqual(['del'])
  })
})

describe('diffBlocks safety and fallbacks', () => {
  const markerInAttribute = /="[^"]*&lt;(?:ins|del)/

  it('replaces a block with a link by a removal and an addition, never splicing markers into it', () => {
    const blocks = diffBlocks('see [docs](http://x/old) now', 'see [docs](http://x/new) now')
    expect(ops(blocks)).toEqual(['del', 'add'])
    for (const b of blocks) expect(b.html).not.toMatch(markerInAttribute)
  })

  it('ignores private-use marker characters in the input', () => {
    const html = diffBlocks('x', '[x](http://a "\uE000 onmouseover=alert(1) x=")').map((b) => b.html).join('')
    expect(html).not.toContain('<ins')
    expect(html).not.toContain('\uE000')
  })

  it('shows a table with a different column count as a removal and an addition', () => {
    const blocks = diffBlocks('| A | B |\n| - | - |\n| 1 | 2 |', '| A | B | C |\n| - | - | - |\n| 1 | 2 | 3 |')
    expect(ops(blocks)).toEqual(['del', 'add'])
    expect(blocks.every((b) => b.html.includes('<table>'))).toBe(true)
  })

  it('falls back when only the heading level changes', () => {
    expect(ops(diffBlocks('# A', '## A'))).toEqual(['del', 'add'])
  })

  it('does not word-diff blocks that share little', () => {
    expect(ops(diffBlocks('Alpha beta gamma delta', 'Completely unrelated words here'))).toEqual(['del', 'add'])
  })

  it('stays fast on a very large one-line paragraph', () => {
    const words = (n: number, tag: string) => Array.from({ length: n }, (_, i) => `${tag}${i % 7}w${i}`).join(' ')
    const start = performance.now()
    diffBlocks(words(10_000, 'a'), words(10_000, 'b'))
    expect(performance.now() - start).toBeLessThan(2000)
  })

  it('stays fast on a long whitespace run', () => {
    const start = performance.now()
    diffBlocks(`x${' '.repeat(60_000)}y z`, `q${' '.repeat(60_000)}y w`)
    expect(performance.now() - start).toBeLessThan(2000)
  })
})

describe('hasChanges', () => {
  it('is false for identical or whitespace-only differences between blocks', () => {
    expect(hasChanges('A\n\nB', 'A\n\n\n\nB')).toBe(false)
    expect(hasChanges('A', 'A')).toBe(false)
  })
  it('is true when text differs', () => {
    expect(hasChanges('A', 'B')).toBe(true)
  })
})

describe('collapseUnchanged', () => {
  const eq = (n: number): DiffBlock[] => Array.from({ length: n }, (_, i) => ({ op: 'equal', html: `e${i}` }))
  const change: DiffBlock = { op: 'change', html: 'c' }

  it('folds long unchanged runs and keeps context next to a change', () => {
    const segs = collapseUnchanged([...eq(5), change, ...eq(5)], 1)
    expect(segs.map((s) => s.kind)).toEqual(['collapsed', 'block', 'block', 'block', 'collapsed'])
    const first = segs[0]
    expect(first.kind === 'collapsed' && first.blocks).toHaveLength(4)
  })

  it('does not fold a single unchanged block', () => {
    const segs = collapseUnchanged([...eq(2), change], 1)
    expect(segs.map((s) => s.kind)).toEqual(['block', 'block', 'block'])
  })

  it('keeps everything with enough context', () => {
    const segs = collapseUnchanged([...eq(2), change, ...eq(2)], 5)
    expect(segs.every((s) => s.kind === 'block')).toBe(true)
  })

  it('folds a fully unchanged document into one segment', () => {
    expect(collapseUnchanged(eq(4), 1).map((s) => s.kind)).toEqual(['collapsed'])
  })
})
