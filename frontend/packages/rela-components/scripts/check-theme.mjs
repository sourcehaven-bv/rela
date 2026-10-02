/*
 * Guards the two things about the dark palette that a person cannot be
 * trusted to keep true by hand.
 *
 *   1. `dark.css` states its palette twice, once under the OS media query and
 *      once under `.dark`, because a selector list cannot contain an `@media`
 *      rule. The two copies must stay identical.
 *   2. Every colour token in `tokens.css` needs a dark counterpart, or it
 *      keeps its light value on a dark background. This is the failure that
 *      is easy to ship: adding a token to `tokens.css` and forgetting this
 *      file leaves one element glowing white.
 *
 * Run with `npm run check:theme`.
 */

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const styles = join(dirname(fileURLToPath(import.meta.url)), '..', 'src', 'styles')
const tokens = readFileSync(join(styles, 'tokens.css'), 'utf8')
const dark = readFileSync(join(styles, 'dark.css'), 'utf8')

const failures = []

/** Every `--rl-…: value` declaration in a chunk of CSS, comments stripped. */
const declarations = (css) => {
  const map = new Map()
  for (const [, name, value] of css.replace(/\/\*[\s\S]*?\*\//g, '').matchAll(/(--rl-[\w-]+)\s*:\s*([^;]+);/g)) {
    map.set(name, value.trim().replace(/\s+/g, ' '))
  }
  return map
}

// 1. The two palette blocks must agree.
const mediaBlock = dark.match(/@media[^{]*\{\s*:root:not\(\.light\)\s*\{([\s\S]*?)\n {2}\}\n\}/)
const classBlock = dark.match(/\n\.dark \{\n([\s\S]*?)\n\}\n\n\/\*\n \* A dark subtree/)

if (!mediaBlock || !classBlock) {
  failures.push(
    'Could not find both palette blocks in dark.css. If you restructured the\n' +
      '  file, update the patterns in scripts/check-theme.mjs to match.',
  )
} else {
  const inMedia = declarations(mediaBlock[1])
  const inClass = declarations(classBlock[1])

  for (const [name, value] of inMedia) {
    if (!inClass.has(name)) failures.push(`${name} is in the media-query block but not in .dark`)
    else if (inClass.get(name) !== value) {
      failures.push(`${name} differs: media-query has "${value}", .dark has "${inClass.get(name)}"`)
    }
  }
  for (const name of inClass.keys()) {
    if (!inMedia.has(name)) failures.push(`${name} is in .dark but not in the media-query block`)
  }
}

// 2. Every colour token needs a dark value.
const isColour = (name) =>
  /^--rl-(color|tag|shadow)-/.test(name) && !/^--rl-focus-ring-(width|gap)$/.test(name)

const darkNames = new Set(declarations(dark).keys())
for (const name of declarations(tokens).keys()) {
  if (isColour(name) && !darkNames.has(name)) {
    failures.push(`${name} is declared in tokens.css but has no dark value in dark.css`)
  }
}

if (failures.length > 0) {
  console.error('Theme check failed:\n')
  for (const f of failures) console.error(`  - ${f}`)
  console.error('')
  process.exit(1)
}

console.log('Theme check passed: both palette blocks agree, every colour token has a dark value.')
