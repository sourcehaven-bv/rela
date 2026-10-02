#!/usr/bin/env node
/**
 * Generates `src/styles/tokens.css` — the colour palette served to custom-app
 * iframes as `_rela.css`, and embedded byte-for-byte in the Go binary as
 * `internal/dataentry/apps_tokens.css`.
 *
 * ## Why this is generated rather than written
 *
 * rela used to declare its own palette here, in parallel with
 * rela-components' `--rl-*` tokens. Two palettes cannot be kept in step: the
 * library's dark values moved and rela's did not, so the shell painted light
 * on a dark page. There is one palette now and it is the library's.
 *
 * But an app iframe cannot reach the library's stylesheets — `_rela.css` is
 * the whole contract, served cross-origin, and an `@import` in it would not
 * resolve. So the palette has to be COPIED into this file, and a copy that is
 * maintained by hand is the same drift with an extra step. Generating it makes
 * the library the single source and this file a build artifact.
 *
 * ## What is copied
 *
 * Only the palette rules: `:root` from `tokens.css`, and from `dark.css` both
 * the `.dark` block and the `prefers-color-scheme` block. Declarations that
 * are not custom properties are dropped, except `color-scheme` — a rule that
 * paints (`.dark { background: … }`) is component CSS, not a token contract.
 *
 * Spacing, radius and typography stay OUT: `tokens.css` is colour-only by
 * contract (see the Go-side `appTypographyCSS`, which owns the four frozen
 * `--font-size-*` steps separately), and `scales.css` is not served to apps.
 *
 * Run via `npm run gen:tokens`. CI re-runs it and fails on a diff, so a
 * library palette change cannot land without this file following.
 */
import { readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import postcss from 'postcss'

const local = (rel) => fileURLToPath(new URL(rel, import.meta.url))

const HEADER = `/*
 * GENERATED FILE — do not edit.
 *
 * Run \`npm run gen:tokens\` to regenerate. The source is rela-components'
 * palette (\`packages/rela-components/src/styles/tokens.css\` +
 * \`dark.css\`); see \`frontend/scripts/gen-app-tokens.mjs\` for what is
 * copied and why.
 *
 * This is the COLOUR contract served to custom-app iframes as \`_rela.css\`,
 * and embedded in the Go binary as \`internal/dataentry/apps_tokens.css\`
 * (pinned byte-for-byte by \`TestAppTokensCSSInSyncWithFrontend\`). Spacing,
 * radius and typography are deliberately not here.
 */
`

/** Keeps `color-scheme` — it selects the built-in light/dark treatment for
 *  form controls and scrollbars, which is part of the palette's job. */
const PALETTE_PROPS = new Set(['color-scheme'])

/** Strips a rule down to its palette declarations, or returns null. */
function paletteRule(rule) {
  const kept = []
  let sawToken = false
  rule.each((decl) => {
    if (decl.type !== 'decl') return
    if (decl.prop.startsWith('--')) {
      sawToken = true
      kept.push(`  ${decl.prop}: ${decl.value};`)
    } else if (PALETTE_PROPS.has(decl.prop.toLowerCase())) {
      kept.push(`  ${decl.prop}: ${decl.value};`)
    }
  })
  if (!sawToken) return null
  return `${rule.selector} {\n${kept.join('\n')}\n}`
}

async function palettesFrom(path) {
  const root = postcss.parse(await readFile(path, 'utf8'))
  const out = []
  root.each((node) => {
    if (node.type === 'rule') {
      const r = paletteRule(node)
      if (r) out.push(r)
    } else if (node.type === 'atrule' && node.name === 'media') {
      const inner = []
      node.each((child) => {
        if (child.type !== 'rule') return
        const r = paletteRule(child)
        if (r) inner.push(r.replace(/^/gm, '  '))
      })
      if (inner.length) out.push(`@media ${node.params} {\n${inner.join('\n\n')}\n}`)
    }
  })
  return out
}

const blocks = [
  ...(await palettesFrom(local('../packages/rela-components/src/styles/tokens.css'))),
  ...(await palettesFrom(local('../packages/rela-components/src/styles/dark.css'))),
]

if (blocks.length === 0) {
  // A silent empty palette is how the editor and every custom app lost their
  // colours once already; fail loudly instead.
  throw new Error('gen-app-tokens: no palette rules found — did the library restructure its tokens?')
}

await writeFile(local('../src/styles/tokens.css'), `${HEADER}\n${blocks.join('\n\n')}\n`, 'utf8')
console.warn(`gen-app-tokens: wrote ${blocks.length} palette blocks`)
