import { describe, it, expect } from 'vitest'
import { blockQuote, findCommentableBlocks } from './blockAnchor'

/** Builds a detached container from HTML, as the rendered body would look. */
function render(html: string): HTMLElement {
  const el = document.createElement('div')
  el.innerHTML = html
  document.body.appendChild(el)
  return el
}

/**
 * Builds a <pre> whose text is set safely.
 *
 * Mermaid sources contain `-->`, which inside an innerHTML string terminates an
 * HTML comment and silently mangles the parse — the fixture, not the code under
 * test, was what first failed here.
 */
function renderPre(text: string, cls = 'mermaid'): HTMLElement {
  const host = document.createElement('div')
  const pre = document.createElement('pre')
  pre.className = cls
  pre.textContent = text
  host.appendChild(pre)
  document.body.appendChild(host)
  return pre
}

describe('blockQuote', () => {
  it('finds the markdown for an image by its URL', () => {
    const source = 'Intro.\n\n![Placeholder](https://example.com/pic.png "A title")\n\nAfter.'
    const img = render('<img src="https://example.com/pic.png" alt="Placeholder">')
      .firstElementChild as HTMLElement

    expect(blockQuote(img, source)).toBe('![Placeholder](https://example.com/pic.png "A title")')
  })

  it('finds an image with empty alt text', () => {
    // Alt is often empty, which is why the URL is what the lookup keys on.
    const source = 'Text.\n\n![](https://example.com/x.png)\n'
    const img = render('<img src="https://example.com/x.png" alt="">')
      .firstElementChild as HTMLElement

    expect(blockQuote(img, source)).toBe('![](https://example.com/x.png)')
  })

  it('prefers a stashed diagram source when the renderer kept one', () => {
    const diagramSource = 'graph TD\n  A --> B'
    const source = `Before.\n\n\`\`\`mermaid\n${diagramSource}\n\`\`\`\n`
    const el = render('<div class="mermaid"></div>').firstElementChild as HTMLElement
    el.dataset.diagramSource = diagramSource

    expect(blockQuote(el, source)).toBe(diagramSource)
  })

  it('falls back to a pre ancestor for an unrendered diagram fence', () => {
    const body = 'graph TD\n  A --> B'
    const source = `Before.\n\n\`\`\`mermaid\n${body}\n\`\`\`\n`
    const pre = renderPre(body)

    expect(blockQuote(pre, source)).toBe(body)
  })

  it('returns null when the source cannot be located', () => {
    // Better no affordance than one anchored on a guess.
    const img = render('<img src="https://elsewhere.example/none.png">')
      .firstElementChild as HTMLElement

    expect(blockQuote(img, 'A body mentioning no such image.')).toBeNull()
  })
})

describe('findCommentableBlocks', () => {
  it('finds images and diagrams, labelling each', () => {
    const diagramSource = 'graph TD\n  A --> B'
    const source = `![Pic](https://example.com/p.png)\n\n\`\`\`mermaid\n${diagramSource}\n\`\`\`\n`
    const container = render('<p><img src="https://example.com/p.png"></p>')
    const pre = document.createElement('pre')
    pre.className = 'mermaid'
    pre.textContent = diagramSource
    container.appendChild(pre)

    const found = findCommentableBlocks(container, source)

    expect(found).toHaveLength(2)
    expect(found.map((b) => b.kind).sort()).toEqual(['diagram', 'image'])
    expect(found.find((b) => b.kind === 'image')?.quote).toBe('![Pic](https://example.com/p.png)')
  })

  it('omits a block whose source cannot be traced', () => {
    const container = render('<img src="https://elsewhere.example/x.png">')

    expect(findCommentableBlocks(container, 'Unrelated body text.')).toEqual([])
  })

  it('counts a mermaid svg inside a wrapper once', () => {
    // The <svg> and its wrapper both match the selector; offering two
    // affordances on one diagram would be a duplicate control.
    const diagramSource = 'graph TD\n  A --> B'
    const source = `\`\`\`mermaid\n${diagramSource}\n\`\`\`\n`
    const container = render('<div></div>')
    const pre = document.createElement('pre')
    pre.className = 'mermaid'
    pre.setAttribute('data-source', diagramSource)
    pre.appendChild(document.createElement('svg'))
    container.appendChild(pre)

    expect(findCommentableBlocks(container, source)).toHaveLength(1)
  })
})
