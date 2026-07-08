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
})
