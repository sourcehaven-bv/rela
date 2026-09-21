import { describe, it, expect } from 'vitest'
import { parseMentionQuery, shouldClearScopeOnBackspace } from './mentionQuery'

describe('parseMentionQuery', () => {
  it('opens on a bare @ at the start of a paragraph', () => {
    expect(parseMentionQuery('@')).toEqual({ query: '', matchLength: 1 })
  })

  it('opens on @ after a space and tracks the query', () => {
    expect(parseMentionQuery('see @TKT')).toEqual({ query: 'TKT', matchLength: 4 })
  })

  it('reports the match length so the trigger and query are replaced together', () => {
    const m = parseMentionQuery('x @abcd')
    expect(m?.matchLength).toBe(5)
  })

  it('closes when a space is typed after the trigger', () => {
    expect(parseMentionQuery('@TKT ')).toBeNull()
    expect(parseMentionQuery('@ ')).toBeNull()
  })

  it('closes inside a code span', () => {
    expect(parseMentionQuery('`@TKT')).toBeNull()
  })

  it('does not fire on an email address', () => {
    expect(parseMentionQuery('mail jeroen@example')).toBeNull()
    expect(parseMentionQuery('a@b')).toBeNull()
  })

  it('fires after an opening bracket or paren', () => {
    expect(parseMentionQuery('(@TKT')).toEqual({ query: 'TKT', matchLength: 4 })
    expect(parseMentionQuery('[@TKT')).toEqual({ query: 'TKT', matchLength: 4 })
  })

  it('tracks the most recent @ when there are several', () => {
    expect(parseMentionQuery('@one and @two')).toEqual({ query: 'two', matchLength: 4 })
  })

  it('returns null when there is no @', () => {
    expect(parseMentionQuery('plain text')).toBeNull()
  })

  it('returns null for undefined, which is what getContent gives outside a paragraph', () => {
    expect(parseMentionQuery(undefined)).toBeNull()
    expect(parseMentionQuery('')).toBeNull()
  })

  it('gives up on an over-long query rather than searching for it', () => {
    expect(parseMentionQuery('@' + 'x'.repeat(65))).toBeNull()
    expect(parseMentionQuery('@' + 'x'.repeat(64))).not.toBeNull()
  })
})

describe('shouldClearScopeOnBackspace', () => {
  it('is false without a scope, whatever the query', () => {
    expect(shouldClearScopeOnBackspace('see @', false)).toBe(false)
    expect(shouldClearScopeOnBackspace('see @abc', false)).toBe(false)
  })

  it('is true with a scope and an empty query', () => {
    expect(shouldClearScopeOnBackspace('see @', true)).toBe(true)
  })

  it('is false while the query still has characters to delete', () => {
    expect(shouldClearScopeOnBackspace('see @a', true)).toBe(false)
    expect(shouldClearScopeOnBackspace('see @abc', true)).toBe(false)
  })

  it('is false when there is no active mention at the cursor', () => {
    expect(shouldClearScopeOnBackspace(undefined, true)).toBe(false)
    expect(shouldClearScopeOnBackspace('', true)).toBe(false)
    expect(shouldClearScopeOnBackspace('no trigger here', true)).toBe(false)
    // A space terminates the query, so this is no longer an active mention.
    expect(shouldClearScopeOnBackspace('see @ ', true)).toBe(false)
  })

  it('is true for a bare @ at the very start of a paragraph', () => {
    expect(shouldClearScopeOnBackspace('@', true)).toBe(true)
  })
})
