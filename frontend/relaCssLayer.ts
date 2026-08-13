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
 * 1. Top-level `:root` rules are EXCLUDED. They carry the design-token
 *    contract, which is byte-identical to `internal/dataentry/apps_tokens.css`
 *    and served to custom-app iframes as `_rela.css`. Inside an app iframe
 *    there is no other rela CSS, so layering the tokens would not order them
 *    against anything — it would merely demote them beneath every unlayered
 *    rule the app author writes, weakening the contract in exactly the place it
 *    exists to serve. Keeping them unlayered makes one file behave identically
 *    in both environments. Pinned by `TestTokensCSSNeverLayered` (Go) and
 *    `TestBuiltCSSIsLayered` (build output).
 *
 *    Only TOP-LEVEL `:root` rules are carved out. A `:root` nested inside
 *    `@media`/`@supports` is a conditional override of a component, not part of
 *    the token contract, and stays in the layer.
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

/** True for a top-level `:root` / `:root.dark` token rule. */
function isTokenRule(node: Rule): boolean {
  if (node.parent?.type !== 'root') return false
  return node.selectors.every((sel) => /^:root(\.[\w-]+)?$/.test(sel.trim()))
}

/**
 * Splits a stylesheet into the parts that must stay unlayered — leading
 * `@charset`/`@import`/`@namespace` statements and the top-level `:root` token
 * rules — and everything else, which is wrapped in `@layer rela`.
 *
 * Returns `null` when the source is already layered.
 */
export function wrapCss(source: string): string | null {
  if (source.includes(`@layer ${RELA_LAYER}`)) return null

  const root = postcss.parse(source)
  const prelude: AtRule[] = []
  const tokens: Rule[] = []

  root.each((node) => {
    if (node.type === 'atrule' && PRELUDE_AT_RULES.has(node.name.toLowerCase())) {
      prelude.push(node)
    } else if (node.type === 'rule' && isTokenRule(node)) {
      tokens.push(node)
    }
  })

  const layer = postcss.atRule({ name: 'layer', params: RELA_LAYER })
  // Move everything that is not prelude/token into the layer, preserving order.
  root.each((node) => {
    if (prelude.includes(node as AtRule) || tokens.includes(node as Rule)) return
    layer.append(node.clone())
    node.remove()
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
