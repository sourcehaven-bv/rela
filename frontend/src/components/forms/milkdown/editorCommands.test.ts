import { describe, it, expect } from 'vitest'
import { INLINE_COMMANDS, BLOCK_COMMANDS, filterBlockCommands } from './editorCommands'

describe('editorCommands', () => {
  it('gives every command a unique id, since ids key the active-state lookup', () => {
    const ids = [...INLINE_COMMANDS, ...BLOCK_COMMANDS].map((c) => c.id)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('gives every command a label and at least one keyword to match on', () => {
    for (const cmd of [...INLINE_COMMANDS, ...BLOCK_COMMANDS]) {
      expect(cmd.label, `${cmd.id} label`).toBeTruthy()
      expect(cmd.keywords.length, `${cmd.id} keywords`).toBeGreaterThan(0)
    }
  })

  it('keeps every keyword lowercase, because matching lowercases the query only', () => {
    for (const cmd of [...INLINE_COMMANDS, ...BLOCK_COMMANDS]) {
      for (const kw of cmd.keywords) expect(kw).toBe(kw.toLowerCase())
    }
  })

  describe('filterBlockCommands', () => {
    it('lists everything in declaration order for an empty query', () => {
      expect(filterBlockCommands('')).toEqual(BLOCK_COMMANDS)
      expect(filterBlockCommands('   ')).toEqual(BLOCK_COMMANDS)
    })

    it('matches on a label prefix', () => {
      expect(filterBlockCommands('quo').map((c) => c.id)).toContain('blockquote')
    })

    it('matches on a keyword the label does not contain', () => {
      // "Numbered list" is found by "ol", which appears only in its keywords.
      expect(filterBlockCommands('ol').map((c) => c.id)).toContain('orderedList')
    })

    it('is case-insensitive', () => {
      expect(filterBlockCommands('TABLE').map((c) => c.id)).toEqual(['table'])
    })

    it('ranks a prefix match above a mere substring match', () => {
      // "ord" prefixes "ordered" (Numbered list) but only appears inside
      // "unordered" (Bullet list), so the numbered list must come first.
      const ids = filterBlockCommands('ord').map((c) => c.id)
      expect(ids).toContain('orderedList')
      expect(ids).toContain('bulletList')
      expect(ids.indexOf('orderedList')).toBeLessThan(ids.indexOf('bulletList'))
    })

    it('returns nothing when no command matches, which closes the menu', () => {
      expect(filterBlockCommands('zzzz')).toEqual([])
    })

    it('finds each heading level by its own shorthand', () => {
      expect(filterBlockCommands('h2').map((c) => c.id)).toEqual(['h2'])
    })
  })
})
