import { marked } from 'marked'
import mermaid from 'mermaid'
import DOMPurify from 'dompurify'

// Initialize mermaid with strict security
mermaid.initialize({
  startOnLoad: false,
  theme: 'default',
  securityLevel: 'strict',
})

// Counter for unique mermaid diagram IDs
let mermaidCounter = 0

/**
 * Render markdown to HTML with GFM support.
 * Returns sanitized HTML string (mermaid diagrams are placeholders).
 * Checkboxes get data-cb-idx attributes for toggle support.
 */
export function renderMarkdown(content: string): string {
  if (!content) return ''

  const rawHtml = marked.parse(content, {
    gfm: true,
    breaks: true,
  }) as string

  // Add data-cb-idx to checkboxes for toggle support
  let cbIdx = 0
  const htmlWithCbIdx = rawHtml.replace(
    /<input\s+type="checkbox"([^>]*)>/gi,
    (_match, attrs) => `<input type="checkbox" data-cb-idx="${cbIdx++}"${attrs}>`
  )

  // Allow data-cb-idx attribute through DOMPurify
  return DOMPurify.sanitize(htmlWithCbIdx, {
    ADD_ATTR: ['data-cb-idx'],
  })
}

/**
 * Get checkbox stats from content (checked/total).
 */
export function getCheckboxStats(content: string): { checked: number; total: number } | null {
  if (!content) return null

  const checkboxPattern = /^\s*- \[([ xX])\]/gm
  const matches = content.match(checkboxPattern)
  if (!matches || matches.length === 0) return null

  const total = matches.length
  const checked = matches.filter((m) => /\[[xX]\]/.test(m)).length

  return { checked, total }
}

/**
 * Render markdown and then process mermaid diagrams.
 * Call this when the content is mounted in the DOM.
 */
export async function renderMermaidDiagrams(container: HTMLElement): Promise<void> {
  const codeBlocks = container.querySelectorAll('pre > code.language-mermaid')

  for (const codeBlock of codeBlocks) {
    const pre = codeBlock.parentElement
    if (!pre) continue

    const code = codeBlock.textContent || ''
    const id = `mermaid-${++mermaidCounter}`

    try {
      const { svg } = await mermaid.render(id, code)
      const div = document.createElement('div')
      div.className = 'mermaid-diagram'
      div.innerHTML = svg
      pre.replaceWith(div)
    } catch (err) {
      console.error('Mermaid render error:', err)
      // Leave the code block as-is on error
    }
  }
}
