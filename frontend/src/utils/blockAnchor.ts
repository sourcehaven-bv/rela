/**
 * Comment anchors for body blocks that cannot be text-selected (TKT-FIO205).
 *
 * An image, a mermaid diagram or a PlantUML diagram renders as an `<img>` or an
 * `<svg>`: there is no text to select, so select-to-comment cannot reach them.
 *
 * # Why this needs no new anchor kind
 *
 * The rendered output has no text, but the SOURCE always does — an image is
 * `![alt](url)` and a diagram is a fenced block with a body. Anchoring to that
 * source text reuses the existing `text` kind exactly as prose does, which
 * means no new server-side kind, no storage migration, and drift handling
 * (re-resolution, the detached flag) comes for free.
 *
 * What the block affordance supplies is a QUOTE the user could not select
 * themselves. Everything downstream is the ordinary text path.
 */

/** Blocks that get a comment affordance, in DOM-query form. */
const COMMENTABLE_BLOCKS = [
  'img',
  // The rendered-mermaid wrapper carries `data-source`; the bare <svg> inside
  // it does not, so match the wrapper (see renderMermaidDiagrams).
  '.mermaid-diagram',
  'svg[id^="mermaid"]',
  '.mermaid',
  'pre.mermaid',
  '.plantuml-diagram-wrapper',
].join(',')

/** A commentable block found in the rendered body. */
export interface CommentableBlock {
  el: HTMLElement
  /** Source text to anchor on — the quote sent to the server. */
  quote: string
  /** Human label for the affordance, e.g. "image" or "diagram". */
  kind: 'image' | 'diagram'
}

/**
 * Finds the source text for a rendered block.
 *
 * Returns null when the block cannot be traced back to source — a diagram
 * whose fence was consumed by the renderer with nothing recoverable, say. A
 * null means "offer no affordance", which is the safe direction: an anchor
 * built on a guess would attach a comment to the wrong place.
 */
export function blockQuote(el: HTMLElement, source: string): string | null {
  // A diagram keeps its source on the element when the renderer stashes it,
  // which is the most reliable handle we have — and for a rendered mermaid SVG
  // it is the ONLY one, since the <pre> it replaced is gone.
  const stashed = el.dataset.diagramSource || el.getAttribute('data-source')
  if (stashed && source.includes(stashed.trim())) return stashed.trim()

  // A mermaid SVG replaced a <pre> whose text was the fence body. The wrapper
  // keeps that text on the container in rela's renderer.
  const pre = el.closest('pre')
  if (pre?.textContent && source.includes(pre.textContent.trim())) {
    return pre.textContent.trim()
  }

  // An image: find its markdown by URL, which is unique enough in practice and
  // present verbatim in the source.
  const src = el.getAttribute('src')
  if (src) {
    const found = findImageMarkdown(source, src)
    if (found) return found
  }

  return null
}

/**
 * Locates the markdown for an image with the given src.
 *
 * Matches `![alt](url …)` on the URL rather than the alt text: alt is often
 * empty, and the URL is what makes the reference unique. The optional title
 * after the URL is included so the quote covers the whole image expression.
 */
function findImageMarkdown(source: string, src: string): string | null {
  const idx = source.indexOf(src)
  if (idx === -1) return null

  // Walk back to the opening `![` and forward to the closing `)`.
  const open = source.lastIndexOf('![', idx)
  if (open === -1) return null
  const close = source.indexOf(')', idx)
  if (close === -1) return null

  const candidate = source.slice(open, close + 1)
  // Guard against spanning unrelated content when the URL appears twice.
  return candidate.includes(src) ? candidate : null
}

/**
 * Finds every commentable block in a rendered body, paired with its source.
 *
 * Blocks whose source cannot be located are omitted rather than offered with a
 * guessed anchor.
 */
export function findCommentableBlocks(container: HTMLElement, source: string): CommentableBlock[] {
  const out: CommentableBlock[] = []
  const seen = new Set<Element>()

  for (const el of container.querySelectorAll<HTMLElement>(COMMENTABLE_BLOCKS)) {
    // An <svg> and its wrapper both match; keep the outermost so a diagram
    // gets one affordance rather than two stacked on the same pixels.
    const wrapper =
      el.closest<HTMLElement>('.mermaid-diagram, .plantuml-diagram-wrapper, .mermaid, pre') ?? el
    if (seen.has(wrapper)) continue
    seen.add(wrapper)

    const quote = blockQuote(el, source)
    if (!quote) continue

    out.push({
      el: wrapper,
      quote,
      kind:
        wrapper.tagName.toLowerCase() === 'img' && !wrapper.closest('.plantuml-diagram-wrapper')
          ? 'image'
          : 'diagram',
    })
  }
  return out
}
