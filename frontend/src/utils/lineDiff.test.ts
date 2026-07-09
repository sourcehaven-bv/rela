import { describe, it, expect } from 'vitest'
import { lineDiff, propertyDiff } from './lineDiff'

describe('lineDiff', () => {
  it('marks unchanged lines equal', () => {
    const d = lineDiff('a\nb\nc', 'a\nb\nc')
    expect(d.every((l) => l.op === 'equal')).toBe(true)
    expect(d.map((l) => l.text)).toEqual(['a', 'b', 'c'])
  })

  it('detects an added line', () => {
    const d = lineDiff('a\nc', 'a\nb\nc')
    expect(d).toEqual([
      { op: 'equal', text: 'a' },
      { op: 'add', text: 'b' },
      { op: 'equal', text: 'c' },
    ])
  })

  it('detects a removed line', () => {
    const d = lineDiff('a\nb\nc', 'a\nc')
    expect(d).toEqual([
      { op: 'equal', text: 'a' },
      { op: 'del', text: 'b' },
      { op: 'equal', text: 'c' },
    ])
  })

  it('handles a changed line as del+add', () => {
    const d = lineDiff('a\nX\nc', 'a\nY\nc')
    expect(d).toEqual([
      { op: 'equal', text: 'a' },
      { op: 'del', text: 'X' },
      { op: 'add', text: 'Y' },
      { op: 'equal', text: 'c' },
    ])
  })

  it('handles empty before (all-add) and empty after (all-del)', () => {
    expect(lineDiff('', 'x\ny')).toEqual([
      { op: 'add', text: 'x' },
      { op: 'add', text: 'y' },
    ])
    expect(lineDiff('x\ny', '')).toEqual([
      { op: 'del', text: 'x' },
      { op: 'del', text: 'y' },
    ])
  })

  it('trims common prefix and suffix around a single change', () => {
    // Only the middle line changed; head/tail come through as equal.
    const d = lineDiff('h1\nh2\nMID-old\nt1\nt2', 'h1\nh2\nMID-new\nt1\nt2')
    expect(d).toEqual([
      { op: 'equal', text: 'h1' },
      { op: 'equal', text: 'h2' },
      { op: 'del', text: 'MID-old' },
      { op: 'add', text: 'MID-new' },
      { op: 'equal', text: 't1' },
      { op: 'equal', text: 't2' },
    ])
  })

  it('degrades to a coarse block diff for very large inputs without hanging', () => {
    // Two large, fully-different bodies would blow past the LCS cell cap; the
    // guard must return a correct (all-del then all-add) diff, fast.
    const before = Array.from({ length: 3000 }, (_, i) => `a${i}`).join('\n')
    const after = Array.from({ length: 3000 }, (_, i) => `b${i}`).join('\n')
    const start = Date.now()
    const d = lineDiff(before, after)
    expect(Date.now() - start).toBeLessThan(1000) // no O(n·m) freeze
    expect(d.filter((l) => l.op === 'del')).toHaveLength(3000)
    expect(d.filter((l) => l.op === 'add')).toHaveLength(3000)
    expect(d.some((l) => l.op === 'equal')).toBe(false)
  })
})

describe('propertyDiff', () => {
  it('reports add, del, and change sorted by key', () => {
    const changes = propertyDiff(
      { kept: 1, removed: 2, changed: 'old' },
      { kept: 1, added: 3, changed: 'new' },
    )
    expect(changes).toEqual([
      { key: 'added', op: 'add', after: 3 },
      { key: 'changed', op: 'change', before: 'old', after: 'new' },
      { key: 'removed', op: 'del', before: 2 },
    ])
  })

  it('treats deep-equal values as unchanged', () => {
    const changes = propertyDiff({ tags: ['a', 'b'] }, { tags: ['a', 'b'] })
    expect(changes).toEqual([])
  })

  it('does not report a change for object values that differ only in key order', () => {
    const changes = propertyDiff(
      { meta: { a: 1, b: 2 } },
      { meta: { b: 2, a: 1 } }, // same content, reordered keys
    )
    expect(changes).toEqual([])
  })

  it('still detects a real change in a nested object value', () => {
    const changes = propertyDiff({ meta: { a: 1 } }, { meta: { a: 2 } })
    expect(changes).toEqual([{ key: 'meta', op: 'change', before: { a: 1 }, after: { a: 2 } }])
  })
})
