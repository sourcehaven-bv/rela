// @vitest-environment jsdom
//
// renderMarkdown sanitizes through DOMPurify, which depends on
// browser-accurate DOM serialization. happy-dom (the suite default) mangles
// adjacent block elements under DOMPurify >= 3.4.6 — e.g. it strips the first
// of two sibling <p> tags — so these assertions fail there while real
// browsers (and the E2E suite) render correctly. jsdom matches browser
// serialization, so this file opts into it. See BUG-SQSV6V.
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderMarkdown, getCheckboxStats, renderMermaidDiagrams } from './markdown'

describe('markdown', () => {
  describe('renderMarkdown', () => {
    it('returns empty string for empty content', () => {
      expect(renderMarkdown('')).toBe('')
    })

    it('renders basic markdown', () => {
      const result = renderMarkdown('# Hello')
      expect(result).toContain('<h1')
      expect(result).toContain('Hello')
    })

    it('renders paragraphs', () => {
      const result = renderMarkdown('This is a paragraph.')
      expect(result).toContain('<p>')
      expect(result).toContain('This is a paragraph.')
    })

    it('renders bold text', () => {
      const result = renderMarkdown('**bold text**')
      expect(result).toContain('<strong>')
      expect(result).toContain('bold text')
    })

    it('renders italic text', () => {
      const result = renderMarkdown('*italic text*')
      expect(result).toContain('<em>')
      expect(result).toContain('italic text')
    })

    it('renders links', () => {
      const result = renderMarkdown('[link](https://example.com)')
      expect(result).toContain('<a')
      expect(result).toContain('href="https://example.com"')
      expect(result).toContain('link')
    })

    it('renders code blocks', () => {
      const result = renderMarkdown('```\ncode\n```')
      expect(result).toContain('<code>')
      expect(result).toContain('code')
    })

    it('renders inline code', () => {
      const result = renderMarkdown('`inline code`')
      expect(result).toContain('<code>')
      expect(result).toContain('inline code')
    })

    it('renders unordered lists', () => {
      const result = renderMarkdown('- item 1\n- item 2')
      expect(result).toContain('<ul>')
      expect(result).toContain('<li>')
      expect(result).toContain('item 1')
      expect(result).toContain('item 2')
    })

    it('renders ordered lists', () => {
      const result = renderMarkdown('1. first\n2. second')
      expect(result).toContain('<ol>')
      expect(result).toContain('<li>')
    })

    it('renders checkboxes', () => {
      const result = renderMarkdown('- [ ] unchecked\n- [x] checked')
      expect(result).toContain('type="checkbox"')
      expect(result).toContain('unchecked')
      expect(result).toContain('checked')
    })

    it('omits data-cb-idx and keeps disabled by default (non-interactive)', () => {
      const result = renderMarkdown('- [ ] one\n- [x] two')
      expect(result).not.toContain('data-cb-idx')
      expect(result).toContain('disabled="" type="checkbox"')
    })

    it('tags each rendered checkbox with sequential data-cb-idx when interactive', () => {
      const result = renderMarkdown('- [ ] one\n- [x] two\n- [ ] three', { interactive: true })
      expect(result).toContain('data-cb-idx="0"')
      expect(result).toContain('data-cb-idx="1"')
      expect(result).toContain('data-cb-idx="2"')
      expect(result).not.toContain('disabled')
    })

    it('resets data-cb-idx counter per render', () => {
      renderMarkdown('- [ ] a\n- [ ] b', { interactive: true })
      const second = renderMarkdown('- [ ] c\n- [ ] d', { interactive: true })
      // Exact-count assertion: exactly two checkboxes, indices 0 and 1.
      const matches = second.match(/data-cb-idx="\d+"/g) ?? []
      expect(matches).toEqual(['data-cb-idx="0"', 'data-cb-idx="1"'])
    })

    it('sanitizes dangerous HTML', () => {
      const result = renderMarkdown('<script>alert("xss")</script>')
      expect(result).not.toContain('<script>')
      expect(result).not.toContain('alert')
    })

    it('sanitizes onclick handlers', () => {
      const result = renderMarkdown('<div onclick="alert(1)">click</div>')
      expect(result).not.toContain('onclick')
    })

    it('preserves safe HTML elements', () => {
      const result = renderMarkdown('**bold** and *italic*')
      expect(result).toContain('<strong>')
      expect(result).toContain('<em>')
    })

    // CommonMark soft-break semantics: single newlines in source are
    // whitespace, not hard breaks. Entity content is hard-wrapped at
    // ~80 chars in the .md source and that wrapping must not survive
    // into HTML — otherwise paragraphs render with the source column
    // width instead of reflowing with the viewport.
    //
    // We assert on the absence of `<br>` and on text equivalence after
    // whitespace collapsing rather than on the exact softbreak character
    // marked emits: CommonMark allows softbreaks to render as either a
    // line ending or a space, so pinning to `\n` would couple the test
    // to a marked.js implementation detail.
    it('treats single newlines inside a paragraph as whitespace, not <br>', () => {
      const result = renderMarkdown('foo\nbar')
      const doc = new DOMParser().parseFromString(result, 'text/html')
      const paragraphs = doc.querySelectorAll('p')
      expect(paragraphs.length).toBe(1)
      expect(paragraphs[0].querySelectorAll('br').length).toBe(0)
      expect(paragraphs[0].textContent?.replace(/\s+/g, ' ').trim()).toBe('foo bar')
    })

    it('preserves CommonMark hard breaks (two trailing spaces)', () => {
      const result = renderMarkdown('foo  \nbar')
      const doc = new DOMParser().parseFromString(result, 'text/html')
      const paragraphs = doc.querySelectorAll('p')
      expect(paragraphs.length).toBe(1)
      // Pin position: a single <br> must sit between "foo" and "bar",
      // not merely be present somewhere in the paragraph.
      const childTags = Array.from(paragraphs[0].childNodes).map((n) =>
        n.nodeType === Node.ELEMENT_NODE ? (n as Element).tagName.toLowerCase() : '#text',
      )
      expect(childTags).toContain('br')
      expect(paragraphs[0].querySelectorAll('br').length).toBe(1)
      expect(paragraphs[0].innerHTML).toMatch(/foo\s*<br[^>]*>\s*bar/)
    })

    it('separates paragraphs split by a blank line', () => {
      const result = renderMarkdown('first paragraph\n\nsecond paragraph')
      const doc = new DOMParser().parseFromString(result, 'text/html')
      const paragraphs = doc.querySelectorAll('p')
      expect(paragraphs.length).toBe(2)
      expect(paragraphs[0].textContent).toBe('first paragraph')
      expect(paragraphs[1].textContent).toBe('second paragraph')
    })

    describe('refResolver (entity-ID code spans)', () => {
      // A reusable resolver covering the seed below; each test passes only
      // the IDs it expects to hit, mirroring the server-side mentions map.
      function makeResolver(map: Record<string, { type: string; title: string; inaccessible?: boolean; inaccessibleReason?: string }>) {
        return (id: string) => map[id] ?? null
      }

      it('rewrites a known-ID code span into a titled link', () => {
        const resolver = makeResolver({
          'TKT-LXYHQ': { type: 'ticket', title: 'Resolve refs' },
        })
        const result = renderMarkdown('see `TKT-LXYHQ` for details', resolver)
        expect(result).toContain('href="/entity/ticket/TKT-LXYHQ"')
        expect(result).toContain('Resolve refs')
        // The code span should be gone (collapsed into the link).
        expect(result).not.toMatch(/<code>TKT-LXYHQ<\/code>/)
      })

      it('rewrites a manual-ID code span into a titled link', () => {
        const resolver = makeResolver({
          'data-entry-ui': { type: 'concept', title: 'Data Entry Web UI' },
        })
        const result = renderMarkdown('covered by `data-entry-ui`', resolver)
        expect(result).toContain('href="/entity/concept/data-entry-ui"')
        expect(result).toContain('Data Entry Web UI')
      })

      it('leaves unknown-ID code spans as <code>', () => {
        const resolver = makeResolver({})
        const result = renderMarkdown('`TKT-NOPE` is unknown', resolver)
        expect(result).toContain('<code>TKT-NOPE</code>')
        expect(result).not.toContain('href=')
      })

      it('does not rewrite multi-token code spans (exact-match only)', () => {
        const resolver = makeResolver({
          'TKT-LXYHQ': { type: 'ticket', title: 'Resolve refs' },
        })
        const result = renderMarkdown('`TKT-LXYHQ and FEAT-010`', resolver)
        expect(result).toContain('<code>TKT-LXYHQ and FEAT-010</code>')
        expect(result).not.toContain('href=')
      })

      it('does not rewrite IDs inside fenced code blocks', () => {
        const resolver = makeResolver({
          'TKT-LXYHQ': { type: 'ticket', title: 'Resolve refs' },
        })
        const result = renderMarkdown('```\nTKT-LXYHQ on a code line\n```', resolver)
        expect(result).not.toContain('href="/entity/ticket/TKT-LXYHQ"')
      })

      it('does not rewrite IDs inside existing link text', () => {
        const resolver = makeResolver({
          'TKT-LXYHQ': { type: 'ticket', title: 'Resolve refs' },
        })
        const result = renderMarkdown('see [TKT-LXYHQ](https://example.com)', resolver)
        expect(result).toContain('href="https://example.com"')
        expect(result).not.toContain('href="/entity/ticket/TKT-LXYHQ"')
      })

      it('renders dangerous titles as escaped text without breaking the link', () => {
        const resolver = makeResolver({
          'TKT-EVIL': { type: 'ticket', title: '<img src=x onerror=alert(1)>' },
        })
        const result = renderMarkdown('see `TKT-EVIL`', resolver)
        // Parse the result to assert against actual DOM structure rather
        // than substring-matching escaped-text payloads.
        const doc = new DOMParser().parseFromString(`<div>${result}</div>`, 'text/html')
        // No live <img> element was injected — only an <a> with text.
        expect(doc.querySelector('img')).toBeNull()
        const a = doc.querySelector('a[href="/entity/ticket/TKT-EVIL"]')
        expect(a).not.toBeNull()
        // The payload survives as text content, never as live markup.
        expect(a?.textContent).toBe('<img src=x onerror=alert(1)>')
        // Defensive: the link node itself carries no event handlers.
        expect(a?.getAttributeNames().some((n) => n.startsWith('on'))).toBe(false)
      })

      it('renders inaccessible targets with a lock affordance and tooltip', () => {
        const resolver = makeResolver({
          'TKT-LOCKED': {
            type: 'ticket',
            title: '',
            inaccessible: true,
            inaccessibleReason: 'git-crypt',
          },
        })
        const result = renderMarkdown('locked: `TKT-LOCKED`', resolver)
        expect(result).toContain('href="/entity/ticket/TKT-LOCKED"')
        // Lock emoji rendered as link text alongside the ID (the title
        // was empty, so the renderer falls back to the bare ID).
        expect(result).toContain('TKT-LOCKED 🔒')
        // Tooltip mirrors PropertyDisplay's inaccessibleTooltip copy.
        expect(result).toContain('title="git-crypt encrypted (run `git-crypt unlock` to read)"')
      })

      it('keeps the readable title alongside the lock when one is supplied', () => {
        // Server may report inaccessible=true while still being able to
        // produce a display title (e.g. the title property is readable
        // but the body is encrypted). The renderer should keep the title
        // as link text and only add the lock affordance afterwards.
        const resolver = makeResolver({
          'TKT-PARTIAL': {
            type: 'ticket',
            title: 'Encrypted Body',
            inaccessible: true,
            inaccessibleReason: 'git-crypt',
          },
        })
        const result = renderMarkdown('locked: `TKT-PARTIAL`', resolver)
        expect(result).toContain('href="/entity/ticket/TKT-PARTIAL"')
        expect(result).toContain('Encrypted Body 🔒')
        // The bare ID is NOT the visible text in this case.
        expect(result).not.toContain('>TKT-PARTIAL 🔒<')
      })

      it('falls back to "inaccessible" tooltip when reason is missing', () => {
        const resolver = makeResolver({
          'TKT-OPAQUE': { type: 'ticket', title: '', inaccessible: true },
        })
        const result = renderMarkdown('`TKT-OPAQUE`', resolver)
        expect(result).toContain('title="inaccessible"')
      })

      it('produces only same-origin entity hrefs (no javascript: or data: URLs)', () => {
        // The href is derived entirely from server-validated (type, id)
        // pairs — there is no path for a resolver to inject a foreign
        // URL scheme. This test guards that invariant against future
        // refactors that might widen the rewriter's input surface.
        const resolver = makeResolver({
          // Pathological type strings would still produce a same-origin
          // path; DOMPurify sanitizes the resulting <a> anyway.
          'TKT-OK': { type: 'ticket', title: 'Fine' },
        })
        const result = renderMarkdown('`TKT-OK`', resolver)
        const doc = new DOMParser().parseFromString(`<div>${result}</div>`, 'text/html')
        const a = doc.querySelector('a')
        expect(a).not.toBeNull()
        const href = a?.getAttribute('href') ?? ''
        // Same-origin path only.
        expect(href.startsWith('/entity/')).toBe(true)
        expect(href).not.toMatch(/^javascript:/i)
        expect(href).not.toMatch(/^data:/i)
        // No on* event handlers on the link.
        const handlerAttrs = a?.getAttributeNames().filter((n) => n.startsWith('on')) ?? []
        expect(handlerAttrs).toEqual([])
      })

      it('cannot inject script via a malicious inaccessible-reason tooltip', () => {
        // Inaccessible tooltip is built from the reason string. Verify a
        // payload in the reason cannot break out of the attribute and
        // inject markup — the worst case is a stripped `title` attribute
        // (DOMPurify's choice), which is still safe.
        const resolver = makeResolver({
          'TKT-EVIL-TIP': {
            type: 'ticket',
            title: '',
            inaccessible: true,
            inaccessibleReason: 'x"><script>alert(1)</script>',
          },
        })
        const result = renderMarkdown('`TKT-EVIL-TIP`', resolver)
        const doc = new DOMParser().parseFromString(`<div>${result}</div>`, 'text/html')
        // No script element survived.
        expect(doc.querySelector('script')).toBeNull()
        // The link is still rendered safely with the lock affordance.
        const a = doc.querySelector('a[href="/entity/ticket/TKT-EVIL-TIP"]')
        expect(a).not.toBeNull()
        expect(a?.textContent).toContain('🔒')
      })

      it('swallows resolver exceptions and leaves the code span intact', () => {
        const resolver = () => {
          throw new Error('boom')
        }
        const result = renderMarkdown('`TKT-WHATEVER`', resolver)
        expect(result).toContain('<code>TKT-WHATEVER</code>')
      })

      it('handles mixed known + unknown spans in the same paragraph', () => {
        const resolver = makeResolver({
          'TKT-LXYHQ': { type: 'ticket', title: 'Resolve refs' },
        })
        const result = renderMarkdown(
          '`TKT-LXYHQ` is real, `TKT-NOPE` is not',
          resolver,
        )
        expect(result).toContain('href="/entity/ticket/TKT-LXYHQ"')
        expect(result).toContain('<code>TKT-NOPE</code>')
      })

      it('behaves like today when no resolver is supplied', () => {
        const result = renderMarkdown('see `TKT-LXYHQ`')
        expect(result).toContain('<code>TKT-LXYHQ</code>')
        expect(result).not.toContain('href=')
      })

      it('rewrites a self-reference like any other entity link', () => {
        // The renderer is symmetric: self-references resolve through the
        // same path. The destination route is a no-op navigation but the
        // affordance is harmless and matches the Lua-side semantics.
        const resolver = makeResolver({
          'TKT-SELF': { type: 'ticket', title: 'Mirror' },
        })
        const result = renderMarkdown('this is `TKT-SELF`', resolver)
        expect(result).toContain('href="/entity/ticket/TKT-SELF"')
        expect(result).toContain('Mirror')
      })
    })
  })

  describe('getCheckboxStats', () => {
    it('returns null for empty content', () => {
      expect(getCheckboxStats('')).toBeNull()
    })

    it('returns null for content without checkboxes', () => {
      expect(getCheckboxStats('No checkboxes here')).toBeNull()
    })

    it('counts unchecked checkboxes', () => {
      const result = getCheckboxStats('- [ ] task 1\n- [ ] task 2')
      expect(result).toEqual({ checked: 0, total: 2 })
    })

    it('counts checked checkboxes with lowercase x', () => {
      const result = getCheckboxStats('- [x] done 1\n- [x] done 2')
      expect(result).toEqual({ checked: 2, total: 2 })
    })

    it('counts checked checkboxes with uppercase X', () => {
      const result = getCheckboxStats('- [X] done 1\n- [X] done 2')
      expect(result).toEqual({ checked: 2, total: 2 })
    })

    it('counts mixed checkboxes', () => {
      const result = getCheckboxStats('- [x] done\n- [ ] todo\n- [X] also done')
      expect(result).toEqual({ checked: 2, total: 3 })
    })

    it('handles indented checkboxes', () => {
      const result = getCheckboxStats('  - [ ] indented task')
      expect(result).toEqual({ checked: 0, total: 1 })
    })

    it('ignores non-checkbox list items', () => {
      const result = getCheckboxStats('- normal item\n- [ ] checkbox item')
      expect(result).toEqual({ checked: 0, total: 1 })
    })

    it('counts checkboxes across the marked-accepted bullet set', () => {
      // Same set as the toggler (parseCheckboxLine in checkboxToggle.ts):
      // `-`, `*`, `+`, and `N.`. The counter and the toggler MUST agree
      // or the (n/m) widget and the click-handler disagree on which
      // checkbox is which.
      const result = getCheckboxStats('- [x] a\n* [ ] b\n+ [x] c\n1. [ ] d')
      expect(result).toEqual({ checked: 2, total: 4 })
    })
  })

  describe('renderMermaidDiagrams', () => {
    beforeEach(() => {
      vi.clearAllMocks()
    })

    it('handles container without mermaid blocks', async () => {
      const container = document.createElement('div')
      container.innerHTML = '<p>No mermaid here</p>'

      await renderMermaidDiagrams(container)

      expect(container.innerHTML).toBe('<p>No mermaid here</p>')
    })

    it('finds mermaid code blocks', async () => {
      const container = document.createElement('div')
      container.innerHTML = '<pre><code class="language-mermaid">graph TD\nA-->B</code></pre>'

      // Mock mermaid.render to avoid actual rendering
      const mermaid = await import('mermaid')
      vi.spyOn(mermaid.default, 'render').mockResolvedValue({
        svg: '<svg>mocked</svg>',
        diagramType: 'flowchart',
        bindFunctions: vi.fn(),
      })

      await renderMermaidDiagrams(container)

      expect(container.querySelector('.mermaid-diagram')).toBeTruthy()
    })

    it('handles mermaid render errors gracefully', async () => {
      const container = document.createElement('div')
      container.innerHTML = '<pre><code class="language-mermaid">invalid</code></pre>'

      const mermaid = await import('mermaid')
      vi.spyOn(mermaid.default, 'render').mockRejectedValue(new Error('Parse error'))
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      await renderMermaidDiagrams(container)

      expect(consoleSpy).toHaveBeenCalled()
      // Original code block should remain
      expect(container.querySelector('pre code.language-mermaid')).toBeTruthy()
    })

    // The rela-server document renderer emits pre.mermaid (htmlutil
    // pre-converts language-mermaid blocks). The util must handle both
    // marked.js's form and this pre-rewritten form.
    it('finds pre.mermaid blocks from rela document renderer', async () => {
      const container = document.createElement('div')
      container.innerHTML = '<pre class="mermaid">graph TD\nA--&gt;B</pre>'

      const mermaid = await import('mermaid')
      const renderSpy = vi.spyOn(mermaid.default, 'render').mockResolvedValue({
        svg: '<svg>server-rendered</svg>',
        diagramType: 'flowchart',
        bindFunctions: vi.fn(),
      })

      await renderMermaidDiagrams(container)

      expect(renderSpy).toHaveBeenCalled()
      // textContent from the pre gets passed to mermaid; HTML entities
      // have been decoded by the browser before we see them.
      expect(renderSpy.mock.calls[0][1]).toBe('graph TD\nA-->B')
      expect(container.querySelector('.mermaid-diagram')).toBeTruthy()
      expect(container.querySelector('pre.mermaid')).toBeNull()
    })
  })
})
