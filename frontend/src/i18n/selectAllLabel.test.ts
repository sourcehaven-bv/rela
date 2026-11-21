/**
 * The select-all checkbox of a table section. A list without sections has one
 * section with no title, and its label read "Select all in " (cut off).
 */
import { DEFAULT_MESSAGES } from 'rela-components/composables/useMessages'
import { NL_MESSAGES } from 'rela-components/composables/messages.nl'

describe('select-all label', () => {
  it.each([
    ['en', DEFAULT_MESSAGES, 'Done', 'Select all in Done', 'Select all'],
    ['nl', NL_MESSAGES, 'Klaar', 'Alles in Klaar selecteren', 'Alles selecteren'],
  ])(
    '%s names the section, or none when it has no title',
    (_lang, messages, section, named, plain) => {
      expect(messages.selectAllIn({ section })).toBe(named)
      expect(messages.selectAllIn({ section: '' })).toBe(plain)
      expect(messages.selectAllIn({ section: '  ' })).toBe(plain)
    }
  )
})
