import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

import { contrastRatio } from './contrast'

/**
 * Pins the colour tokens to WCAG AA in both themes. The palette follows the
 * Atlas UI kit, whose mid shades often fail AA (the kit's blue/500 button
 * carries white text at 2.9:1), so a kit-driven retune must pass here first.
 *
 * Reads the generated `styles/tokens.css`: `:root` for light and `.dark` for
 * dark, which is what both the SPA and custom apps receive.
 */
const css = readFileSync(resolve(__dirname, '../styles/tokens.css'), 'utf-8')

function block(selector: string): Record<string, string> {
  const start = css.indexOf(`\n${selector} {`)
  if (start < 0) throw new Error(`tokens.css has no ${selector} block`)
  const body = css.slice(start, css.indexOf('}', start))
  const tokens: Record<string, string> = {}
  for (const m of body.matchAll(/(--rl-[a-z0-9-]+):\s*(#[0-9a-f]{6});/gi)) tokens[m[1]] = m[2]
  return tokens
}

const light = block(':root')
const dark = { ...light, ...block('.dark') }

const KIT_HUES = ['grey', 'red', 'orange', 'yellow', 'green', 'teal', 'blue'] as const
const KIT_SHADES = ['50', '100', '200', '300', '400', '500', '600', '700', '800', '900'] as const

const TEXT = 4.5
const NON_TEXT = 3

// [foreground, background, minimum]
const pairs: [string, string, number][] = [
  ['--rl-color-text', '--rl-color-bg', TEXT],
  ['--rl-color-text', '--rl-color-bg-hover', TEXT],
  ['--rl-color-text', '--rl-color-bg-selected', TEXT],
  ['--rl-color-text-muted', '--rl-color-bg-hover', TEXT],
  ['--rl-color-text-subtle', '--rl-color-bg-hover', TEXT],
  ['--rl-color-text-inverse', '--rl-color-accent', TEXT],
  ['--rl-color-text-inverse', '--rl-color-accent-hover', TEXT],
  ['--rl-color-accent', '--rl-color-bg', TEXT],
  ['--rl-color-text-inverse', '--rl-color-danger', TEXT],
  ['--rl-color-text-inverse', '--rl-color-danger-hover', TEXT],
  ['--rl-color-danger', '--rl-color-bg', TEXT],
  ['--rl-color-danger', '--rl-color-danger-bg', TEXT],
  ['--rl-color-danger', '--rl-color-danger-bg-hover', TEXT],
  ['--rl-color-info-fg', '--rl-color-info-bg', TEXT],
  ['--rl-color-success-fg', '--rl-color-success-bg', TEXT],
  ['--rl-color-warning-fg', '--rl-color-warning-bg', TEXT],
  ...(['grey', 'blue', 'green', 'amber', 'red', 'purple', 'yellow', 'teal'] as const).map(
    (c): [string, string, number] => [`--rl-tag-${c}-fg`, `--rl-tag-${c}-bg`, TEXT],
  ),
  ...(['green', 'amber', 'red', 'grey', 'blue', 'yellow', 'teal'] as const).map(
    (c): [string, string, number] => [`--rl-color-status-${c}`, '--rl-color-bg', NON_TEXT],
  ),
  // Every kit shade carries its own text colour at AA, in both themes: the
  // style colour `red/700` is painted as a fill with that text on it.
  ...KIT_HUES.flatMap((hue) =>
    KIT_SHADES.map((shade): [string, string, number] => [
      `--rl-kit-${hue}-${shade}-fg`,
      `--rl-kit-${hue}-${shade}`,
      TEXT,
    ]),
  ),
  ['--rl-color-focus', '--rl-color-bg', NON_TEXT],
]

describe.each([
  ['light', light],
  ['dark', dark],
])('%s theme tokens meet WCAG AA', (_, theme) => {
  it.each(pairs)('%s on %s', (fg, bg, min) => {
    expect(theme[fg], fg).toBeDefined()
    expect(theme[bg], bg).toBeDefined()
    expect(contrastRatio(theme[fg], theme[bg])).toBeGreaterThanOrEqual(min)
  })
})
