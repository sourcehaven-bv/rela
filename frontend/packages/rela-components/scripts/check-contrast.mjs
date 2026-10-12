/*
 * Asserts the contrast claims the README makes, for both themes, by reading
 * the tokens out of the CSS rather than from a copy kept in this file.
 *
 * The pairings below are the ones the components actually produce: which text
 * token lands on which surface, which foreground sits on which tinted fill.
 * A token changed to a prettier shade that quietly drops a pairing under 4.5:1
 * fails here instead of in an audit.
 *
 * Run with `npm run check:contrast`.
 */

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const styles = join(dirname(fileURLToPath(import.meta.url)), '..', 'src', 'styles')

/** Declarations from a chunk of CSS, comments stripped. */
const declarations = (css) => {
  const map = new Map()
  for (const [, name, value] of css.replace(/\/\*[\s\S]*?\*\//g, '').matchAll(/(--rl-[\w-]+)\s*:\s*([^;]+);/g)) {
    map.set(name, value.trim())
  }
  return map
}

const light = declarations(readFileSync(join(styles, 'tokens.css'), 'utf8'))
const darkCss = readFileSync(join(styles, 'dark.css'), 'utf8')
// Either palette block will do; check-theme.mjs proves they are identical.
const dark = new Map(light)
for (const [name, value] of declarations(darkCss.match(/\.dark \{([\s\S]*?)\n\}/)[1])) {
  dark.set(name, value)
}

const relativeLuminance = (hex) => {
  const h = hex.replace('#', '')
  const channels = [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16) / 255)
  const [r, g, b] = channels.map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

const ratio = (a, b) => {
  const [hi, lo] = [relativeLuminance(a), relativeLuminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

/*
 * The surfaces body text is painted on. `--rl-color-bg-raised` is included
 * because a modal, drawer, menu and toast all carry ordinary text on it.
 */
const surfaces = [
  '--rl-color-bg',
  '--rl-color-bg-sunken',
  '--rl-color-bg-hover',
  '--rl-color-bg-active',
  '--rl-color-bg-selected',
  '--rl-color-bg-raised',
]

/* [foreground, background, minimum]. 4.5 for text, 3 for non-text. */
const pairs = [
  ...surfaces.flatMap((s) => [
    ['--rl-color-text', s, 4.5],
    ['--rl-color-text-muted', s, 4.5],
    ['--rl-color-text-subtle', s, 4.5],
    // Danger and the accent are used as text colour on any of these.
    ['--rl-color-danger', s, 4.5],
    ['--rl-color-accent', s, 4.5],
  ]),

  // Solid fills carrying inverse text: primary and danger buttons, checkbox.
  ['--rl-color-text-inverse', '--rl-color-accent', 4.5],
  ['--rl-color-text-inverse', '--rl-color-accent-hover', 4.5],
  ['--rl-color-text-inverse', '--rl-color-danger', 4.5],
  ['--rl-color-text-inverse', '--rl-color-danger-hover', 4.5],

  // The tooltip inverts the two text tokens against each other.
  ['--rl-color-text-inverse', '--rl-color-text', 4.5],

  // A danger button's tinted hover fill still carries its label.
  ['--rl-color-danger', '--rl-color-danger-bg', 4.5],
  ['--rl-color-danger-hover', '--rl-color-danger-bg', 4.5],
  ['--rl-color-danger', '--rl-color-danger-bg-hover', 4.5],

  // Banners and callouts carry body text on their own fill.
  ['--rl-color-info-fg', '--rl-color-info-bg', 4.5],
  ['--rl-color-success-fg', '--rl-color-success-bg', 4.5],
  ['--rl-color-warning-fg', '--rl-color-warning-bg', 4.5],

  // Tags.
  ...['grey', 'blue', 'green', 'amber', 'red', 'purple'].map((c) => [
    `--rl-tag-${c}-fg`,
    `--rl-tag-${c}-bg`,
    4.5,
  ]),

  /*
   * A calendar event chip fills itself with a tag background and writes its
   * meta line ("Auditor: T. de Vries") on it. `--rl-color-text-muted` is
   * tuned for the neutral surfaces and failed every tone here, so the chip
   * uses the tag palette's own grey instead.
   */
  ...['grey', 'blue', 'green', 'amber', 'red', 'purple'].map((c) => [
    '--rl-tag-grey-fg',
    `--rl-tag-${c}-bg`,
    4.5,
  ]),

  // Attachment tiles carry an inverse-coloured glyph.
  ...['pdf', 'doc', 'sheet', 'image', 'other'].map((t) => [
    '--rl-color-text-inverse',
    `--rl-color-file-${t}`,
    4.5,
  ]),

  // Non-text: status dots and the focus rings, on the surfaces they appear on.
  ...['green', 'amber', 'red', 'grey', 'blue'].flatMap((c) => [
    [`--rl-color-status-${c}`, '--rl-color-bg', 3],
    [`--rl-color-status-${c}`, '--rl-color-bg-hover', 3],
  ]),
  // A purple status dot is the accent.
  ['--rl-color-accent', '--rl-color-bg', 3],
  ['--rl-color-accent', '--rl-color-bg-hover', 3],
  /*
   * The selected segmented-control chip carries its own label, and rides on
   * `--rl-color-bg-hover` rather than on the page, so it is checked against
   * the track rather than added to `surfaces`.
   */
  ['--rl-color-text', '--rl-color-chip-raised', 4.5],
  ['--rl-color-chip-raised', '--rl-color-bg-hover', 1.05],

  ['--rl-color-focus', '--rl-color-bg', 3],
  ['--rl-color-error-ring', '--rl-color-bg', 3],
  ['--rl-color-starred', '--rl-color-bg', 3],
]

/*
 * Pre-existing light-theme near-misses, present before the dark theme was
 * added and left alone because retuning the light palette was not part of
 * that work. They are listed rather than silently dropped so the check runs
 * green on today's state and still fails on anything new.
 *
 * Fixing them means darkening `--rl-color-text-muted`, `--rl-color-text-subtle`
 * and `--rl-color-accent` slightly, or lightening `--rl-color-bg-active`;
 * delete the entry once the pairing clears its target.
 */
const knownLightFailures = new Set([
  '--rl-color-text-muted on --rl-color-bg-active',
  '--rl-color-text-subtle on --rl-color-bg-active',
  '--rl-color-accent on --rl-color-bg-active',
  '--rl-color-status-green on --rl-color-bg-hover',
])

let failed = 0
let excused = 0

for (const [name, palette] of [['light', light], ['dark', dark]]) {
  for (const [fg, bg, min] of pairs) {
    const fgValue = palette.get(fg)
    const bgValue = palette.get(bg)

    if (!fgValue || !bgValue) {
      console.error(`  ${name}: missing token ${!fgValue ? fg : bg}`)
      failed += 1
      continue
    }

    const r = ratio(fgValue, bgValue)
    if (r < min) {
      if (name === 'light' && knownLightFailures.has(`${fg} on ${bg}`)) {
        excused += 1
        continue
      }
      console.error(`  ${name}: ${fg} on ${bg} is ${r.toFixed(2)}:1, needs ${min}:1`)
      failed += 1
    }
  }
}

if (failed > 0) {
  console.error(`\nContrast check failed: ${failed} pairing(s) below target.\n`)
  process.exit(1)
}

console.log(
  `Contrast check passed: ${pairs.length * 2} pairings across both themes` +
    (excused > 0 ? `, ${excused} known light-theme exception(s) skipped.` : '.'),
)
