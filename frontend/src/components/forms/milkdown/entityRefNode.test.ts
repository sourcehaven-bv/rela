import { describe, it, expect } from 'vitest'
import { Editor, rootCtx, defaultValueCtx } from '@milkdown/kit/core'
import { commonmark } from '@milkdown/kit/preset/commonmark'
import { gfm } from '@milkdown/kit/preset/gfm'
import { getMarkdown } from '@milkdown/kit/utils'
import { entityRefNode, isValidEntityRefId, looksLikeEntityRef } from './entityRefNode'
import { RELA_STRINGIFY_OPTIONS } from './serializerContract'
import { remarkStringifyOptionsCtx } from '@milkdown/kit/core'

/**
 * Mounts a real editor over `initial` and returns the markdown it serializes
 * back. This drives the actual parse/serialize pair rather than the specs in
 * isolation, so a mismatch between them is caught here.
 */
async function roundTripThroughEditor(initial: string): Promise<string> {
  const root = document.createElement('div')
  document.body.appendChild(root)
  const editor = await Editor.make()
    .config((ctx) => {
      ctx.set(rootCtx, root)
      ctx.set(defaultValueCtx, initial)
      ctx.update(remarkStringifyOptionsCtx, (prev) => ({
        ...prev,
        ...RELA_STRINGIFY_OPTIONS,
      }))
    })
    .use(commonmark)
    .use(gfm)
    .use(entityRefNode)
    .create()

  const out = editor.action(getMarkdown())
  await editor.destroy()
  root.remove()
  return out
}

describe('isValidEntityRefId', () => {
  it('accepts the id shapes the store accepts', () => {
    for (const id of ['TKT-U2R7GU', 'BUG-R9EHKV', 'iso-27001-a.5.1', 'x', '123abc']) {
      expect(isValidEntityRefId(id)).toBe(true)
    }
  })

  it('rejects ids that would break the code span or the store', () => {
    for (const id of [
      '',
      'has space',
      'has`backtick',
      'a--b',
      'a/b',
      'a\\b',
      'a\nb',
      'a\tb',
      'x'.repeat(1025),
    ]) {
      expect(isValidEntityRefId(id)).toBe(false)
    }
  })

  it('rejects non-strings', () => {
    for (const v of [null, undefined, 42, {}, []]) {
      expect(isValidEntityRefId(v)).toBe(false)
    }
  })
})

describe('looksLikeEntityRef', () => {
  it('does not consult the graph', () => {
    // A reference to something that does not exist is still a reference. It
    // must survive a round-trip, or an unreadable ref would be silently
    // rewritten on save. Shape is the only test applied.
    expect(looksLikeEntityRef('TKT-NO-SUCH-ENTITY')).toBe(true)
    expect(looksLikeEntityRef('TKT-NOPE')).toBe(true)
  })

  it('rejects a prose code span', () => {
    expect(looksLikeEntityRef('go test ./...')).toBe(false)
    expect(looksLikeEntityRef('a b')).toBe(false)
  })
})

describe('entityRef node round-trip', () => {
  it('writes an entity ref back as the same code span', async () => {
    const src = 'See `TKT-U2R7GU` for detail.\n'
    expect(await roundTripThroughEditor(src)).toBe(src)
  })

  it('preserves several refs in one paragraph', async () => {
    const src = 'Both `TKT-ABC` and `BUG-XYZ` apply.\n'
    expect(await roundTripThroughEditor(src)).toBe(src)
  })

  it('leaves a non-ref code span as inline code', async () => {
    const src = 'Run `go test ./...` first.\n'
    expect(await roundTripThroughEditor(src)).toBe(src)
  })

  it('preserves a ref inside a list item and a heading', async () => {
    const src = '## About `TKT-ABC`\n\n- see `BUG-XYZ`\n'
    expect(await roundTripThroughEditor(src)).toBe(src)
  })

  // A table is re-padded to its column widths on serialization, so the bytes
  // change even though nothing was edited. That is byte churn, not drift: the
  // write-back guard suppresses it at save time. What must hold is that the
  // ref inside the cell survives and the result is a fixed point.
  it('preserves a ref inside a table cell, modulo cell padding', async () => {
    const src = '| ref | note |\n| --- | ---- |\n| `TKT-ABC` | yes |\n'
    const once = await roundTripThroughEditor(src)
    expect(once).toContain('`TKT-ABC`')
    expect(await roundTripThroughEditor(once)).toBe(once)
  })

  it('does not rewrite a ref to an entity that cannot be resolved', async () => {
    // No mentions map is installed here at all, so nothing resolves. The
    // markdown must come back byte-identical regardless.
    const src = 'Ref `TKT-UNREADABLE` stays.\n'
    expect(await roundTripThroughEditor(src)).toBe(src)
  })

  it('leaves a fenced code block containing a ref-shaped token alone', async () => {
    const src = '```\nTKT-ABC\n```\n'
    expect(await roundTripThroughEditor(src)).toBe(src)
  })
})
