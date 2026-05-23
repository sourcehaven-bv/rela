import { describe, it, expect } from 'vitest'
import { toggleCheckboxInSource } from './checkboxToggle'

// Mirrors what the Go side used to test (TestToggleCheckbox in
// internal/dataentry/helpers_test.go) plus the additional bullet shapes
// that marked v17's task-list extension accepts as interactive checkboxes:
// `* [ ]`, `+ [ ]`, ordered `1. [ ]`. The toggler MUST accept the same
// set as the renderer or clicks on those checkboxes throw `out of range`
// (see TKT-R7Q9 RR-T8TV).

interface OkCase {
  name: string
  content: string
  index: number
  want: string
}

interface ErrCase {
  name: string
  content: string
  index: number
  match: RegExp
}

const okCases: OkCase[] = [
  { name: 'check unchecked', content: '- [ ] task one', index: 0, want: '- [x] task one' },
  { name: 'uncheck checked', content: '- [x] task one', index: 0, want: '- [ ] task one' },
  { name: 'uncheck uppercase', content: '- [X] task one', index: 0, want: '- [ ] task one' },
  {
    name: 'toggle second of three',
    content: '- [ ] first\n- [ ] second\n- [x] third',
    index: 1,
    want: '- [ ] first\n- [x] second\n- [x] third',
  },
  { name: 'star bullet', content: '* [ ] task', index: 0, want: '* [x] task' },
  { name: 'plus bullet', content: '+ [ ] task', index: 0, want: '+ [x] task' },
  { name: 'ordered single-digit', content: '1. [ ] task', index: 0, want: '1. [x] task' },
  { name: 'ordered multi-digit', content: '42. [ ] task', index: 0, want: '42. [x] task' },
  { name: 'preserves indentation', content: '    - [ ] indented', index: 0, want: '    - [x] indented' },
  {
    name: 'preserves CRLF line endings on untouched lines',
    content: '- [ ] first\r\n- [ ] second',
    index: 1,
    want: '- [ ] first\r\n- [x] second',
  },
  {
    name: 'mixed bullet shapes counted in source order',
    content: '* [ ] a\n- [ ] b\n+ [ ] c',
    index: 1,
    want: '* [ ] a\n- [x] b\n+ [ ] c',
  },
]

const errCases: ErrCase[] = [
  { name: 'index out of range', content: '- [ ] only one', index: 1, match: /out of range/ },
  { name: 'no checkboxes', content: 'just text', index: 0, match: /out of range/ },
  {
    name: 'rejects checkbox-shaped lines without trailing space',
    content: '- [ ]nospace',
    index: 0,
    match: /out of range/,
  },
]

describe('toggleCheckboxInSource', () => {
  for (const tc of okCases) {
    it(tc.name, () => {
      expect(toggleCheckboxInSource(tc.content, tc.index)).toBe(tc.want)
    })
  }

  describe('errors', () => {
    for (const tc of errCases) {
      it(tc.name, () => {
        expect(() => toggleCheckboxInSource(tc.content, tc.index)).toThrow(tc.match)
      })
    }
  })
})
