import { describe, it, expect } from 'vitest'
import postcss from 'postcss'
import { wrapCss, RELA_LAYER } from './relaCssLayer'

/**
 * Pins the cascade-layer wrap that makes operator `custom.css` win.
 *
 * The invariant: everything rela emits is inside `@layer rela` EXCEPT the
 * top-level `:root` token rules (served to custom-app iframes as `_rela.css`,
 * where there is no other rela CSS to layer against) and the leading
 * `@charset`/`@import` statements (illegal inside a layer). See TKT-3DBK6I.
 */

/** Offset of the opening `@layer rela {` block in the output. */
function layerStart(css: string): number {
  const m = /@layer\s+rela\s*\{/.exec(css)
  return m ? m.index : -1
}

/** Everything inside the layer block. */
function insideLayer(css: string): string {
  const at = layerStart(css)
  return at < 0 ? '' : css.slice(at)
}

/** Everything before the layer block. */
function beforeLayer(css: string): string {
  const at = layerStart(css)
  return at < 0 ? css : css.slice(0, at)
}

describe('wrapCss', () => {
  it('wraps ordinary rules in the rela layer', () => {
    const out = wrapCss('.sidebar[data-v-abc]{color:red}') ?? ''
    expect(insideLayer(out)).toContain('.sidebar[data-v-abc]')
  })

  it('declares layer order up-front so it does not depend on load order', () => {
    // 18 route chunks are appended to <head> at runtime; whichever arrives
    // first would otherwise establish the layer's position.
    expect(wrapCss('.a{color:red}')).toMatch(/^@layer rela;/)
  })

  it('returns null for already-layered input (idempotent)', () => {
    expect(wrapCss(`@layer ${RELA_LAYER} {.a{color:red}}`)).toBeNull()
  })

  it('wraps empty input so the invariant holds for every emitted stylesheet', () => {
    expect(wrapCss('')).toContain(`@layer ${RELA_LAYER}`)
  })

  describe('token carve-out', () => {
    it('keeps top-level :root outside the layer', () => {
      const out = wrapCss(':root{--accent-color:#4772fb}.a{color:red}') ?? ''
      expect(beforeLayer(out)).toContain('--accent-color')
      expect(insideLayer(out)).toContain('.a')
    })

    it('keeps :root.dark outside the layer', () => {
      const out = wrapCss(':root.dark{--bg-color:#164155}.a{color:red}') ?? ''
      expect(beforeLayer(out)).toContain('--bg-color')
    })

    it('carves out CONSECUTIVE :root blocks, not just the first', () => {
      // Regression: the real bundle emits `:root{...}:root.dark{...}` back to
      // back. A boundary-consuming regex carved out the first and left
      // `:root.dark` INSIDE the layer, silently demoting every dark-mode token.
      const out = wrapCss(':root{--a:1}:root.dark{--a:2}.x{color:red}') ?? ''
      expect(beforeLayer(out)).toContain('--a:1')
      expect(beforeLayer(out)).toContain('--a:2')
      expect(insideLayer(out)).not.toContain(':root')
    })

    it('does NOT carve out rules that merely reference a token', () => {
      // Regression: an early heuristic matched any file containing
      // `--sidebar-bg`, wrongly excluding 9KB of SettingsView component CSS.
      const out = wrapCss('.panel{background:var(--sidebar-bg)}') ?? ''
      expect(insideLayer(out)).toContain('.panel')
    })

    it('does NOT carve out a descendant selector like :root .fa-rotate-90', () => {
      // Font Awesome ships this. It is vendor component CSS, not a token
      // declaration, and belongs inside the layer.
      const out = wrapCss(':root .fa-rotate-90{filter:none}') ?? ''
      expect(insideLayer(out)).toContain('.fa-rotate-90')
      expect(beforeLayer(out)).not.toContain('.fa-rotate-90')
    })

    it('carves out a bare theme class like .dark', () => {
      // rela-components declares its dark palette on `.dark` (which themes a
      // SUBTREE, so it cannot be written as `:root.dark`). Layering it while
      // the light `:root` palette stayed unlayered meant light won at any
      // specificity: the app rendered light with `.dark` on the html element.
      const out = wrapCss('.dark{--rl-color-bg:#0d0d0f}.b{c:d}') ?? ''
      expect(beforeLayer(out)).toContain('--rl-color-bg')
      expect(insideLayer(out)).toContain('.b')
    })

    it('carves out a palette that also sets color-scheme', () => {
      // Both tokens.css and dark.css set `color-scheme` beside their tokens —
      // it picks the built-in treatment for form controls and scrollbars, so
      // it is part of the same decision. A tokens-only test would have let the
      // real palette stay layered.
      const out = wrapCss(':root{color-scheme:light;--rl-color-bg:#fff}.b{c:d}') ?? ''
      expect(beforeLayer(out)).toContain('--rl-color-bg')
      expect(insideLayer(out)).toContain('.b')
    })

    it('does NOT carve out a rule of nothing but color-scheme', () => {
      // No tokens means no contract to protect.
      const out = wrapCss(':root{color-scheme:light}.b{c:d}') ?? ''
      expect(insideLayer(out)).toContain('color-scheme')
    })

    it('carves out a palette inside @media (prefers-color-scheme)', () => {
      // How a palette reaches the OS preference. Carved out WITH its at-rule,
      // or the OS-dark tokens lose to the unlayered light ones.
      const src = '@media(prefers-color-scheme:dark){:root:not(.light){--a:2}}.b{c:d}'
      const out = wrapCss(src) ?? ''
      expect(beforeLayer(out)).toContain('--a:2')
      expect(insideLayer(out)).not.toContain('--a:2')
    })

    it('does NOT carve out an @media block that sets ordinary properties', () => {
      // A responsive component override, not a palette. The distinction is the
      // body: only custom-property declarations qualify.
      const out = wrapCss('@media(min-width:0){.panel{color:red}}.b{c:d}') ?? ''
      expect(insideLayer(out)).toContain('@media')
      expect(insideLayer(out)).toContain('.panel')
    })

    it('does NOT carve out a .dark rule that paints, rather than declaring tokens', () => {
      // dark.css has both: the palette, and a `.dark{background:…}` rule that
      // paints a dark subtree. Only the palette is a token contract.
      const out = wrapCss('.dark{background:var(--rl-color-bg)}.b{c:d}') ?? ''
      expect(insideLayer(out)).toContain('background')
      expect(beforeLayer(out)).not.toContain('background')
    })

    it('SPLITS a rule the minifier merged from a palette and a paint rule', () => {
      // The real regression. dark.css declares `.dark` twice — palette, then
      // the subtree paint rule — and lightningcss merges them into one rule
      // declaring both. Rejecting that as "not a pure palette" layered the
      // entire dark palette, so the unlayered light `:root` beat it at any
      // specificity and the app rendered light with `.dark` on <html>.
      const out = wrapCss('.dark{--rl-color-bg:#0d0d0f;background:var(--rl-color-bg)}') ?? ''
      expect(beforeLayer(out)).toContain('--rl-color-bg:#0d0d0f')
      expect(beforeLayer(out)).not.toContain('background')
      expect(insideLayer(out)).toContain('background')
    })

    it('splits a merged palette nested in @media too', () => {
      const src = '@media(prefers-color-scheme:dark){:root:not(.light){--a:2;background:#000}}'
      const out = wrapCss(src) ?? ''
      expect(beforeLayer(out)).toContain('--a:2')
      expect(beforeLayer(out)).not.toContain('background')
      expect(insideLayer(out)).toContain('background')
    })

    it('does NOT carve out a comma-selector list containing :root', () => {
      const out = wrapCss(':root, .x{--a:1}.b{c:d}') ?? ''
      expect(insideLayer(out)).toContain('.x')
    })
  })

  describe('prelude at-rules stay ahead of the layer', () => {
    it('hoists @import above the layer (it is illegal inside one)', () => {
      // Browsers DROP an @import nested in @layer, so a wrapped @import
      // silently loses the imported stylesheet entirely.
      const out = wrapCss('@import url("x.css");.a{color:red}') ?? ''
      expect(out.indexOf('@import')).toBeLessThan(layerStart(out))
      expect(insideLayer(out)).not.toContain('@import')
    })

    it('keeps @charset as the very first bytes', () => {
      const out = wrapCss('@charset "UTF-8";.a{color:red}') ?? ''
      expect(out.startsWith('@charset')).toBe(true)
      expect(insideLayer(out)).not.toContain('@charset')
    })
  })

  describe('output is always structurally valid', () => {
    // These are the assertions whose absence let a regex implementation ship
    // corrupt CSS while every `toContain` test stayed green.
    const cases: Record<string, string> = {
      empty: '',
      simple: '.a{color:red}',
      tokens: ':root{--a:1}.b{c:d}',
      'consecutive tokens': ':root{--a:1}:root.dark{--a:2}',
      'comment containing a closing brace': ':root{/* } */--a:1}.b{c:d}',
      'string containing a closing brace': '.a{content:"}"}.b{c:d}',
      'url containing braces': '.a{background:url("a}b.png")}.b{c:d}',
      nesting: ':root{--a:1;& .x{color:red}}.b{color:blue}',
      'media query': '@media(min-width:0){.a{color:red}}',
      'font-face': '@font-face{font-family:X;src:url(x.woff2)}',
      keyframes: '@keyframes spin{from{transform:none}to{transform:rotate(1turn)}}',
      supports: '@supports(display:grid){.a{display:grid}}',
      'import then rules': '@import url("x.css");.a{color:red}',
      'charset then rules': '@charset "UTF-8";.a{color:red}',
    }

    for (const [name, css] of Object.entries(cases)) {
      it(`parses cleanly and round-trips: ${name}`, () => {
        const out = wrapCss(css)
        expect(out).not.toBeNull()
        const result = out ?? ''

        // Does a real CSS parser accept it? Counting raw `{`/`}` would be
        // wrong here — braces legitimately appear inside comments, strings and
        // url() — so structural validity is the parser's answer, not a tally.
        expect(() => postcss.parse(result)).not.toThrow()

        // Re-stringifying a parsed tree must be stable: if the wrap had spliced
        // text into a comment or split a block, the parse would land somewhere
        // different and this would drift.
        expect(postcss.parse(result).toString()).toBe(result)
      })

      it(`preserves every declaration: ${name}`, () => {
        // Guards against content being dropped or hoisted out of its rule —
        // the failure mode where output stays balanced but means something
        // different.
        const declsOf = (css: string) => {
          const found: string[] = []
          postcss.parse(css).walkDecls((d) => {
            found.push(`${d.prop}:${d.value}`)
            return undefined
          })
          return found.sort()
        }
        expect(declsOf(wrapCss(css) ?? '')).toEqual(declsOf(css))
      })
    }
  })
})
