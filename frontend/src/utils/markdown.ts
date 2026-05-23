import { Marked, type Tokens } from 'marked'
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
 * Resolution result for an entity-ID code span. Mirrors the server-side
 * `Mention` shape returned in `ViewResponse.mentions` (TKT-747O): the
 * server walks each rendered markdown blob, resolves bare-ID code spans
 * against the store, and surfaces the resulting `{type, title}` map so
 * the renderer can rewrite the code spans into in-app links without a
 * second round-trip.
 */
export interface EntityRef {
  type: string
  title: string
  inaccessible?: boolean
  inaccessibleReason?: string
}

export type EntityRefResolver = (id: string) => EntityRef | null

/**
 * Render markdown to HTML with GFM support.
 * Returns sanitized HTML string (mermaid diagrams are placeholders).
 * Checkboxes get data-cb-idx attributes for toggle support.
 *
 * When a `refResolver` is supplied, inline code spans whose entire text
 * matches an entity ID known to the resolver are rewritten as in-app
 * links titled with the target entity's title. Matches the Lua-side
 * semantics from TKT-LXYHQ exactly: only bare-content code spans are
 * touched; code blocks, link text, and multi-token spans are left
 * alone. Inaccessible targets (e.g. git-crypt encrypted) render as
 * `<a>ID 🔒</a>` with a tooltip mirroring `PropertyDisplay.vue`.
 */
export interface RenderMarkdownOptions {
  refResolver?: EntityRefResolver
  /**
   * When true, render task-list checkboxes as enabled inputs tagged with
   * `data-cb-idx="N"` so a delegated click handler can toggle them via the
   * server. Use this only at call sites that actually wire up the handler
   * (entry-content body in EntityDetail.vue). Defaults to false: checkboxes
   * render `disabled`, matching marked's default, so users in card/content
   * views aren't misled into thinking inert checkboxes are clickable.
   */
  interactive?: boolean
}

export function renderMarkdown(
  content: string,
  refResolverOrOptions?: EntityRefResolver | RenderMarkdownOptions,
): string {
  if (!content) return ''

  const options: RenderMarkdownOptions =
    typeof refResolverOrOptions === 'function'
      ? { refResolver: refResolverOrOptions }
      : refResolverOrOptions ?? {}

  // Per-render Marked instance + counter so each call numbers its own
  // checkboxes from 0. EntityDetail.vue maps data-cb-idx back to the
  // source-line index when toggling; the renderer hook is the only thing
  // producing the attribute, so its sequence is the authoritative one.
  //
  // For interactive renders we emit an enabled `<input>` (no `disabled`)
  // because the browser swallows click events on disabled inputs even when
  // a JS listener is attached. The Vue handler calls `e.preventDefault()`
  // and reloads the view from the server, which re-renders the checkbox
  // with the new state.
  //
  // For non-interactive renders we keep marked's default `disabled` so the
  // checkbox is clearly inert — without that, users get clickable-looking
  // checkboxes whose clicks silently no-op.
  let cbIdx = 0
  const refResolver = options.refResolver
  const instance = new Marked({
    gfm: true,
    // Soft line breaks are whitespace (CommonMark default). Entity content
    // is hard-wrapped at ~80 chars in source markdown; treating each newline
    // as a <br> made HTML mirror the source's column width instead of
    // reflowing to the viewport. Authors who want a hard break use the
    // CommonMark two-trailing-spaces form ("foo  \n").
    breaks: false,
    renderer: {
      checkbox({ checked }) {
        const checkedAttr = checked ? ' checked=""' : ''
        if (!options.interactive) {
          return `<input disabled="" type="checkbox"${checkedAttr}> `
        }
        const idx = cbIdx++
        return `<input data-cb-idx="${idx}" type="checkbox"${checkedAttr}> `
      },
    },
    walkTokens: refResolver
      ? (token) => rewriteEntityRefToken(token, refResolver)
      : undefined,
  })

  const rawHtml = instance.parse(content) as string

  // Allow data-cb-idx attribute through DOMPurify
  return DOMPurify.sanitize(rawHtml, {
    ADD_ATTR: ['data-cb-idx'],
  })
}

// rewriteEntityRefToken mutates a `codespan` token in place into a `link`
// token when its text matches a resolver hit. Token-level rewriting keeps
// the eventual HTML escaping under marked's control: link text flows
// through marked's text-renderer (which HTML-escapes) so a malicious entity
// title cannot break out of the `<a>` element.
function rewriteEntityRefToken(token: unknown, resolve: EntityRefResolver): void {
  if (!isCodespanToken(token)) return
  let hit: EntityRef | null
  try {
    hit = resolve(token.text)
  } catch {
    // Defensive: a resolver bug must never break markdown rendering.
    return
  }
  // Require a type so we can build a stable URL. Title may be empty for
  // an inaccessible target whose display source is unreadable; in that
  // case we fall back to using the ID as link text. For everything else
  // a missing title is the resolver telling us "I don't know enough to
  // present this", and we leave the code span alone.
  if (!hit || !hit.type) return
  const visibleTitle = hit.title || (hit.inaccessible ? token.text : '')
  if (!visibleTitle) return

  const href = `/entity/${hit.type}/${token.text}`
  // Inaccessible targets get a trailing lock affordance; keep the
  // readable title when one was supplied (the lock only conveys "the
  // underlying file is encrypted") and only fall back to the bare ID
  // when the title would otherwise be empty.
  const linkText = hit.inaccessible ? `${visibleTitle} 🔒` : visibleTitle
  const tokenTitle = hit.inaccessible ? inaccessibleTooltipFor(hit.inaccessibleReason) : null

  const rewritten = token as unknown as Tokens.Link & { raw?: string }
  rewritten.type = 'link'
  rewritten.href = href
  rewritten.title = tokenTitle
  rewritten.text = linkText
  rewritten.tokens = [{ type: 'text', raw: linkText, text: linkText } as Tokens.Text]
  // Clear the original codespan `raw` (the backticked source) so any
  // downstream consumer that re-emits markdown sees the link shape, not
  // a stale code-span literal. Marked's HTML renderer doesn't read
  // `raw` on links, but defensive consistency is cheap.
  rewritten.raw = `[${linkText}](${href})`
}

function isCodespanToken(token: unknown): token is Tokens.Codespan {
  return (
    typeof token === 'object' &&
    token !== null &&
    (token as { type?: unknown }).type === 'codespan' &&
    typeof (token as { text?: unknown }).text === 'string'
  )
}

// inaccessibleTooltipFor mirrors PropertyDisplay.vue's inaccessibleTooltip
// helper so users see consistent copy whether the lock indicator appears
// on a property or on an entity-ref link.
function inaccessibleTooltipFor(reason: string | undefined): string {
  if (reason === 'git-crypt') {
    return 'git-crypt encrypted (run `git-crypt unlock` to read)'
  }
  if (reason) return `inaccessible (${reason})`
  return 'inaccessible'
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
 *
 * Handles two source forms:
 * - `<pre><code class="language-mermaid">…</code></pre>` — what marked.js
 *   emits for our in-frontend markdown rendering (property views etc.).
 * - `<pre class="mermaid">…</pre>` — what the rela-server document
 *   renderer emits (goldmark + htmlutil.ConvertMermaidBlocks).
 *
 * Both forms are replaced in place with an SVG diagram (or left as-is
 * on parse error).
 */
export async function renderMermaidDiagrams(container: HTMLElement): Promise<void> {
  type Target = { pre: Element; source: string }
  const targets: Target[] = []

  // Form 1: marked.js-style fenced blocks.
  for (const codeBlock of container.querySelectorAll('pre > code.language-mermaid')) {
    const pre = codeBlock.parentElement
    if (pre) targets.push({ pre, source: codeBlock.textContent || '' })
  }

  // Form 2: rela-server's pre-rewritten blocks. Guard against double-matching
  // Form 1 (which would set class on the code, not the pre).
  for (const pre of container.querySelectorAll('pre.mermaid')) {
    // Already covered by Form 1 if it has a code child. It won't, but
    // being explicit is cheap.
    if (pre.querySelector('code.language-mermaid')) continue
    targets.push({ pre, source: pre.textContent || '' })
  }

  for (const { pre, source } of targets) {
    const id = `mermaid-${++mermaidCounter}`
    try {
      const { svg } = await mermaid.render(id, source)
      const div = document.createElement('div')
      div.className = 'mermaid-diagram'
      div.innerHTML = svg
      pre.replaceWith(div)
    } catch (err) {
      console.error('Mermaid render error:', err)
      // Leave the block as-is on error.
    }
  }

  // Signal completion for scroll-settle listeners. Mermaid's SVG injection
  // shifts layout; components that care about anchor positions (e.g. the
  // router's scroll-to-anchor) re-check their targets on this event rather
  // than polling on an arbitrary interval.
  if (targets.length > 0) {
    container.dispatchEvent(
      new CustomEvent('rela:mermaid-rendered', { bubbles: true }),
    )
  }
}
