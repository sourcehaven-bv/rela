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
  })
})
