import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'

/**
 * The library's overlay z-scale is rebased onto rela's in `styles/stacking.css`.
 *
 * Why this is a test over source rather than a mounted assertion: Vitest does
 * not apply an SFC's scoped `<style>`, so `getComputedStyle` on a mounted
 * RlModal resolves against no CSS and would pass whatever the stylesheet says.
 * `focusRing.test.ts` documents the same reasoning for the same reason.
 *
 * What can actually break this: every file involved declares the tokens on
 * `:root`, unlayered (`relaCssLayer.ts` keeps custom-property-only rules out
 * of the layer), so the WINNER IS SOURCE ORDER. `tokens.css` is generated
 * from the library and repeats the library's z-values. The rebase used to sit
 * in `rl.css`, which `main.ts` imports before `tokens.css`, so the library's
 * values won: a confirm opened at 100, behind the 900 dialog that raised it,
 * and the session screen at 301 (BUG-ZL4K7QW). An earlier version of this
 * test read `rl.css` alone and passed throughout.
 *
 * So the tests below resolve each token the way the bundle does: walk the
 * stylesheets in `main.ts` import order, expand their `@import`s, and take the
 * last declaration. A route chunk loads after these, so one that redeclared
 * a `--rl-z-*` token would win in the build and pass here; none does.
 */

const STYLES = resolve(__dirname)
const SRC = resolve(STYLES, '..')
const LIBRARY_STYLES = resolve(SRC, '..', 'packages', 'rela-components', 'src', 'styles')

function read(path: string): string {
  return readFileSync(path, 'utf8')
}

function stripComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, '')
}

/** Resolves an import specifier the way Vite does for the files involved. */
function resolveSpecifier(spec: string, fromDir: string): string | null {
  if (spec.startsWith('rela-components/styles/')) {
    return resolve(LIBRARY_STYLES, spec.slice('rela-components/styles/'.length))
  }
  if (spec.startsWith('.')) return resolve(fromDir, spec)
  // Fonts and other packages declare no z tokens.
  return null
}

/** A stylesheet with its `@import`s inlined, in order. */
function expand(path: string): string {
  const css = stripComments(read(path))
  return css.replace(/@import\s+['"]([^'"]+)['"]\s*;/g, (_m, spec: string) => {
    const target = resolveSpecifier(spec, dirname(path))
    return target ? expand(target) : ''
  })
}

/** Every stylesheet `main.ts` imports, expanded and concatenated in order. */
function bundledCss(): string {
  const mainTs = read(resolve(SRC, 'main.ts'))
  return [...mainTs.matchAll(/^import\s+['"]([^'"]+\.css)['"]/gm)]
    .map((m) => resolveSpecifier(m[1], SRC))
    .filter((p): p is string => p !== null)
    .map(expand)
    .join('\n')
}

/** The LAST declaration of a token, which is the one that wins. */
function lastValue(css: string, token: string): number | null {
  const matches = [...css.matchAll(new RegExp(`${token}:\\s*(\\d+)`, 'g'))]
  if (matches.length === 0) return null
  return Number(matches[matches.length - 1][1])
}

const bundle = bundledCss()
const stackingCss = stripComments(read(resolve(STYLES, 'stacking.css')))

function effective(token: string): number {
  const value = lastValue(bundle, token)
  expect(value, `${token} is not declared in any stylesheet main.ts imports`).not.toBeNull()
  return value!
}

/** The `:layer` a dialog passes to RlModal to sit below confirms. */
function dialogLayer(file: string): number {
  const source = read(resolve(SRC, file))
  const match = source.match(/:layer="(\d+)"/)
  expect(match, `${file} no longer passes a :layer`).not.toBeNull()
  return Number(match![1])
}

/** The session screen's z-index: a token plus an offset. */
function sessionScreenZ(): number {
  const source = read(resolve(SRC, 'components', 'common', 'WorkspaceAccessScreen.vue'))
  const match = source.match(/z-index:\s*calc\(var\((--rl-z-[a-z-]+)\)\s*\+\s*(\d+)\)/)
  expect(match, 'WorkspaceAccessScreen no longer stacks on a z token').not.toBeNull()
  return effective(match![1]) + Number(match![2])
}

describe('the overlay stack as the bundle resolves it', () => {
  /**
   * rela's own tiers: 900 for a dialog that must stay below a confirm raised
   * from within it, 1000 for the standard overlay tier, 10000 for editor
   * fullscreen. An RlModal left at the library's 100 renders underneath all
   * of them, including StatusBar's overlay and every unmigrated modal.
   */
  it('puts RlModal on rela’s standard overlay tier, not the library’s 100', () => {
    expect(effective('--rl-z-modal')).toBe(1000)
  })

  /**
   * The order the browser test found broken: the session screen over the
   * confirm, the confirm over the dialog that raised it ("Discard new
   * note?" over New Note).
   */
  it('stacks session screen > confirm > dialog', () => {
    const confirm = effective('--rl-z-modal')
    for (const dialog of [
      'components/forms/InlineCreateFormModal.vue',
      'components/entity/DuplicateModal.vue',
    ]) {
      expect(dialogLayer(dialog), dialog).toBeLessThan(confirm)
    }
    expect(sessionScreenZ()).toBeGreaterThan(confirm)
  })

  it('keeps a drawer below the 900 tier so a rela dialog still wins', () => {
    expect(effective('--rl-z-drawer')).toBeLessThan(900)
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
    ].map(effective)

    const ascending = [...order].sort((a, b) => a - b)
    expect(order).toEqual(ascending)
  })

  /** Editor fullscreen (Milkdown/EasyMDE at 9999) stays on top of everything. */
  it('leaves every overlay below editor fullscreen', () => {
    expect(effective('--rl-z-toast')).toBeLessThan(9999)
    expect(sessionScreenZ()).toBeLessThan(9999)
  })

  /**
   * A token the library stacks an overlay with, but which stacking.css does
   * not rebase, would come in at the library's range. The sticky tokens are
   * excluded on purpose: they order table headers inside a scroll container,
   * not overlays against the page.
   */
  it('rebases every overlay token the library declares', () => {
    const libraryTokens = read(resolve(LIBRARY_STYLES, 'tokens.css'))
    const declared = [...libraryTokens.matchAll(/--rl-z-([a-z-]+):/g)].map((m) => m[1])
    const scrollOrdering = ['sticky', 'sticky-raised']

    const unrebased = declared
      .filter((name) => !scrollOrdering.includes(name))
      .filter((name) => effective(`--rl-z-${name}`) !== lastValue(stackingCss, `--rl-z-${name}`))

    expect(
      unrebased,
      `overlay tokens not winning from stacking.css: ${unrebased.join(', ')}`
    ).toEqual([])
  })
})
