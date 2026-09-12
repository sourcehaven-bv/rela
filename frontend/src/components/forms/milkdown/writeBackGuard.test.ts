import { describe, it, expect } from 'vitest'
import { guardWriteBack } from './writeBackGuard'

describe('guardWriteBack', () => {
  it('reports unchanged when the bytes match', () => {
    expect(guardWriteBack('# a\n', '# a\n', false)).toEqual({
      value: '# a\n',
      verdict: 'unchanged',
    })
  })

  it('saves the edit when the user typed something', () => {
    expect(guardWriteBack('# a\n', '# b\n', true)).toEqual({
      value: '# b\n',
      verdict: 'edited',
    })
  })

  it('suppresses a reformat that means the same thing', () => {
    const original = '| a | b |\n| - | - |\n| 1 | 2 |\n'
    const churned = '| a  | b  |\n| -- | -- |\n| 1  | 2  |\n'
    expect(guardWriteBack(original, churned, false)).toEqual({
      value: original,
      verdict: 'churn-suppressed',
    })
  })

  it('suppresses a setext-to-ATX rewrite', () => {
    expect(guardWriteBack('Title\n=====\n', '# Title\n', false)).toEqual({
      value: 'Title\n=====\n',
      verdict: 'churn-suppressed',
    })
  })

  it('blocks a meaning change that arrived without an edit', () => {
    expect(guardWriteBack('[x](/a)\n', '[x](/b)\n', false)).toEqual({
      value: '[x](/a)\n',
      verdict: 'drift-blocked',
    })
  })

  it('blocks dropped content that arrived without an edit', () => {
    expect(guardWriteBack('a\n\nb\n', 'a\n', false).verdict).toBe('drift-blocked')
  })

  it('keeps a dirty edit even when it drops content', () => {
    // Deleting a paragraph is a legitimate edit. The guard must not second
    // guess the user; it only judges round-trips they did not ask for.
    expect(guardWriteBack('a\n\nb\n', 'a\n', true)).toEqual({
      value: 'a\n',
      verdict: 'edited',
    })
  })

  it('preserves an entity ref through a churn suppression', () => {
    const original = 'See `TKT-ABC` now.\n'
    expect(guardWriteBack(original, 'See `TKT-ABC` now.\n', false).value).toBe(original)
  })
})
