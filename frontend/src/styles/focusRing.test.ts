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

  it('every 2px box-shadow ring resolves through a custom property', () => {
    const offenders: string[] = []
    for (const f of files) {
      const text = readFileSync(f, 'utf8')
      for (const [i, line] of text.split('\n').entries()) {
        if (!/box-shadow:\s*0 0 0 2px/.test(line)) continue
        if (/^\s*(\*|\/\*|\/\/)/.test(line)) continue
        // A ring must name a var(); a raw colour is what this ticket removed.
        if (!/var\(--/.test(line)) offenders.push(`${rel(f)}:${i + 1}  ${line.trim()}`)
      }
    }
    expect(offenders, `ring with a non-token colour:\n${offenders.join('\n')}`).toEqual([])
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

  it('does not use !important, so operator custom.css can still win', () => {
    // The @layer rela wrap means an unlayered operator rule already outranks
    // this; a layered !important would invert that (see docs/customisation.md).
    //
    // Checked against DECLARATIONS only. The file's own comment explains why
    // !important is avoided, so a naive substring match finds its own prose
    // and fails on a correct file — which it did on first run.
    const declarations = css
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .split('\n')
      .filter((l) => l.includes(':'))
      .join('\n')
    expect(declarations).not.toContain('!important')
  })
})
