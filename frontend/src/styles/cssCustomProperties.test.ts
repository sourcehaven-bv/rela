import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'

// Every `var(--x)` without a fallback must name a custom property that is
// defined somewhere. An undefined one makes the declaration invalid at
// computed-value time, so `background` silently becomes transparent and
// `color` falls back to the inherited value. Nothing else reports it: the
// build, lint and typecheck all pass, and Vitest does not apply scoped SFC
// styles, so a mounted-component assertion cannot see it either.
//
// The menu in SectionCreateButton shipped with `var(--bg-primary)` and had no
// background at all. That name, like `--text-primary` or `--bg-card`, comes
// from another design system and was never defined here.
//
// Limits: this checks names, not scope. A property declared in one
// component's scoped style counts as defined everywhere. Only .vue, .css and
// .ts files under src/ and the rela-components library are read, so a
// property that only a dependency's stylesheet defines needs a fallback.

const SRC = resolve(__dirname, '..')
const LIBRARY = resolve(SRC, '..', 'packages', 'rela-components', 'src')

function walk(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules') continue
    const full = join(dir, name)
    if (statSync(full).isDirectory()) walk(full, out)
    else if (/\.(vue|css|ts)$/.test(full) && !/\.(test|spec|stories)\.ts$/.test(full))
      out.push(full)
  }
  return out
}

/** Drops block comments, and `//` line comments that start a line or follow code. */
function stripComments(text: string): string {
  return text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[\s;{}(),])\/\/.*$/gm, '$1')
}

/**
 * Names a source defines. A stylesheet, inline `style` or `:style` object key
 * declares `--x:` (or `'--x':`); script can also call `setProperty('--x', …)`.
 */
function definitions(source: string): string[] {
  const text = stripComments(source)
  return [
    ...[...text.matchAll(/(?<![\w-])['"`]?(--[\w-]+)['"`]?\s*:/g)].map((m) => m[1]),
    ...[...text.matchAll(/setProperty\(\s*['"`](--[\w-]+)['"`]/g)].map((m) => m[1]),
  ]
}

/** Names a source reads with `var(--x)` and no fallback. */
function usesWithoutFallback(source: string): string[] {
  return [...stripComments(source).matchAll(/var\(\s*(--[\w-]+)\s*\)/g)].map((m) => m[1])
}

const root = resolve(SRC, '..')
const sources = new Map(
  [...walk(SRC), ...walk(LIBRARY)].map((f) => [f.slice(root.length + 1), readFileSync(f, 'utf8')])
)
const defined = new Set([...sources.values()].flatMap(definitions))

describe('CSS custom properties', () => {
  it('every var() without a fallback names a defined property', () => {
    const offenders = new Set<string>()
    let scanned = 0
    for (const [file, text] of sources) {
      for (const name of usesWithoutFallback(text)) {
        scanned++
        if (!defined.has(name)) offenders.add(`${file}  var(${name})`)
      }
    }
    // Guards the scanner: a broken pattern would find nothing and pass.
    expect(scanned).toBeGreaterThan(500)
    const list = [...offenders]
    expect(
      list,
      `undefined custom properties (use an --rl-color-* token):\n${list.join('\n')}`
    ).toEqual([])
  })

  it('reads definitions and uses the way the browser does', () => {
    const vue = `
      <template><div :style="{ '--span': n }" /></template>
      <script setup lang="ts">
      // var(--in-a-comment)
      el.style.setProperty('--set', '1')
      const cls = '--modifier'
      </script>
      <style scoped>
      /* --commented: red; */
      .a--b:hover { --own: 1px; color: var(
        --missing
      ); background: var(--with-fallback, red); }
      </style>`
    expect(definitions(vue).sort()).toEqual(['--own', '--set', '--span'])
    expect(usesWithoutFallback(vue)).toEqual(['--missing'])
  })
})
