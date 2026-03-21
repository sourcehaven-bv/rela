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
 */
export function renderMarkdown(content: string): string {
  if (!content) return ''

  const rawHtml = marked.parse(content, {
    gfm: true,
    breaks: true,
  }) as string

  return DOMPurify.sanitize(rawHtml)
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
