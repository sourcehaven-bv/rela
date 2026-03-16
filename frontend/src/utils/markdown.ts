import { marked } from 'marked'
import mermaid from 'mermaid'

// Initialize mermaid
mermaid.initialize({
  startOnLoad: false,
  theme: 'default',
  securityLevel: 'loose',
})

// Counter for unique mermaid diagram IDs
let mermaidCounter = 0

/**
 * Render markdown to HTML with GFM support.
 * Returns HTML string synchronously (mermaid diagrams are placeholders).
 */
export function renderMarkdown(content: string): string {
  if (!content) return ''

  return marked.parse(content, {
    gfm: true,
    breaks: true,
  }) as string
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
