import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'

// TKT-FRING7. Focus rings are `var(--focus-ring)`, never a hardcoded colour.
//
// These are GREP tests over the source, deliberately, and it is worth saying
// why rather than reaching for a mounted-component assertion: Vitest does not
// apply an SFC's scoped <style>, so `getComputedStyle(...).boxShadow` on a
// mounted widget resolves against no CSS at all and passes whatever the
// stylesheet says. TKT-CBSTYLE shipped a real cascade bug underneath exactly
// that kind of green test. Reading the source is the only assertion here that
// can actually fail for the right reason.

const SRC = resolve(__dirname, '..')

function walk(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules') continue
    const full = join(dir, name)
    if (statSync(full).isDirectory()) walk(full, out)
    else if (/\.(vue|css)$/.test(full)) out.push(full)
  }
  return out
}

const files = walk(SRC)
const rel = (f: string) => f.slice(SRC.length + 1)

describe('focus rings use the shared token', () => {
  // The literal that was swept. Matches the rgba() form only — a bare
  // `#6366f1` is NOT matched on purpose: `utils/palette.ts` uses it as the
  // default palette's accent VALUE, which is legitimate and must not be
  // "fixed". Widening this pattern to the hex would break that file.
  const INDIGO = /rgba?\(\s*99\s*,\s*102\s*,\s*241/

  it('no rgba(99, 102, 241, …) literal remains in any .vue or .css source', () => {
    const offenders: string[] = []
    for (const f of files) {
      const text = readFileSync(f, 'utf8')
      for (const [i, line] of text.split('\n').entries()) {
        // Skip prose: several files explain in a comment which literal they
        // replaced, and that reference should not trip the guard.
        const isComment = /^\s*(\*|\/\*|\/\/)/.test(line)
        // A literal used as a var() FALLBACK is dead code — the custom
        // property it backs is unconditionally defined in tokens.css, so it
        // can never render. Out of scope for this ticket (see the planning
        // checklist): sweeping ~47 of them is churn for zero visual change.
        // The guard tracks what RENDERS, so it ignores them too.
        const isDeadFallback = /var\(--[\w-]+,\s*rgba?\(/.test(line)
        if (!isComment && !isDeadFallback && INDIGO.test(line)) {
          offenders.push(`${rel(f)}:${i + 1}  ${line.trim()}`)
        }
      }
    }
    expect(offenders, `hardcoded indigo found:\n${offenders.join('\n')}`).toEqual([])
  })

  // Generalised deliberately. The first version of this test matched
  // `box-shadow: 0 0 0 2px` and only the two rgba triples being swept, which
  // enforces "don't reintroduce THESE colours at THIS width" — not the actual
  // invariant. It missed a live offender in the same tree:
  // DocumentsPanel's `0 0 0 3px rgba(59, 130, 246, 0.1)`, a hardcoded BLUE at
  // 3px, wrong on both axes the narrow regex keyed on.
  //
  // This version matches any ring-shaped box-shadow at any width and any
  // colour notation, and requires a custom property.
  it('every ring-shaped box-shadow resolves through a custom property', () => {
    // Matched against the WHOLE declaration, not a line. Prettier wraps a
    // two-shadow ring across three lines, so a line-scoped regex never sees
    // `box-shadow:` and its colour together — which let a novel hardcoded
    // colour through in exactly the rings this ticket created. Verified by
    // mutation: `0 0 0 4px #00ff00` on a continuation line passed the
    // line-based version and fails this one.
    const COLOUR_LITERAL = /#[0-9a-f]{3,8}\b|rgba?\(|hsla?\(/i
    // A spread-only shadow (`0 0 0 <n>px`) is a ring; anything with a real
    // offset or blur is a drop shadow and not a focus indicator.
    const RING_SPREAD = /(^|,)\s*0\s+0\s+0\s+[\d.]+px/
    const offenders: string[] = []
    for (const f of files) {
      const text = readFileSync(f, 'utf8')
      // Strip comments first so prose examples never trip the guard.
      const code = text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '')
      for (const m of code.matchAll(/box-shadow:\s*([^;}]*)[;}]/g)) {
        const value = m[1]
        if (!RING_SPREAD.test(value)) continue
        if (COLOUR_LITERAL.test(value.replace(/var\(--[\w-]+,[^)]*\)/g, ''))) {
          const line = code.slice(0, m.index).split('\n').length
          offenders.push(`${rel(f)}:${line}  box-shadow: ${value.trim().replace(/\s+/g, ' ').slice(0, 80)}`)
        }
      }
    }
    expect(offenders, `ring with a hardcoded colour:\n${offenders.join('\n')}`).toEqual([])
  })

  // The error-state ring has the identical defect and was found BY the guard
  // above, not by the survey that scoped this ticket: six widgets drew
  // `rgba(239, 68, 68, …)`, a red that is not --error-color (#e5484d /
  // #f87171) and does not follow the theme.
  it('no rgba(239, 68, 68, …) error-ring literal remains', () => {
    const RED = /rgba?\(\s*239\s*,\s*68\s*,\s*68/
    const offenders: string[] = []
    for (const f of files) {
      const text = readFileSync(f, 'utf8')
      for (const [i, line] of text.split('\n').entries()) {
        if (/^\s*(\*|\/\*|\/\/)/.test(line)) continue
        if (/var\(--[\w-]+,\s*rgba?\(/.test(line)) continue
        if (RED.test(line)) offenders.push(`${rel(f)}:${i + 1}  ${line.trim()}`)
      }
    }
    expect(offenders, `hardcoded error red found:\n${offenders.join('\n')}`).toEqual([])
  })

  it('the palette default keeps its #6366f1 (the guard must not overreach)', () => {
    // Negative test for the guard itself: this value is a legitimate default
    // palette accent. If a future "cleanup" sweeps it, palette assignment
    // changes silently — so pin that it is still here and still untouched.
    const palette = readFileSync(join(SRC, 'utils', 'palette.ts'), 'utf8')
    expect(palette).toContain('#6366f1')
  })
})

describe('forced-colors focus fallback', () => {
  const css = readFileSync(join(SRC, 'styles', 'focus-ring.css'), 'utf8')

  it('restores a real outline under forced-colors', () => {
    expect(css).toMatch(/@media\s*\(forced-colors:\s*active\)/)
    expect(css).toMatch(/outline:\s*2px solid Highlight/)
  })

  it('is imported from main.ts, or the rule never ships', () => {
    const main = readFileSync(join(SRC, 'main.ts'), 'utf8')
    expect(main).toContain("import './styles/focus-ring.css'")
  })

  // This test previously asserted the OPPOSITE — that `!important` is absent —
  // and in doing so pinned the bug in place. Without it the rule loses the
  // cascade to every `input:focus { outline: none }` it exists to override
  // ((0,1,0) vs (0,1,1), and Vue's scoped [data-v-x] widens the gap), so the
  // control ends up with no indicator at all under forced-colors. Verified in
  // Chrome: with `!important` the fallback wins; without it, it does not.
  it('uses !important, without which the rule loses the cascade and does nothing', () => {
    const declarations = css
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .split('\n')
      .filter((l) => l.includes(':'))
      .join('\n')
    expect(declarations).toContain('outline: 2px solid Highlight !important')
  })
})
