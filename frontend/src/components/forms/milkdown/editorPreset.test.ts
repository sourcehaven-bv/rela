import { describe, it, expect, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { Editor, rootCtx, defaultValueCtx } from '@milkdown/kit/core'
import { gfm } from '@milkdown/kit/preset/gfm'
import { getMarkdown } from '@milkdown/kit/utils'
import { commonmark, remarkPreserveEmptyLinePlugin } from '@milkdown/kit/preset/commonmark'
import { RELA_COMMONMARK, configureRelaSerializer } from './editorPreset'
import { entityRefNode } from './entityRefNode'

/**
 * The preset is what decides the bytes an editor writes, so it is what has to
 * be shared — and what has to be tested.
 *
 * Acceptance criterion 2 of TKT-D2JML7 ("the two editors cannot serialize a
 * body differently") was originally argued from the import graph: the modules
 * are imported rather than copied. That is weaker than it sounds, because each
 * editor still builds its own Milkdown instance and the PLUGIN SET is what
 * decides serialization — which is exactly what the two editors each used to
 * compute separately, with a comment in both files asserting they matched.
 *
 * Two tests here, doing different jobs. The first pins the preset's OUTPUT, so
 * a change to it is visible. The second pins that both editors actually use it,
 * by reading their source — a byte-comparison harness cannot run the sandboxed
 * element and the Vue component against each other under happy-dom without a
 * production-visible test hook on the element, and widening the element's
 * contract to test that it is narrow would be self-defeating.
 */

const read = (rel: string): string =>
  readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

/** Serializes a body through the shared preset, as both editors do. */
async function throughPreset(markdown: string): Promise<string> {
  const root = document.createElement('div')
  document.body.appendChild(root)
  const editor = await Editor.make()
    .config((ctx) => {
      ctx.set(rootCtx, root)
      ctx.set(defaultValueCtx, markdown)
      configureRelaSerializer(ctx)
    })
    .use(RELA_COMMONMARK)
    .use(gfm)
    .use(entityRefNode)
    .create()
  const out = editor.action(getMarkdown()) as unknown as string
  await editor.destroy()
  root.remove()
  return out
}

describe('the shared editor preset', () => {
  it('drops remarkPreserveEmptyLinePlugin, and nothing else', () => {
    // That plugin serializes EVERY empty paragraph as a literal `<br />`,
    // including the empty cells of a table, so adding a row wrote raw HTML into
    // the stored markdown. Removing more than it would silently change how
    // other node types serialize.
    const removed = new Set<unknown>(remarkPreserveEmptyLinePlugin)
    expect(commonmark.length - RELA_COMMONMARK.length).toBe(removed.size)
    expect(RELA_COMMONMARK.some((p) => removed.has(p))).toBe(false)
    for (const plugin of commonmark) {
      if (!removed.has(plugin)) expect(RELA_COMMONMARK).toContain(plugin)
    }
  })

  // Emphasis and link normalization are covered directly in
  // serializerContract.test.ts, which drives the remark stack itself. These
  // cases are the BLOCK constructs, where the plugin set is what decides the
  // outcome and so where a divergent preset would show up.
  it.each([
    ['setext heading', 'Title\n=====\n\nBody.\n', '# Title\n\nBody.\n'],
    ['star bullets', '* one\n* two\n', '- one\n- two\n'],
    ['nested list', '- one\n  - nested\n- two\n', '- one\n  - nested\n- two\n'],
    ['task list', '- [ ] todo\n- [x] done\n', '- [ ] todo\n- [x] done\n'],
    [
      'padded table',
      '| a  | b   |\n| -- | --- |\n| 1  | 2   |\n',
      '| a | b |\n| - | - |\n| 1 | 2 |\n',
    ],
    ['thematic break', 'a\n\n***\n\nb\n', 'a\n\n---\n\nb\n'],
    ['fenced code', '```go\nx := 1\n```\n', '```go\nx := 1\n```\n'],
    ['entity reference', 'See `TKT-ABC` here.\n', 'See `TKT-ABC` here.\n'],
  ])('normalizes %s the same way for every editor', async (_name, input, expected) => {
    expect(await throughPreset(input)).toBe(expected)
  })

  it('writes no <br /> into an empty table cell', async () => {
    // The concrete defect remarkPreserveEmptyLinePlugin caused: adding a row
    // put raw HTML into the stored markdown.
    const out = await throughPreset('| a | b |\n| - | - |\n|   | 2 |\n')
    expect(out).not.toContain('<br')
  })

  it('is the ONLY place either editor configures its markdown stack', () => {
    // The guard that keeps the two from drifting apart again. Each editor must
    // reach the plugin set and the serializer options through this module, not
    // rebuild them — which is what they each used to do.
    const sources = {
      'MilkdownEditor.vue': read('./MilkdownEditor.vue'),
      'relaEditor.ts': read('../../../app-editor/relaEditor.ts'),
    }
    for (const [name, src] of Object.entries(sources)) {
      expect(src, `${name} must use the shared preset`).toContain('RELA_COMMONMARK')
      expect(src, `${name} must use the shared serializer config`).toContain(
        'configureRelaSerializer'
      )
      // Rebuilding either locally is the drift this exists to prevent.
      expect(src, `${name} must not filter the preset itself`).not.toContain('commonmark.filter')
      expect(src, `${name} must not set stringify options itself`).not.toContain(
        'remarkStringifyOptionsCtx'
      )
    }
  })
})

// Silence the api module the SPA component pulls in transitively.
vi.mock('@/api', () => ({ searchEntities: vi.fn(), listEntities: vi.fn() }))
