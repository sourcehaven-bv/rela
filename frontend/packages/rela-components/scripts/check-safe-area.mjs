/*
 * Guards the safe-area insets against the one failure that testing does not
 * catch: applying the same inset twice.
 *
 * A doubled inset looks like a correct inset. It clears the notch, so nothing
 * overlaps and nothing is hidden; the content simply sits further from the
 * edge than it should. Every lower-bound assertion an app is likely to write
 * (`padding-top >= 56px`) passes, and a device profile that reports no inset
 * at all — which is most of them — reduces both the right and the wrong
 * version to zero. It reaches production looking fine on every machine that
 * does not have a notch.
 *
 * So the rule is structural rather than measured: within one component, an
 * edge belongs to exactly one element. Two nested elements may not both pad
 * the same edge with the same inset.
 *
 * Where a container genuinely must inset a child that also insets itself —
 * the app shell's sidebar drawer, which protects whatever a consumer puts in
 * that slot — the container zeroes the child's `--rl-sidebar-safe-*` rather
 * than the two agreeing informally. That indirection is what this check
 * treats as the sanctioned escape, and it verifies the zeroing is really
 * there rather than assumed.
 *
 * Run with `npm run check:safe-area`.
 */

import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join, relative } from 'node:path'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const src = join(root, 'src')

/** Every .vue and .css file under src/, recursively. */
function* files(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) yield* files(path)
    else if (/\.(vue|css)$/.test(entry.name)) yield path
  }
}

const EDGES = ['top', 'right', 'bottom', 'left']

/** The raw inset tokens, which are the ones that must not stack. */
const rawInset = (edge) => `--rl-safe-inset-${edge}`

/**
 * Declarations that pad or offset a given edge, as `{ selector, property }`.
 * Comments are stripped first, so a token named in prose is never a match.
 */
function insetUses(css, edge) {
  const cleaned = css.replace(/\/\*[\s\S]*?\*\//g, '')
  const uses = []

  /*
   * Selector-with-body pairs. Nested at-rules (`@media`) are flattened by
   * matching only blocks whose body has no `{`, which leaves the declaration
   * blocks themselves regardless of what they sit inside.
   */
  for (const [, selector, body] of cleaned.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    if (!body.includes(rawInset(edge))) continue

    for (const [, property, value] of body.matchAll(/([\w-]+)\s*:\s*([^;]+)/g)) {
      if (!value.includes(rawInset(edge))) continue
      // Only the properties that actually move content away from the edge.
      if (!/^(padding|margin|inset|top|right|bottom|left)/.test(property)) continue
      // A custom property re-exporting the inset is indirection, not a use.
      if (property.startsWith('--')) continue
      uses.push({ selector: selector.trim().replace(/\s+/g, ' '), property })
    }
  }

  return uses
}

/**
 * Whether one selector is plausibly an ancestor of another within the same
 * file. Deliberately shallow: these are BEM-ish class names in scoped styles,
 * so a shared block prefix is what nesting looks like here.
 *
 * `.rl-sidebar` is an ancestor of `.rl-sidebar__header`; two separate
 * `__element` selectors are siblings and may each own their own edge.
 */
function mayNest(outer, inner) {
  const block = (selector) => selector.match(/\.([\w-]+?)(?:__|--|[\s:,]|$)/)?.[1]
  const outerBlock = block(outer)
  const innerBlock = block(inner)
  if (!outerBlock || !innerBlock || outerBlock !== innerBlock) return false

  // The bare block is the root element; anything else in it sits inside.
  const isRoot = (selector) => !/__/.test(selector)
  return isRoot(outer) && !isRoot(inner)
}

const failures = []

for (const path of files(src)) {
  const css = readFileSync(path, 'utf8')
  if (!css.includes('--rl-safe-inset-')) continue
  const where = relative(root, path)

  for (const edge of EDGES) {
    const uses = insetUses(css, edge)
    if (uses.length < 2) continue

    for (const outer of uses) {
      for (const inner of uses) {
        if (outer === inner) continue
        if (!mayNest(outer.selector, inner.selector)) continue
        failures.push(
          `${where}: the ${edge} inset is applied by both ` +
            `\`${outer.selector}\` (${outer.property}) and ` +
            `\`${inner.selector}\` (${inner.property}), which nest. ` +
            `One element owns an edge.`,
        )
      }
    }
  }
}

/*
 * The sanctioned cross-component case: the app shell insets its sidebar
 * drawer so a consumer's own slot markup is protected, and `RlSidebar` insets
 * itself for every other context. Those two must not both apply, so the shell
 * is required to zero the child's tokens wherever it pads the drawer.
 */
const shell = readFileSync(join(src, 'components', 'layout', 'RlAppShell.vue'), 'utf8')
const shellDrawerBlocks = shell
  .replace(/\/\*[\s\S]*?\*\//g, '')
  .match(/\.rl-app-shell[^{}]*__sidebar[^{}]*\{[^{}]*\}/g) ?? []

for (const block of shellDrawerBlocks) {
  const pads = /(?:^|[\s;])(?:padding|left|top|bottom)[\w-]*\s*:[^;]*--rl-safe-inset-/.test(block)
  if (!pads) continue

  for (const edge of ['top', 'bottom', 'left']) {
    if (!block.includes(`--rl-sidebar-safe-${edge}: 0`)) {
      failures.push(
        `RlAppShell.vue: a sidebar rule insets the drawer but does not zero ` +
          `\`--rl-sidebar-safe-${edge}\`, so RlSidebar would add the ${edge} ` +
          `inset a second time.`,
      )
    }
  }
}

if (failures.length) {
  console.error('Safe-area check failed:\n')
  for (const failure of failures) console.error(`  - ${failure}`)
  console.error(
    '\nA doubled inset still clears the notch, so it passes any lower-bound\n' +
      'assertion and shows as zero on a device without one. Give each edge a\n' +
      'single owner, or zero the child through the `--rl-sidebar-safe-*` tokens.',
  )
  process.exit(1)
}

console.log(
  `Safe-area check passed: no edge is inset twice, and the shell's drawer ` +
    `zeroes the sidebar's own insets.`,
)
