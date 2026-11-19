import { describe, it, expect } from 'vitest'
import { linkHref, opensInNewTab } from './linkHref'

// Built from code points so the test file itself stays free of invisible characters.
const ZW = String.fromCharCode(0x200b)
const BOM = String.fromCharCode(0xfeff)
const TAB = String.fromCharCode(0x09)
const LF = String.fromCharCode(0x0a)
const NUL = String.fromCharCode(0x00)

describe('linkHref', () => {
  describe('links', () => {
    const cases: Array<[name: string, input: string, expected: string]> = [
      ['an https URL', 'https://example.com/a.pdf', 'https://example.com/a.pdf'],
      ['an http URL', 'http://example.com/a', 'http://example.com/a'],
      ['a mailto address', 'mailto:billing@example.com', 'mailto:billing@example.com'],
      ['a mixed-case scheme', 'HtTpS://Example.com/A', 'https://example.com/A'],
      ['an upper-case mailto', 'MAILTO:a@example.com', 'mailto:a@example.com'],
      ['leading whitespace', '   https://example.com/a', 'https://example.com/a'],
      ['a leading tab and newline', `${TAB}${LF}https://example.com/a`, 'https://example.com/a'],
      ['a leading control character', `${NUL}https://example.com/a`, 'https://example.com/a'],
      ['query and fragment on https', 'https://ex.com/?q=1#f', 'https://ex.com/?q=1#f'],
      [
        'a mailto query, dropped',
        'mailto:a@example.com?bcc=x@evil.example',
        'mailto:a@example.com',
      ],
      ['a mailto fragment, dropped', 'mailto:a@example.com#x?bcc=y', 'mailto:a@example.com'],
    ]
    it.each(cases)('%s', (_name, input, expected) => {
      expect(linkHref(input)).toBe(expected)
    })
  })

  describe('keeps as text', () => {
    const cases: Array<[name: string, input: unknown]> = [
      ['a javascript URL', 'javascript:alert(1)'],
      ['a mixed-case javascript URL', 'JaVaScRiPt:alert(1)'],
      ['javascript after leading whitespace', '  javascript:alert(1)'],
      ['javascript split by a tab', `java${TAB}script:alert(1)`],
      ['javascript split by a newline', `java${LF}script:alert(1)`],
      ['an entity-encoded javascript URL', '&#106;avascript:alert(1)'],
      ['a data URL', 'data:text/html;base64,PHNjcmlwdD4='],
      ['a vbscript URL', 'vbscript:msgbox(1)'],
      ['an ftp URL', 'ftp://example.com/a'],
      ['a file URL', 'file:///etc/passwd'],
      ['a zero-width space before the scheme', `${ZW}https://example.com`],
      ['a BOM before the scheme', `${BOM}javascript:alert(1)`],
      ['a bare host', 'example.com'],
      ['a file name', 'notes.txt'],
      ['a relative path', '/invoices/1.pdf'],
      ['a protocol-relative URL', '//evil.example/x'],
      ['an empty string', ''],
      ['whitespace only', '   '],
      ['a number', 42],
      ['undefined', undefined],
    ]
    it.each(cases)('%s', (_name, input) => {
      expect(linkHref(input)).toBeUndefined()
    })
  })
})

describe('opensInNewTab', () => {
  it('opens web pages in a new tab and mail links in place', () => {
    expect(opensInNewTab('https://example.com/')).toBe(true)
    expect(opensInNewTab('http://example.com/')).toBe(true)
    expect(opensInNewTab('mailto:a@example.com')).toBe(false)
  })
})
