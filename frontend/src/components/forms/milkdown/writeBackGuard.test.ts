import { describe, it, expect } from 'vitest'
import { guardWriteBack, decideEmit } from './writeBackGuard'

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

describe('decideEmit', () => {
  // The bug this exists to prevent: `original` must be the bytes the parent
  // handed over, NOT the editor's own settled serialization. Comparing the
  // round-tripped form against itself always says "unchanged", so churn was
  // reported clean and written back — a setext heading silently became ATX.
  const SETEXT = 'Title\n=====\n\nbody\n'
  const CHURNED = '# Title\n\nbody\n'

  it('ignores the editor echoing back its own load', () => {
    expect(decideEmit(CHURNED, SETEXT, CHURNED, false)).toEqual({ action: 'ignore' })
  })

  // Even if the echo check is bypassed (a later transaction re-emits the same
  // churned text), the guard must still recognise it as churn against the
  // ORIGINAL and refuse to propagate it.
  it('ignores churn measured against the original, not the settled value', () => {
    expect(decideEmit(CHURNED, SETEXT, 'something else', false)).toEqual({
      action: 'ignore',
    })
  })

  it('emits a real edit', () => {
    const edited = '# Title\n\nbody EDITED\n'
    expect(decideEmit(edited, SETEXT, CHURNED, true)).toEqual({
      action: 'emit',
      value: edited,
    })
  })

  it('reports drift rather than emitting it', () => {
    // Undirty content whose meaning differs from the original: the guard
    // cannot tell a serializer bug from an edit it did not observe, so it
    // refuses. Losing the edit silently would be worse than the churn.
    expect(decideEmit('completely different\n', SETEXT, CHURNED, false)).toEqual({
      action: 'report-drift',
    })
  })

  it('ignores an unchanged document', () => {
    expect(decideEmit(SETEXT, SETEXT, CHURNED, false)).toEqual({ action: 'ignore' })
  })
})
