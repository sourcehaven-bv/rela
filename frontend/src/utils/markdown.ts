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

  // Allowlist the attributes rela adds to rendered markdown. DOMPurify strips
  // anything not listed, silently — so a new marker attribute that is not added
  // here simply vanishes, with no error to debug.
  //
  // `mark` itself needs no ADD_TAGS: it is in DOMPurify's default allowlist.
  // The comment attributes carry a server-minted id and a boolean; neither is
  // a URL or a script sink.
  return DOMPurify.sanitize(rawHtml, {
    ADD_ATTR: ['data-cb-idx', 'data-comment-id', 'data-comment-uncertain', 'data-comment-chip'],
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
// Re-exported from checkboxToggle so the stats counter and the click-handler
// route through the exact same line-parser. A divergence between the two
// would silently mis-align the (n/m) widget with which line a click toggles.
export { checkboxStats as getCheckboxStats } from './checkboxToggle'

// Sanitizer instance dedicated to mermaid output. It carries the label-lifting
// hook below, which must not run for renderMarkdown's ordinary HTML, so it
// cannot share the default DOMPurify instance.
const mermaidPurifier = DOMPurify(window)

// Marker pairing a lifted label with its foreignObject across the SVG pass.
// The hook strips it from EVERY element on entry, so input cannot spoof a
// slot and have someone else's label written into an element of its choosing;
// it is removed again before the element reaches the document.
const LABEL_SLOT_ATTR = 'data-rela-label-slot'

// Labels lifted out during the current sanitizeMermaidSVG call. Module-level
// because a DOMPurify hook has no other channel back to its caller; sanitize()
// is synchronous, so a call only ever observes its own entries.
let liftedLabels: string[] = []

mermaidPurifier.addHook('uponSanitizeElement', (node, data) => {
  if (node.nodeType !== Node.ELEMENT_NODE) return
  const el = node as Element
  el.removeAttribute(LABEL_SLOT_ATTR)
  if (data.tagName !== 'foreignobject') return
  el.setAttribute(LABEL_SLOT_ATTR, String(liftedLabels.length))
  liftedLabels.push(el.innerHTML)
  el.textContent = ''
})

// Element types that never belong in a diagram label. A label is formatted
// text, so dropping embedded media costs nothing legitimate and closes the one
// thing strict mermaid still lets through from a hostile label: the event
// handler is stripped but the element survives, which is enough for a request
// to an attacker-chosen URL when the diagram is viewed.
const LABEL_FORBID_TAGS = ['img', 'picture', 'source', 'video', 'audio', 'iframe', 'object', 'embed']

/**
 * Sanitize a mermaid-produced SVG and return it as a detached element.
 *
 * Mermaid runs with `securityLevel: 'strict'`, so it already escapes diagram
 * labels. This is defence in depth, and it is not theoretical: with a hostile
 * label, strict mermaid emits no event-handler attributes but DOES emit a live
 * `<img>` element built from the label text. It also means a future
 * `securityLevel: 'loose'` — a one-line change, and the setting you need for
 * clickable nodes — cannot silently turn user-authored diagram source into
 * stored XSS.
 *
 * ## Why this is two passes and not one `DOMPurify.sanitize(svg)`
 *
 * Mermaid renders flowchart and state-diagram labels as XHTML
 * (`<div><span class="nodeLabel"><p>…<br>…</p></span></div>`) inside an SVG
 * `<foreignObject>`. DOMPurify 3.4 cannot retain that subtree in a single
 * call, in any configuration — measured, not assumed:
 *
 *  - With the SVG profile alone the HTML elements are not allowed, so they are
 *    unwrapped: only their text survives. `Line1<br>Line2` becomes
 *    `Line1Line2`, the `.nodeLabel` styling hook is gone, and bare text is not
 *    valid `foreignObject` content, so it may not paint at all.
 *  - Allowing them (the `html` profile, or `ADD_TAGS`) is WORSE: DOMPurify's
 *    namespace check runs after the allowed-tags check and force-removes an
 *    allowed HTML element under an SVG parent, children and all. The label
 *    vanishes entirely.
 *
 * Either way shapes and arrows still render, so the diagram looks fine and
 * reads as empty or run-together boxes. Sequence diagrams use SVG `<text>` and
 * are NOT affected, so checking one diagram type hides it.
 *
 * So the label subtrees are lifted out by a hook DURING the SVG pass (the raw
 * string is only ever handed to DOMPurify — there is no pre-parse, which is
 * what a previous version did and what turned the untrusted text into DOM
 * before any sanitizer saw it), then each label is sanitized on its own under
 * the HTML profile and written back into its emptied `foreignObject`. Setting
 * `innerHTML` on a foreignObject parses in the XHTML namespace, exactly as
 * mermaid itself does. Both halves are sanitized; the split is about parsing
 * context, not exemption.
 *
 * Input whose sanitized root is not an `<svg>` (a parse failure, or text that
 * was never a diagram) yields an empty host rather than rendering the literal
 * string on the page.
 */
function sanitizeMermaidSVG(svg: string): HTMLElement {
  const host = document.createElement('div')
  host.className = 'mermaid-diagram'

  liftedLabels = []
  const fragment = mermaidPurifier.sanitize(svg, {
    USE_PROFILES: { svg: true, svgFilters: true },
    ADD_TAGS: ['foreignObject'],
    ADD_ATTR: [LABEL_SLOT_ATTR],
    RETURN_DOM_FRAGMENT: true,
  })
  const labels = liftedLabels
  liftedLabels = []

  const root = fragment.firstElementChild
  if (!root || root.nodeName.toLowerCase() !== 'svg') return host
  host.appendChild(fragment)

  for (const slot of Array.from(host.querySelectorAll(`[${LABEL_SLOT_ATTR}]`))) {
    const i = Number(slot.getAttribute(LABEL_SLOT_ATTR))
    slot.removeAttribute(LABEL_SLOT_ATTR)
    if (slot.nodeName.toLowerCase() !== 'foreignobject') continue
    if (!Number.isInteger(i) || i < 0 || i >= labels.length) continue
    slot.innerHTML = DOMPurify.sanitize(labels[i], {
      USE_PROFILES: { html: true },
      FORBID_TAGS: LABEL_FORBID_TAGS,
    })
  }
  return host
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
      pre.replaceWith(sanitizeMermaidSVG(svg))
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

/**
 * Encode PlantUML source for a server `~h` (hex) request.
 *
 * PlantUML servers accept three transfer encodings; we use the hex form
 * (`~h<hex>`) because it needs no deflate/compression library — the diagram
 * source's UTF-8 bytes are simply lowercased-hex encoded. URLs are longer than
 * the deflate form but require zero extra frontend dependencies and stay well
 * within typical URL limits for hand-authored diagrams.
 */
export function encodePlantUMLHex(source: string): string {
  const bytes = new TextEncoder().encode(source)
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
  return `~h${hex}`
}

/**
 * Build the diagram URL for a PlantUML server and encoded source, or null when
 * the configured base URL is unsafe to use as an <img src>.
 *
 * The server URL is validated server-side at config load (see
 * dataentryconfig.validateApp), but we re-check the scheme here as defense in
 * depth: this value becomes a live `img.src` with no CSP backstop on the SPA,
 * so a non-http(s) scheme that somehow reaches the client must never be
 * emitted. Joins without doubling the slash regardless of a trailing slash on
 * the base URL.
 */
function plantUMLImageURL(serverURL: string, source: string): string | null {
  let parsed: URL
  try {
    parsed = new URL(serverURL)
  } catch {
    return null
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null
  const base = serverURL.replace(/\/+$/, '')
  return `${base}/svg/${encodePlantUMLHex(source)}`
}

/**
 * Render PlantUML diagrams by replacing fenced ```plantuml blocks with an
 * <img> pointed at the configured PlantUML server. Mirrors
 * renderMermaidDiagrams: call it after the markdown HTML is mounted, and it
 * handles the same two source forms —
 * - `<pre><code class="language-plantuml">…</code></pre>` (marked.js, client
 *   render of entity/section content), and
 * - `<pre class="plantuml">…</pre>` (rela-server documents, goldmark +
 *   htmlutil.ConvertPlantUMLBlocks).
 *
 * `serverURL` is the operator-configured `app.plantuml_server_url`. When it is
 * empty/undefined the function is a no-op: blocks are left as plain code, no
 * diagram source leaves the browser, and no network request is made. Presence
 * of the URL is therefore the on/off switch for the whole feature.
 *
 * Unlike mermaid, rendering happens on the server behind `serverURL`, so this
 * pass only constructs the <img> element; the browser fetches the SVG lazily.
 *
 * Returns the number of blocks replaced (0 when disabled or none found).
 */
export function renderPlantUMLDiagrams(
  container: HTMLElement,
  serverURL: string | undefined | null,
): number {
  if (!serverURL) return 0

  type Target = { pre: Element; source: string }
  const targets: Target[] = []

  // Form 1: marked.js-style fenced blocks.
  for (const codeBlock of container.querySelectorAll('pre > code.language-plantuml')) {
    const pre = codeBlock.parentElement
    if (pre) targets.push({ pre, source: codeBlock.textContent || '' })
  }

  // Form 2: rela-server's pre-rewritten blocks. The `code` child can never be
  // present here (goldmark emits Form 1, the server rewrite produces a bare
  // <pre class="plantuml">, and the two don't co-occur), but the guard keeps
  // Form 2 from re-wrapping a node Form 1 already claimed if that invariant
  // ever changes.
  for (const pre of container.querySelectorAll('pre.plantuml')) {
    if (pre.querySelector('code.language-plantuml')) continue
    targets.push({ pre, source: pre.textContent || '' })
  }

  let rendered = 0
  for (const { pre, source } of targets) {
    const trimmed = source.trim()
    if (!trimmed) continue
    const url = plantUMLImageURL(serverURL, trimmed)
    if (!url) continue // unsafe server URL — leave the block as plain code.

    const img = document.createElement('img')
    img.className = 'plantuml-diagram'
    img.loading = 'lazy'
    img.alt = 'PlantUML diagram'
    // Stateless render request: never leak the rela page URL to the server.
    img.referrerPolicy = 'no-referrer'
    // On any load failure (server down, 414 from an oversized hex URL,
    // non-image response) restore a readable code block instead of leaving a
    // broken-image glyph with the source gone. Mirrors renderMermaidDiagrams'
    // "leave the source on error" resilience.
    img.addEventListener('error', () => {
      const fallback = document.createElement('pre')
      const code = document.createElement('code')
      code.className = 'language-plantuml'
      code.textContent = trimmed
      fallback.appendChild(code)
      img.closest('.plantuml-diagram-wrapper')?.replaceWith(fallback)
    })
    // Layout shifts when the (lazy) image actually loads; signal scroll-settle
    // listeners the same way mermaid does after its SVG injection.
    img.addEventListener('load', () => {
      container.dispatchEvent(new CustomEvent('rela:plantuml-rendered', { bubbles: true }))
    })
    img.src = url

    const wrapper = document.createElement('div')
    wrapper.className = 'plantuml-diagram-wrapper'
    wrapper.appendChild(img)
    pre.replaceWith(wrapper)
    rendered++
  }

  return rendered
}
