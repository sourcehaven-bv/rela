import postcss, { type AtRule, type Rule } from 'postcss'

/**
 * Wraps rela's emitted CSS in a single `@layer rela { … }`, so that an
 * operator's unlayered `custom.css` wins the cascade.
 *
 * ## Why this exists
 *
 * Operator CSS is served at `/_custom/custom.css` (from the project's custom/
 * directory) and injected as a `<link>` in
 * `<head>`. That is NOT enough on its own. The production build emits ~19 CSS
 * files: one eager `index-*.css` linked from `index.html`, plus one per
 * route-level chunk. Vite appends the chunk stylesheets to `<head>` at runtime
 * (`__vitePreload` → `document.head.appendChild(link)`), i.e. *after* the
 * injected operator link. At equal specificity the later sheet wins, so
 * operator CSS lost every tie against a route view — a skin worked on the
 * dashboard and silently died on a list view.
 *
 * Cascade layers fix this at a level source order cannot reach: an unlayered
 * declaration beats a layered one regardless of order OR specificity. So
 * operator CSS wins even when it is less specific and loads first.
 *
 * ## Two deliberate carve-outs
 *
 * 1. Top-level PALETTE rules are EXCLUDED — a rule whose body is nothing but
 *    custom-property declarations. They carry the design-token contract, which
 *    is byte-identical to `internal/dataentry/apps_tokens.css` and served to
 *    custom-app iframes as `_rela.css`. Inside an app iframe there is no other
 *    rela CSS, so layering the tokens would not order them against anything —
 *    it would merely demote them beneath every unlayered rule the app author
 *    writes, weakening the contract in exactly the place it exists to serve.
 *    Keeping them unlayered makes one file behave identically in both
 *    environments. Pinned by `TestTokensCSSNeverLayered` (Go) and
 *    `TestBuiltCSSIsLayered` (build output).
 *
 *    **The test is the BODY, not the selector, and that is load-bearing.**
 *    This used to match `:root` and `:root.dark` by name. rela-components
 *    selects its dark palette three other ways — a bare `.dark` (which also
 *    themes a subtree, not just the document), and `:root:not(.light)` inside
 *    `@media (prefers-color-scheme: dark)`. None matched, so the dark tokens
 *    were layered while the light `:root` ones were not, and an unlayered
 *    declaration beats a layered one at ANY specificity: the light palette won
 *    even with `.dark` on the html element. The app rendered light in dark
 *    mode with the correct class applied, which looks like a JavaScript bug
 *    and is not one.
 *
 *    A palette rule nested in `@media`/`@supports` is carved out WITH its
 *    at-rule, since `prefers-color-scheme` is how a palette reaches the OS
 *    preference. A conditional rule that sets ordinary properties is a
 *    component override, not a palette, and stays in the layer.
 *
 *    A rule that sets BOTH is SPLIT rather than rejected. The minifier merges
 *    adjacent rules that share a selector, and `dark.css` has two `.dark`
 *    blocks: the palette, and a paint rule (`color`/`background`) that gives a
 *    dark SUBTREE its own background. Minified they become one rule declaring
 *    tokens and ordinary properties together — so "carve out only pure
 *    palettes" silently layered the whole dark palette again, exactly the
 *    failure this carve-out was widened to fix. Splitting keeps both halves
 *    where they belong and does not depend on how the source happens to be
 *    authored or how aggressively it is minified.
 *
 * 2. `!important` INVERTS under layers: a layered `!important` beats an
 *    unlayered one. So rela's own `!important` rules still beat an operator's
 *    `!important`. That is a permanent property of this design, not a bug —
 *    it is documented in docs/customisation.md.
 *
 * ## Why postcss and not a regex
 *
 * A regex over `\{[^}]*\}` cannot see comments, strings, or nesting, and it
 * silently corrupted real inputs: `:root{/* } *\/--a:1}` hoisted half a block
 * and spliced `@layer rela {` into the middle of a comment, and a nested
 * `:root{&.x{}}` produced unbalanced output. It also wrapped `@charset` and
 * `@import`, which are ILLEGAL inside `@layer` and are dropped by browsers.
 * Parsing removes that whole class of bug by construction.
 *
 * Build-only: `generateBundle` is a Rollup build hook and does not run under
 * `vite` dev server, so `npm run dev` has no layer. Verify cascade changes
 * against `npm run build`. (`npm run build:e2e` IS a real `vite build`, so the
 * e2e suite does exercise the layer.)
 */
export const RELA_LAYER = 'rela'

/** Statement at-rules that MUST precede any `@layer` block in a stylesheet. */
const PRELUDE_AT_RULES = new Set(['charset', 'import', 'namespace'])

/** Conditional at-rules a palette may legitimately sit inside. */
const CONDITIONAL_AT_RULES = new Set(['media', 'supports'])

/**
 * True for a selector that themes the document (or a whole subtree of it)
 * rather than picking out particular elements.
 *
 * `:root`, `:root.dark`, `:root:not(.light)` and a bare theme class like
 * `.dark` qualify. `:root .fa-rotate-90` (a descendant) and `.x` in
 * `:root, .x` do not — those set tokens on specific components, and carving
 * them out would exempt ordinary component CSS from the layer.
 *
 * A bare class is admitted because rela-components uses `.dark` to theme a
 * SUBTREE, not just the html element; requiring `:root` would layer it.
 */
function isDocumentScope(sel: string): boolean {
  return /^(:root|\.[\w-]+)(:not\([.\w-]+\)|\.[\w-]+)*$/.test(sel)
}

/**
 * A palette targets ONE scope. Every palette in the tree is a single
 * selector (`:root`, `.dark`, `:root:not(.light)`), so a comma list is
 * evidence the rule is setting tokens on components — `:root, .x` would
 * otherwise carve `.x` out of the layer along with the document.
 */
function isSingleScope(node: Rule): boolean {
  return node.selectors.length === 1
}

/**
 * Non-custom properties a palette may set. `color-scheme` tells the browser
 * which built-in light/dark treatment to use for form controls, scrollbars
 * and the canvas, so it belongs with the palette that decides the theme —
 * both `tokens.css` and `dark.css` set it beside their tokens. Layering it
 * away from them would split one decision across two cascade levels.
 */
const PALETTE_PROPS = new Set(['color-scheme'])

/** True for a declaration that belongs to the token contract. */
function isPaletteDecl(prop: string): boolean {
  return prop.startsWith('--') || PALETTE_PROPS.has(prop.toLowerCase())
}

/**
 * Splits a rule into its palette half and its remainder, or returns `null`
 * when it holds no palette at all.
 *
 * Beyond requiring a document-scope selector, a palette is recognised by what
 * it SETS. See the carve-out note above for why matching selector names
 * instead silently un-themed dark mode, and why a MIXED rule is split rather
 * than rejected.
 *
 * A rule declaring no custom properties is not a palette — there is no token
 * contract to protect, so carving it out would serve nothing.
 */
function splitPalette(node: Rule): { palette: Rule; rest: Rule | null } | null {
  if (!isSingleScope(node)) return null
  if (!isDocumentScope(node.selectors[0].trim())) return null

  const palette = node.clone()
  palette.removeAll()
  const rest = node.clone()
  rest.removeAll()

  let sawToken = false
  for (const child of node.nodes ?? []) {
    if (child.type === 'decl' && isPaletteDecl(child.prop)) {
      if (child.prop.startsWith('--')) sawToken = true
      palette.append(child.clone())
    } else if (child.type !== 'comment') {
      rest.append(child.clone())
    }
  }

  if (!sawToken) return null
  return { palette, rest: rest.nodes?.length ? rest : null }
}

/**
 * Splits a top-level conditional at-rule (`@media`/`@supports`) into its
 * palette half and its remainder — how a palette reaches
 * `prefers-color-scheme`. Returns `null` when it carries no palette.
 */
function splitPaletteAtRule(node: AtRule): { palette: AtRule; rest: AtRule | null } | null {
  if (!CONDITIONAL_AT_RULES.has(node.name.toLowerCase())) return null

  const palette = node.clone()
  palette.removeAll()
  const rest = node.clone()
  rest.removeAll()

  for (const child of node.nodes ?? []) {
    if (child.type === 'comment') continue
    const split = child.type === 'rule' ? splitPalette(child) : null
    if (split) {
      palette.append(split.palette)
      if (split.rest) rest.append(split.rest)
    } else {
      rest.append(child.clone())
    }
  }

  if (!palette.nodes?.length) return null
  return { palette, rest: rest.nodes?.length ? rest : null }
}

/**
 * Splits a stylesheet into the parts that must stay unlayered — leading
 * `@charset`/`@import`/`@namespace` statements and the top-level palette rules
 * (including a `@media`/`@supports` block that holds only palettes) — and
 * everything else, which is wrapped in `@layer rela`.
 *
 * Returns `null` when the source is already layered.
 */
export function wrapCss(source: string): string | null {
  if (source.includes(`@layer ${RELA_LAYER}`)) return null

  const root = postcss.parse(source)
  const prelude: AtRule[] = []
  const tokens: (Rule | AtRule)[] = []
  const layer = postcss.atRule({ name: 'layer', params: RELA_LAYER })

  // One pass: each top-level node is prelude, palette, layered, or — for a
  // mixed rule the minifier merged — split across the last two.
  root.each((node) => {
    if (node.type === 'atrule' && PRELUDE_AT_RULES.has(node.name.toLowerCase())) {
      prelude.push(node)
      return
    }

    const split =
      node.type === 'rule'
        ? splitPalette(node)
        : node.type === 'atrule'
          ? splitPaletteAtRule(node)
          : null

    if (split) {
      tokens.push(split.palette)
      if (split.rest) layer.append(split.rest)
    } else {
      layer.append(node.clone())
    }
  })

  // Rebuild: prelude first (they must precede any @layer), then the bare
  // `@layer rela;` declaration that PINS layer order at first parse — with 18
  // runtime-appended chunks, whichever loads first would otherwise establish
  // the layer's position — then the unlayered tokens, then the layer itself.
  const out = postcss.root()
  prelude.forEach((n) => out.append(n))
  out.append(postcss.atRule({ name: 'layer', params: RELA_LAYER }))
  tokens.forEach((n) => out.append(n))
  out.append(layer)

  return out.toString()
}
