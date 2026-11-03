import { describe, it, expect } from 'vitest'
import { fileRefNames } from './fileRefs'

describe('fileRefNames', () => {
  it.each([
    [
      'a keyed storage path',
      'attachments/T-1/bijlagen/c54fb91819f593e9/report.pdf',
      ['report.pdf'],
    ],
    ['a legacy bare name', 'report.pdf', ['report.pdf']],
    ['backslash separators', 'attachments\\T-1\\bijlagen\\abc\\report.pdf', ['report.pdf']],
    ['a trailing slash', 'dir/report.pdf/', ['report.pdf']],
    ['an empty string', '', []],
    ['a dot-dot entry', '..', []],
    ['a non-string', 42, []],
    ['null', null, []],
  ])('reads %s', (_label, value, want) => {
    expect(fileRefNames(value)).toEqual(want)
  })

  it('sorts names and drops duplicates, as the entity page lists them', () => {
    const value = ['x/b.pdf', 'y/a.pdf', 'z/b.pdf', '']
    expect(fileRefNames(value)).toEqual(['a.pdf', 'b.pdf'])
  })
})
