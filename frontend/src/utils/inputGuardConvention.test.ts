import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'

/**
 * BUG-DNP5E7. Keyboard handlers guard with `isInputFocused()`, never an inline
 * element-type check.
 *
 * This is a GREP test over the source, for the reason that motivates
 * `styles/focusRing.test.ts`: the behavioural tests beside the two handlers
 * (Sidebar.shortcut.test.ts, SearchView.shortcut.test.ts) only see the handlers
 * that exist today. The defect guarded against is a THIRD handler written later
 * with its own partial copy of the rule — which compiles, type-checks, lints
 * clean, and fails silently by firing a shortcut inside a contenteditable
 * surface.
 *
 * ## What this can and cannot catch
 *
 * A regex over source is a heuristic. Two deliberate choices bound how leaky it
 * is, both made after an adversarial review found real bypasses:
 *
 * - **Whole-file matching, not line-by-line.** A guard split across lines by
 *   Prettier, or hoisted into a `const TAGS = [...]` used later, is invisible
 *   to a per-line loop *by construction*. Comments are stripped first so prose
 *   describing the old guard does not trip it.
 * - **`isInputFocused` must appear in the same HANDLER, not merely the same
 *   file.** A file-level check passes a second, unguarded handler added to a
 *   file that already imports the helper — which is precisely the shape of the
 *   bug being fixed, so the weaker check would miss its own recurrence.
 *
 * The honest long-term answer to the first point is an ESLint
 * `no-restricted-syntax` selector on the AST, which is immune to formatting and
 * aliasing. That is a larger change than this bug warrants; noted as follow-up.
 *
 * `utils/dom.ts` is the one place allowed to test element types: it IS the
 * shared guard.
 */

const SRC = resolve(__dirname, '..')

function walk(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules') continue
    const full = join(dir, name)
    if (statSync(full).isDirectory()) walk(full, out)
    else if (/\.(vue|ts)$/.test(full) && !/\.test\.ts$/.test(full)) out.push(full)
  }
  return out
}

const files = walk(SRC)
const rel = (f: string) => f.slice(SRC.length + 1)

/** Strips comments so prose about the old guard does not trip the patterns. */
function stripComments(text: string): string {
  return text
    .replace(/\/\*[\s\S]*?\*\//g, ' ')
    .replace(/<!--[\s\S]*?-->/g, ' ')
    .replace(/^[ \t]*\/\/.*$/gm, ' ')
}

/**
 * Files exempt from the scan, each with a reason.
 *
 * Keep this list short. An entry here is a claim that an element-type test in
 * the file is NOT a keyboard input guard.
 */
const EXEMPT = new Map<string, string>([
  ['utils/dom.ts', 'implements isInputFocused() — the sanctioned element-type test'],
])

/**
 * Hand-rolled element-type guards, across the aliases that mean the same thing.
 *
 * `tagName`/`nodeName` return uppercase, `localName` lowercase, so the tag
 * alternation is case-insensitive and the property name carries the anchor.
 * Also covers `instanceof HTMLInputElement` and `matches`/`closest` against an
 * input selector, which are the same decision by another spelling.
 */
const HANDROLLED_TYPE_TEST = [
  /\b(?:tagName|nodeName|localName)\b[\s\S]{0,120}?['"](?:input|textarea|select)['"]/i,
  /['"](?:input|textarea|select)['"][\s\S]{0,120}?\b(?:tagName|nodeName|localName)\b/i,
  /\binstanceof\s+HTML(?:Input|TextArea|Select)Element\b/,
  // A selector naming a bare TAG. `(?<![.#\w-])` rejects a class or id that
  // merely contains the word — `.closest('.entity-target-select')` is a class
  // lookup, not an element-type test, and must not trip this.
  /\.(?:matches|closest)\(\s*['"][^'"]*(?<![.#\w-])(?:input|textarea|select)\b(?![\w-])[^'"]*['"]\s*\)/i,
]

/** `e.key === 'x'` / `!== 'x'` / `case 'x':` for a single printable character. */
const BARE_PRINTABLE = /(?:\.key\s*[!=]==?\s*|case\s+)['"][^'"\\]['"]/

/** A global keydown registration, on `document` or `window`. */
const GLOBAL_KEYDOWN = /(?:document|window)\.addEventListener\(\s*['"]keydown['"]/

/**
 * Splits source into candidate handler bodies by brace matching from each
 * `function`/arrow declaration that mentions a key comparison. Crude, but it is
 * what makes the guard check per-handler instead of per-file.
 */
function handlerBodies(text: string): string[] {
  const bodies: string[] = []
  const re = /(?:function\s+\w+\s*\([^)]*\)|=>)\s*\{/g
  let m: RegExpExecArray | null
  while ((m = re.exec(text)) !== null) {
    let depth = 0
    let i = m.index + m[0].length - 1
    const start = i
    for (; i < text.length; i++) {
      if (text[i] === '{') depth++
      else if (text[i] === '}') {
        depth--
        if (depth === 0) break
      }
    }
    bodies.push(text.slice(start, i + 1))
  }
  return bodies
}

describe('keyboard input guards use the shared helper', () => {
  it('no source file hand-rolls an element-type input check', () => {
    const offenders: string[] = []

    for (const f of files) {
      const relPath = rel(f)
      if (EXEMPT.has(relPath)) continue

      const text = stripComments(readFileSync(f, 'utf8'))
      for (const pattern of HANDROLLED_TYPE_TEST) {
        const hit = pattern.exec(text)
        if (hit) {
          offenders.push(`${relPath}  ${hit[0].replace(/\s+/g, ' ').trim().slice(0, 90)}`)
          break
        }
      }
    }

    expect(
      offenders,
      `Hand-rolled element-type input guard found. Use isInputFocused() from ` +
        `@/utils/dom — it also covers contenteditable (Milkdown/ProseMirror), ` +
        `<select> and CodeMirror, which an element-type test misses:\n${offenders.join('\n')}`
    ).toEqual([])
  })

  it('the exemption list only names files that still exist', () => {
    // A stale exemption silently widens the scan's blind spot.
    const present = new Set(files.map(rel))
    const stale = [...EXEMPT.keys()].filter((f) => !present.has(f))
    expect(stale, `stale exemptions: ${stale.join(', ')}`).toEqual([])
  })

  it('every global handler reacting to a bare printable key calls the guard', () => {
    // The positive half, and the assertion with real teeth: it is agnostic to
    // HOW a wrong guard is spelled. A `localName` check still fails here,
    // because what is asserted is the presence of the shared call.
    //
    // Scoped to BARE PRINTABLE keys, which is the whole risk surface. A handler
    // firing only on Escape, Enter or a Cmd/Ctrl chord must NOT be guarded —
    // those are meant to work from inside a text field (Escape blurs, Cmd+Enter
    // saves), and requiring isInputFocused() there would be a regression. Five
    // such handlers exist today and are correct.
    const offenders: string[] = []

    for (const f of files) {
      const text = stripComments(readFileSync(f, 'utf8'))
      if (!GLOBAL_KEYDOWN.test(text)) continue

      for (const body of handlerBodies(text)) {
        if (!BARE_PRINTABLE.test(body)) continue
        if (body.includes('isInputFocused')) continue
        const key = BARE_PRINTABLE.exec(body)?.[0] ?? '?'
        offenders.push(`${rel(f)}  handler matching ${key.trim()}`)
      }
    }

    expect(
      offenders,
      `A global keydown handler reacts to a bare printable key without calling ` +
        `isInputFocused() from @/utils/dom. Such a shortcut must not fire while ` +
        `the user is typing — including in a contenteditable surface ` +
        `(Milkdown/ProseMirror), which an element-type check misses:\n` +
        `${offenders.join('\n')}`
    ).toEqual([])
  })
})
