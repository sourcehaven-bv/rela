/*
 * Guards the two scales whose drift is invisible: motion and stacking.
 *
 * Both fail the same quiet way. A transition written `120ms` looks right,
 * behaves right, and matches its neighbours today — it is wrong only later,
 * when an app retunes `--rl-duration-fast` and one component keeps the old
 * speed. Nothing renders incorrectly, so no test and no screenshot catches it;
 * the component is simply half a scale behind, and the difference is 30ms.
 *
 * A raw `z-index: 30` is worse, because it reads as deliberate. The tokens
 * carry the ORDER as their contract and the numbers are incidental, so an app
 * rebasing the ladder onto its own scale moves every token and leaves the
 * literal where it was. The layer that was level with the flyout is now behind
 * it, drawn but unclickable — which is exactly the failure the token block in
 * `tokens.css` warns about, from the other direction.
 *
 * So the rule is: a transition's duration and easing come from the tokens, and
 * a stacking value either comes from the tokens or is a local one.
 *
 * Two deliberate exemptions, both narrow:
 *
 * - `animation` shorthands are not checked. A looping indicator's period is
 *   tuned against what it indicates, and `tokens.css` states why those stay as
 *   literals: putting a spinner and a drawer on one scale makes changing either
 *   break the other.
 * - `z-index: 0/1/2` inside a component is a local stacking decision — a card
 *   above its own column's backdrop — and says nothing about where the
 *   component sits among the overlay layers. Anything higher is claiming a
 *   place in that ladder and has to name it.
 *
 * Run with `npm run check:motion`.
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

/*
 * The highest z-index a component may write without naming a token. Above this
 * it is competing with the overlay ladder rather than ordering its own parts.
 */
const LOCAL_STACKING_MAX = 2

/*
 * Where the tokens themselves are declared, and so the one file allowed to
 * write the raw values they hold.
 */
const TOKEN_FILES = new Set(['src/styles/tokens.css', 'src/styles/dark.css'])

const failures = []

for (const path of files(src)) {
  const rel = relative(root, path).replace(/\\/g, '/')
  if (TOKEN_FILES.has(rel)) continue

  const source = readFileSync(path, 'utf8')
  const lines = source.split('\n')

  lines.forEach((line, index) => {
    const at = `${rel}:${index + 1}`

    /*
     * Transition declarations. Matched on the property rather than anywhere a
     * duration appears, so an `animation` on the next line is untouched.
     */
    const transition = line.match(/(?:^|[\s;{])transition(?:-duration)?\s*:\s*([^;}]*)/)
    if (transition) {
      const value = transition[1]

      const literalDuration = value.match(/(?<![-\w(])\d+(?:\.\d+)?m?s\b/)
      if (literalDuration) {
        failures.push(
          `${at}: transition duration \`${literalDuration[0]}\` is a literal. ` +
            `Use --rl-duration-fast (a state change in place), ` +
            `--rl-duration-base (something small moving a short way) or ` +
            `--rl-duration-slow (a large surface crossing the screen).`,
        )
      }

      /*
       * Bare easing keywords. `ease-out`, `linear` and a `cubic-bezier` are
       * left alone: a component choosing a different curve on purpose is a
       * legitimate local decision, where a bare `ease` is just the default
       * spelled out and should read from the token.
       */
      if (/(?<![-\w])ease(?![-\w(])/.test(value)) {
        failures.push(
          `${at}: transition easing \`ease\` is a literal. Use var(--rl-ease).`,
        )
      }
    }

    /* Stacking. */
    const stacking = line.match(/(?:^|[\s;{])z-index\s*:\s*([^;}]+)/)
    if (stacking) {
      const value = stacking[1].trim()
      if (/^-?\d+$/.test(value)) {
        const depth = Number(value)
        if (Math.abs(depth) > LOCAL_STACKING_MAX) {
          failures.push(
            `${at}: z-index \`${value}\` is a literal above the local range ` +
              `(±${LOCAL_STACKING_MAX}). A value this high is claiming a place in the ` +
              `overlay ladder, so it has to name one of the --rl-z-* tokens: ` +
              `an app that rebases the ladder moves the tokens and leaves this behind.`,
          )
        }
      }
    }
  })
}

if (failures.length) {
  console.error('Motion and stacking check failed:\n')
  for (const failure of failures) console.error(`  - ${failure}`)
  console.error(
    '\nBoth of these drift silently: the component still renders correctly and\n' +
      'only falls out of step once an app retunes the scale. Read the duration,\n' +
      'easing and stacking values from the tokens in src/styles/tokens.css.',
  )
  process.exit(1)
}

console.log(
  'Motion and stacking check passed: transitions read their duration and easing ' +
    'from the tokens, and no component writes a stacking value that competes ' +
    'with the overlay ladder.',
)
