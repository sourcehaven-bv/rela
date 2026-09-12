import { describe, it, expect } from 'vitest'
import { Editor, rootCtx, defaultValueCtx, editorStateCtx } from '@milkdown/kit/core'
import { commonmark } from '@milkdown/kit/preset/commonmark'
import { gfm } from '@milkdown/kit/preset/gfm'
import { getMarkdown } from '@milkdown/kit/utils'
import type { EditorState } from '@milkdown/kit/prose/state'
import { entityRefNode } from './entityRefNode'
import { buildResolutionTransaction, isResolutionTransaction } from './entityRefResolution'
import type { EntityRefResolver } from '@/utils/markdown'

async function makeEditor(initial: string) {
  const root = document.createElement('div')
  document.body.appendChild(root)
  const editor = await Editor.make()
    .config((ctx) => {
      ctx.set(rootCtx, root)
      ctx.set(defaultValueCtx, initial)
    })
    .use(commonmark)
    .use(gfm)
    .use(entityRefNode)
    .create()
  const state = editor.ctx.get(editorStateCtx) as EditorState
  return { editor, state, root }
}

function refAttrs(state: EditorState) {
  const out: Array<Record<string, unknown>> = []
  state.doc.descendants((node) => {
    if (node.type.name === 'entityRef') out.push({ ...node.attrs })
  })
  return out
}

describe('entity ref resolution', () => {
  it('leaves the id alone and the title null when nothing resolves', async () => {
    const { editor, state, root } = await makeEditor('See `TKT-ABC`.\n')
    expect(buildResolutionTransaction(state, undefined)).toBeNull()
    expect(refAttrs(state)).toEqual([
      {
        id: 'TKT-ABC',
        title: null,
        entityType: null,
        inaccessible: false,
        resolvedFromServer: false,
      },
    ])
    await editor.destroy()
    root.remove()
  })

  it('applies a resolved title and type', async () => {
    const { editor, state, root } = await makeEditor('See `TKT-ABC`.\n')
    const resolver: EntityRefResolver = (id) =>
      id === 'TKT-ABC' ? { type: 'ticket', title: 'Fix the thing' } : null
    const tr = buildResolutionTransaction(state, resolver)
    expect(tr).not.toBeNull()
    const next = state.apply(tr!)
    expect(refAttrs(next)).toEqual([
      {
        id: 'TKT-ABC',
        title: 'Fix the thing',
        entityType: 'ticket',
        inaccessible: false,
        // Recorded so a later map that omits this id clears the title rather
        // than leaving a stale one on screen.
        resolvedFromServer: true,
      },
    ])
    await editor.destroy()
    root.remove()
  })

  it('does not write the resolved title into the markdown', async () => {
    const { editor, state, root } = await makeEditor('See `TKT-ABC`.\n')
    const resolver: EntityRefResolver = () => ({ type: 'ticket', title: 'Secret Title' })
    const tr = buildResolutionTransaction(state, resolver)!
    const next = state.apply(tr)
    editor.ctx.set(editorStateCtx, next)
    const md = editor.action(getMarkdown())
    expect(md).toBe('See `TKT-ABC`.\n')
    expect(md).not.toContain('Secret Title')
    await editor.destroy()
    root.remove()
  })

  it('leaves a reference the resolver refuses as its bare id', async () => {
    // An entity the principal may not read produces no mention at all. The
    // reference must stay, showing the id, rather than being rewritten or
    // dropped.
    const { editor, state, root } = await makeEditor('See `TKT-HIDDEN`.\n')
    const resolver: EntityRefResolver = () => null
    expect(buildResolutionTransaction(state, resolver)).toBeNull()
    expect(refAttrs(state)[0]).toMatchObject({ id: 'TKT-HIDDEN', title: null })
    await editor.destroy()
    root.remove()
  })

  it('carries the inaccessible flag through', async () => {
    const { editor, state, root } = await makeEditor('See `TKT-LOCKED`.\n')
    const resolver: EntityRefResolver = () => ({
      type: 'ticket',
      title: 'TKT-LOCKED',
      inaccessible: true,
      inaccessibleReason: 'git-crypt',
    })
    const next = state.apply(buildResolutionTransaction(state, resolver)!)
    expect(refAttrs(next)[0]).toMatchObject({
      inaccessible: true,
      title: 'TKT-LOCKED',
    })
    await editor.destroy()
    root.remove()
  })

  it('treats a throwing resolver as no answer', async () => {
    const { editor, state, root } = await makeEditor('See `TKT-ABC`.\n')
    const resolver: EntityRefResolver = () => {
      throw new Error('boom')
    }
    expect(buildResolutionTransaction(state, resolver)).toBeNull()
    await editor.destroy()
    root.remove()
  })

  it('marks its transaction so a change listener can ignore it', async () => {
    const { editor, state, root } = await makeEditor('See `TKT-ABC`.\n')
    const resolver: EntityRefResolver = () => ({ type: 'ticket', title: 'T' })
    const tr = buildResolutionTransaction(state, resolver)!
    expect(isResolutionTransaction(tr)).toBe(true)
    expect(tr.getMeta('addToHistory')).toBe(false)
    await editor.destroy()
    root.remove()
  })

  it('keeps a title the picker supplied when the resolver has no entry', async () => {
    // Regression: a just-inserted reference cannot be in the mentions map,
    // which the server computed at load. The resolver returned null for it and
    // the plugin blanked the title the picker had just put on the node, so a
    // freshly inserted reference displayed as a bare ID.
    const { editor, state, root } = await makeEditor('placeholder\n')
    const node = state.schema.nodes.entityRef.create({
      id: 'TKT-NEW',
      title: 'Freshly Picked',
      entityType: 'ticket',
      inaccessible: false,
    })
    const withRef = state.apply(state.tr.replaceWith(0, 0, node))

    // A resolver that knows nothing about this id, as at load time.
    const resolver: EntityRefResolver = () => null
    expect(buildResolutionTransaction(withRef, resolver)).toBeNull()
    expect(refAttrs(withRef)[0]).toMatchObject({
      id: 'TKT-NEW',
      title: 'Freshly Picked',
      entityType: 'ticket',
    })
    await editor.destroy()
    root.remove()
  })

  it('still upgrades a bare reference once the resolver knows it', async () => {
    // The complement: keeping an existing title must not stop a reference that
    // has none from picking one up.
    const { editor, state, root } = await makeEditor('See `TKT-ABC`.\n')
    const resolver: EntityRefResolver = () => ({ type: 'ticket', title: 'Now Known' })
    const next = state.apply(buildResolutionTransaction(state, resolver)!)
    expect(refAttrs(next)[0]).toMatchObject({ title: 'Now Known', entityType: 'ticket' })
    await editor.destroy()
    root.remove()
  })

  it('resolves every reference in the document', async () => {
    const { editor, state, root } = await makeEditor('`TKT-A` and `TKT-B`\n')
    const resolver: EntityRefResolver = (id) => ({ type: 'ticket', title: `T:${id}` })
    const next = state.apply(buildResolutionTransaction(state, resolver)!)
    expect(refAttrs(next).map((a) => a.title)).toEqual(['T:TKT-A', 'T:TKT-B'])
    await editor.destroy()
    root.remove()
  })

  // A title must not outlive the grant that produced it. The node keeps a
  // picker-supplied title (the server has not weighed in on that id yet), but
  // once the server HAS resolved it, a later map that omits it means access
  // was withdrawn or the entity is gone — and the title has to go with it.
  it('clears a server-resolved title when a later map omits the id', async () => {
    const { editor, state, root } = await makeEditor('See `TKT-ABC`.\n')

    const granted: EntityRefResolver = (id) =>
      id === 'TKT-ABC' ? { type: 'ticket', title: 'Fix the thing' } : null
    const resolved = state.apply(buildResolutionTransaction(state, granted)!)
    expect(refAttrs(resolved)[0]).toMatchObject({
      title: 'Fix the thing',
      resolvedFromServer: true,
    })

    // Access revoked: the map is present but no longer carries this id.
    const revoked: EntityRefResolver = () => null
    const cleared = resolved.apply(buildResolutionTransaction(resolved, revoked)!)
    expect(refAttrs(cleared)[0]).toMatchObject({
      id: 'TKT-ABC',
      title: null,
      entityType: null,
      resolvedFromServer: false,
    })

    await editor.destroy()
    root.remove()
  })

  // The distinction that makes the above safe: no map at all is not the same
  // as a map that declined the id. A response carrying no mention data must
  // not blank titles the editor already has.
  it('keeps a server-resolved title when there is no map at all', async () => {
    const { editor, state, root } = await makeEditor('See `TKT-ABC`.\n')
    const granted: EntityRefResolver = () => ({ type: 'ticket', title: 'Fix the thing' })
    const resolved = state.apply(buildResolutionTransaction(state, granted)!)

    expect(buildResolutionTransaction(resolved, undefined)).toBeNull()
    expect(refAttrs(resolved)[0]).toMatchObject({ title: 'Fix the thing' })

    await editor.destroy()
    root.remove()
  })
})
