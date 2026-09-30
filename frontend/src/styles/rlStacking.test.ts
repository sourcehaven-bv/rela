import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

/**
 * The library's overlay z-scale is rebased onto rela's in `styles/rl.css`.
 *
 * Why this is a grep test over source rather than a mounted assertion: Vitest
 * does not apply an SFC's scoped `<style>`, so `getComputedStyle` on a mounted
 * RlModal resolves against no CSS and would pass whatever the stylesheet says.
 * `focusRing.test.ts` documents the same reasoning for the same reason.
 *
 * What can actually break this:
 *
 *  - Both files declare the tokens on `:root`, at equal specificity, so the
 *    WINNER IS SOURCE ORDER. Moving rela's import above the library's in
 *    `main.ts` would silently restore the library's values.
 *  - A new overlay token added upstream (the library gained `--rl-z-sticky`
 *    and `--rl-z-sticky-raised` after this integration began) would come in
 *    unrebased.
 */

const STYLES = resolve(__dirname)
const rlCss = readFileSync(resolve(STYLES, 'rl.css'), 'utf8')
const mainTs = readFileSync(resolve(STYLES, '..', 'main.ts'), 'utf8')

/** Reads the LAST declaration of a token, which is the one that wins. */
function declaredValue(css: string, token: string): number | null {
  const matches = [...css.matchAll(new RegExp(`${token}:\\s*(\\d+)`, 'g'))]
  if (matches.length === 0) return null
  return Number(matches[matches.length - 1][1])
}

describe('the library overlay stack is rebased onto rela’s', () => {
  /**
   * rela's own tiers: 900 for a dialog that must stay below a confirm raised
   * from within it, 1000 for the standard overlay tier, 10000 for editor
   * fullscreen. An RlModal left at the library's 100 renders underneath all
   * of them — including StatusBar's overlay and every unmigrated modal.
   */
  it('puts RlModal on rela’s standard overlay tier, not the library’s 100', () => {
    expect(declaredValue(rlCss, '--rl-z-modal')).toBe(1000)
  })

  it('keeps a drawer below the 900 tier so a rela dialog still wins', () => {
    const drawer = declaredValue(rlCss, '--rl-z-drawer')
    expect(drawer).not.toBeNull()
    expect(drawer!).toBeLessThan(900)
  })

  /**
   * The rebase must not reorder the library's own stack: a menu or tooltip
   * opened from inside a dialog, or from inside a shell panel such as the
   * sidebar flyout, has to clear it, and menus and tooltips teleport to body
   * so neither can fall back on DOM order.
   */
  it('preserves the library’s internal order', () => {
    const order = [
      '--rl-z-shell-panel',
      '--rl-z-shell-panel-full',
      '--rl-z-shell-flyout',
      '--rl-z-drawer',
      '--rl-z-modal',
      '--rl-z-menu',
      '--rl-z-tooltip',
      '--rl-z-toast',
    ].map((token) => {
      const value = declaredValue(rlCss, token)
      expect(value, `${token} is not rebased in rl.css`).not.toBeNull()
      return value!
    })

    const ascending = [...order].sort((a, b) => a - b)
    expect(order).toEqual(ascending)
  })

  /** Editor fullscreen (Milkdown/EasyMDE at 9999) stays on top of everything. */
  it('leaves every overlay below editor fullscreen', () => {
    expect(declaredValue(rlCss, '--rl-z-toast')!).toBeLessThan(9999)
  })

  /**
   * Equal specificity means the later import wins, so the rebase only holds
   * while rl.css is imported AFTER the library's own tokens (which it pulls in
   * itself, at the top of the file).
   */
  it('is imported from main.ts, where its overrides land last', () => {
    expect(mainTs).toContain("import './styles/rl.css'")
  })

  /**
   * A token the library stacks an overlay with, but which rl.css does not
   * rebase, would come in at the library's range. The sticky tokens are
   * excluded on purpose: they order table headers inside a scroll container,
   * not overlays against the page.
   */
  it('rebases every overlay token the library declares', () => {
    const libraryTokens = readFileSync(
      resolve(STYLES, '..', '..', 'packages', 'rela-components', 'src', 'styles', 'tokens.css'),
      'utf8'
    )
    const declared = [...libraryTokens.matchAll(/--rl-z-([a-z-]+):/g)].map((m) => m[1])
    const scrollOrdering = ['sticky', 'sticky-raised']

    const missing = declared
      .filter((name) => !scrollOrdering.includes(name))
      .filter((name) => declaredValue(rlCss, `--rl-z-${name}`) === null)

    expect(
      missing,
      `overlay tokens not rebased in rl.css: ${missing.join(', ')}`
    ).toEqual([])
  })
})
