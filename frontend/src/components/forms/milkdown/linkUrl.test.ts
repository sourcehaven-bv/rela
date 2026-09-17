import { describe, it, expect } from 'vitest'
import { normalizeLinkUrl } from './linkUrl'

/** Built from code points so the test file itself stays free of invisible characters. */
const ZW = String.fromCharCode(0x200b)
const TAB = String.fromCharCode(0x09)
const BOM = String.fromCharCode(0xfeff)

function accepted(input: string): string {
  const r = normalizeLinkUrl(input)
  if (!r.ok) throw new Error(`expected accept, got ${r.reason}`)
  return r.url
}

describe('normalizeLinkUrl', () => {
  describe('accepts and normalizes', () => {
    const cases: Array<[name: string, input: string, expected: string]> = [
      ['an ordinary https URL', 'https://example.com/a', 'https://example.com/a'],
      ['an http URL', 'http://example.com/a', 'http://example.com/a'],
      ['a bare host', 'example.com', 'https://example.com/'],
      ['a bare host with a path', 'example.com/path', 'https://example.com/path'],
      // The case that motivated testing the host pattern before the scheme
      // pattern: `example.com:` otherwise parses as a scheme.
      ['a bare host with a port', 'example.com:8080/path', 'https://example.com:8080/path'],
      ['query and fragment', 'https://ex.com/?q=1&x=2#f', 'https://ex.com/?q=1&x=2#f'],
      ['a mailto address', 'mailto:a@b.com', 'mailto:a@b.com'],
      ['an uppercase scheme', 'MAILTO:A@B.com', 'mailto:A@B.com'],
      ['surrounding whitespace', '  https://example.com/a  ', 'https://example.com/a'],
      ['an already-punycoded host', 'https://xn--e1awd7f.example/ok', 'https://xn--e1awd7f.example/ok'],
    ]

    it.each(cases)('%s', (_name, input, expected) => {
      expect(accepted(input)).toBe(expected)
    })
  })

  describe('refuses', () => {
    const cases: Array<[name: string, input: string, reason: string]> = [
      ['an empty string', '', 'empty'],
      ['whitespace only', '   ', 'empty'],
      ['javascript:', 'javascript:alert(1)', 'blocked-scheme'],
      ['data:', 'data:text/html,<script>x</script>', 'blocked-scheme'],
      ['vbscript:', 'vbscript:msgbox(1)', 'blocked-scheme'],
      ['file:', 'file:///etc/passwd', 'blocked-scheme'],
      // tel: and ftp: are allowed by the preset's sanitizer and NOT by us.
      // These two pin the narrowing; without them a future "just reuse the
      // preset" refactor would pass its tests.
      ['tel: (allowed by the preset, not by us)', 'tel:+3112345', 'blocked-scheme'],
      ['ftp: (allowed by the preset, not by us)', 'ftp://x.com/f', 'blocked-scheme'],
      ['an absolute path', '/relative/path', 'not-absolute'],
      ['a relative path', '../up', 'not-absolute'],
      ['a bare word', 'notaurl', 'not-absolute'],
      ['a bare email address', 'user@example.com', 'not-absolute'],
      ['a protocol-relative URL', '//evil.com/x', 'protocol-relative'],
      ['a windows path', 'C:/Users/foo', 'blocked-scheme'],
    ]

    it.each(cases)('%s', (_name, input, reason) => {
      const r = normalizeLinkUrl(input)
      expect(r.ok).toBe(false)
      if (!r.ok) {
        expect(r.reason).toBe(reason)
        // The message names the rule. It must never echo the input, or a
        // crafted URL reaches the error surface as content. Skipped for
        // blank input, where there is no needle to look for and every
        // string trivially contains the empty one.
        const needle = input.trim().slice(0, 12)
        if (needle) expect(r.message).not.toContain(needle)
        expect(r.message.length).toBeGreaterThan(0)
      }
    })
  })

  describe('scheme obfuscation', () => {
    // A browser discards these characters while resolving the scheme, so
    // `java<TAB>script:` runs. Refusing them is the whole point of stripping
    // before the scheme is read.
    const cases: Array<[name: string, input: string]> = [
      ['a zero-width space inside the scheme', `ja${ZW}vascript:alert(1)`],
      ['a tab inside the scheme', `java${TAB}script:alert(1)`],
      ['a BOM inside the scheme', `java${BOM}script:alert(1)`],
      ['a leading zero-width space', `${ZW}javascript:alert(1)`],
      ['a newline inside the scheme', 'java\nscript:alert(1)'],
    ]

    it.each(cases)('refuses %s', (_name, input) => {
      expect(normalizeLinkUrl(input).ok).toBe(false)
    })
  })

  describe('the returned value is clean, not merely accepted', () => {
    // This is the assertion that would have caught the original design, which
    // delegated to the preset's `sanitizeLinkHref`. That function refuses
    // obfuscated SCHEMES but returns the unnormalized string, so a zero-width
    // character elsewhere in the URL survived into the stored markdown. A
    // refusal-only test suite passes against that bug.
    function hasIgnoredChar(s: string): boolean {
      for (const ch of s) {
        const c = ch.codePointAt(0) ?? 0
        if (
          c <= 0x20 ||
          (c >= 0x7f && c <= 0xa0) ||
          (c >= 0x200b && c <= 0x200d) ||
          c === 0x2028 ||
          c === 0x2029 ||
          c === 0xfeff
        )
          return true
      }
      return false
    }

    it.each([
      ['a zero-width space in the path', `https://ex.com/${ZW}path`, 'https://ex.com/path'],
      ['a leading zero-width space', `${ZW}https://ex.com/a`, 'https://ex.com/a'],
      ['a BOM in the path', `https://ex.com/${BOM}path`, 'https://ex.com/path'],
    ])('strips %s from the stored value', (_name, input, expected) => {
      const url = accepted(input)
      expect(url).toBe(expected)
      expect(hasIgnoredChar(url)).toBe(false)
    })
  })

  describe('mailto parameters', () => {
    it('drops a query string carrying bcc', () => {
      const r = normalizeLinkUrl('mailto:a@b.com?subject=hi&bcc=evil@x.com')
      expect(r).toMatchObject({ ok: true, url: 'mailto:a@b.com', strippedParams: true })
    })

    it('leaves a plain mailto alone and does not claim to have stripped', () => {
      const r = normalizeLinkUrl('mailto:a@b.com')
      expect(r).toMatchObject({ ok: true, url: 'mailto:a@b.com' })
      if (r.ok) expect(r.strippedParams).toBeUndefined()
    })

    it('keeps a query string on http, which has no equivalent side effect', () => {
      expect(accepted('https://ex.com/?q=1')).toBe('https://ex.com/?q=1')
    })
  })

  describe('hosts we deliberately do not special-case', () => {
    it('refuses a single-label host with a port', () => {
      // `localhost:3000` has no dot, so it reads as a scheme. Accepting it
      // would mean treating any `word:number` as a host. Entity bodies are
      // shared documents; a link to someone's dev server is not useful there.
      expect(normalizeLinkUrl('localhost:3000').ok).toBe(false)
    })

    it('accepts an IDN host without punycode-normalizing it', () => {
      // Homograph detection is the browser address bar's job. Rewriting to
      // punycode here would surprise legitimate non-ASCII users.
      const url = accepted('https://пример.рф/путь')
      expect(url).toContain('/')
      expect(normalizeLinkUrl('https://еxample.com').ok).toBe(true)
    })

    it('accepts an IP host', () => {
      expect(accepted('1.2.3.4')).toBe('https://1.2.3.4/')
    })
  })

  it('handles a very long URL without truncating it', () => {
    const long = `https://example.com/${'a'.repeat(5000)}`
    expect(accepted(long)).toBe(long)
  })
})
