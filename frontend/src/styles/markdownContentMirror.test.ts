/// <reference types="node" />
// This test runs in Node (vitest) and needs node:fs / node:url. The project
// tsconfig scopes `types` to vitest/globals, so pull in node types file-locally
// rather than widening them project-wide.
//
// Guards the hand-maintained DRIFT between the SPA's shared markdown stylesheet
// and the sandboxed app-editor's inlined copy (TKT-D2JML7).
//
// The EasyMDE live-preview element rules exist in two places because the
// app-editor bundle inlines its own CSS and cannot load the SPA global sheet:
//   - styles/markdown-content.css      → `.EasyMDEContainer .editor-preview ...`
//   - app-editor/relaEditorTheme.css   → `.EasyMDEContainer .editor-preview ...`
// They must stay identical. This test extracts every `.editor-preview` rule
// from both files and asserts the selector→declaration map matches, so a
// change to one that isn't mirrored in the other fails CI instead of silently
// drifting (which is exactly the failure this whole stylesheet consolidation
// was undoing).
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, it, expect } from 'vitest'

const read = (rel: string) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

// Parse a stylesheet into a map of `selector` → normalized declaration block,
// keeping only rules whose selector mentions `.editor-preview`. Declarations
// are whitespace-normalized so formatting differences don't cause failures —
// only real value changes do. Comments are stripped first.
function editorPreviewRules(css: string): Map<string, string> {
  const withoutComments = css.replace(/\/\*[\s\S]*?\*\//g, '')
  const rules = new Map<string, string>()
  const ruleRe = /([^{}]+)\{([^{}]*)\}/g
  let match: RegExpExecArray | null
  while ((match = ruleRe.exec(withoutComments)) !== null) {
    const selector = match[1].trim().replace(/\s+/g, ' ')
    // Only the element rules inside the preview are mirrored. Bare container
    // selectors (`.editor-preview`, `.editor-preview-side`) carry chrome
    // (padding/background/border, the --md-list-pad definition) that differs
    // by host: in the SPA it lives in MarkdownEditor.vue's scoped block, in
    // the app-editor it's in relaEditorTheme.css. Compare only rules that
    // target a descendant element of `.editor-preview`.
    if (!/\.editor-preview\s+\S/.test(selector)) continue
    const body = match[2]
      .split(';')
      .map((d) => d.trim())
      .filter(Boolean)
      .sort()
      .join('; ')
    rules.set(selector, body)
  }
  return rules
}

describe('markdown-content editor-preview mirror', () => {
  const shared = editorPreviewRules(read('./markdown-content.css'))
  const mirror = editorPreviewRules(read('../app-editor/relaEditorTheme.css'))

  it('both files actually define editor-preview rules (guards a broken extractor)', () => {
    expect(shared.size).toBeGreaterThan(10)
    expect(mirror.size).toBeGreaterThan(10)
  })

  it('the shared sheet and the app-editor mirror define the same editor-preview rules', () => {
    // Compare as sorted selector lists first for a readable diff on mismatch.
    expect([...mirror.keys()].sort()).toEqual([...shared.keys()].sort())
  })

  it('every mirrored editor-preview rule has identical declarations', () => {
    for (const [selector, body] of shared) {
      expect(mirror.get(selector), `declarations for "${selector}" drifted`).toBe(body)
    }
  })
})
