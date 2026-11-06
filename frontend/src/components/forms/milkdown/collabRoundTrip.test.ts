import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'
import * as Y from 'yjs'
import { prosemirrorToYXmlFragment, yXmlFragmentToProseMirrorRootNode } from 'y-prosemirror'
import { Editor, rootCtx, parserCtx, serializerCtx, schemaCtx } from '@milkdown/kit/core'
import type { Ctx } from '@milkdown/kit/ctx'
import { gfm } from '@milkdown/kit/preset/gfm'
import type { Node as ProseNode } from '@milkdown/kit/prose/model'
import { RELA_COMMONMARK, RELA_OUTPUT_NODES, configureRelaSerializer } from './editorPreset'
import { entityRefNode } from './entityRefNode'
import { taskList } from './taskListItem'

/**
 * Co-editing (TKT-CJL604) stores the editor's document in a Yjs fragment and
 * rebuilds it on every peer. The fragment must carry everything the
 * serializer reads, or a body loses meaning on its way through the room and
 * the write-back guard refuses the save. This pins the round trip for the
 * repository's own bodies, with the editor's own plugin set.
 */

let editor: Editor
let ctx: Ctx

beforeAll(async () => {
  const root = document.createElement('div')
  document.body.appendChild(root)
  editor = await Editor.make()
    .config((c) => {
      c.set(rootCtx, root)
      configureRelaSerializer(c)
    })
    .use(RELA_COMMONMARK)
    .use(gfm)
    .use(entityRefNode)
    .use(RELA_OUTPUT_NODES)
    .use(taskList)
    .create()
  ctx = editor.ctx
})

afterAll(async () => {
  await editor.destroy()
})

/** Parses, sends through a room, and rebuilds on another peer. */
function throughRoom(markdown: string): { before: ProseNode; after: ProseNode } {
  const parse = ctx.get(parserCtx)
  const schema = ctx.get(schemaCtx)
  const before = parse(markdown)
  const a = new Y.Doc()
  prosemirrorToYXmlFragment(before, a.getXmlFragment('prosemirror'))
  const b = new Y.Doc()
  Y.applyUpdate(b, Y.encodeStateAsUpdate(a))
  const after = yXmlFragmentToProseMirrorRootNode(b.getXmlFragment('prosemirror'), schema)
  return { before, after }
}

function serialize(node: ProseNode): string {
  return ctx.get(serializerCtx)(node)
}

describe('a body through a co-editing room', () => {
  it.each([
    ['paragraphs', 'one\n\ntwo\n'],
    ['marks', '**bold** _em_ `code` ~~gone~~ [link](https://x.example)\n'],
    ['entity reference', 'See `TKT-U2R7GU` here.\n'],
    ['entity reference in bold', 'See **`TKT-U2R7GU`** here.\n'],
    ['entity reference in a link', '[`TKT-U2R7GU`](https://x.example)\n'],
    ['aligned table', '| a | b | c |\n| :- | :-: | -: |\n| 1 | 2 | 3 |\n'],
    ['plain table', '| a | b |\n| - | - |\n| 1 | 2 |\n'],
    ['task list', '- [ ] todo\n- [x] done\n'],
    ['ordered list', '3. three\n4. four\n'],
    ['code block', '```go\nx := 1\n```\n'],
    ['comment chip', '<!-- guidance -->\n\nText.\n'],
    ['image', '![alt](https://x.example/a.png "t")\n'],
    ['hard break', 'a\\\nb\n'],
    ['blockquote', '> quoted\n'],
    ['heading', '## Title\n'],
  ])('keeps %s', (_name, markdown) => {
    const { before, after } = throughRoom(markdown)
    expect(after.toJSON()).toEqual(before.toJSON())
    expect(serialize(after)).toBe(serialize(before))
  })

  const repoRoot = resolve(process.cwd(), '..')
  const roots = [join(repoRoot, 'tickets/entities'), join(repoRoot, 'docs-project')]
  const all = roots.flatMap((r) => walk(r))
  const FULL = process.env.RELA_FULL_CORPUS === '1'
  const files = FULL ? all : all.filter((_, i) => i % 8 === 0)

  it('keeps every corpus body', () => {
    expect(files.length).toBeGreaterThan(100)
    const lost: string[] = []
    for (const file of files) {
      const src = body(readFileSync(file, 'utf8'))
      if (!src.trim()) continue
      const { before, after } = throughRoom(src)
      if (serialize(after) !== serialize(before)) lost.push(file)
    }
    expect(lost).toEqual([])
  }, 300_000)
})

describe('view-only entity reference attributes', () => {
  const LOCAL = ['title', 'entityType', 'entityId', 'inaccessible', 'resolvedFromServer']

  function resolved(): ProseNode {
    const doc = ctx.get(parserCtx)('See `TKT-U2R7GU` here.\n')
    const json = doc.toJSON()
    const ref = json.content[0].content.find((n: { type: string }) => n.type === 'entityRef')
    ref.attrs = { ...ref.attrs, title: 'Hidden title', entityType: 'ticket', entityId: 'TKT-U2R7GU' }
    return ctx.get(schemaCtx).nodeFromJSON(json)
  }

  function refElement(frag: Y.XmlFragment): Y.XmlElement {
    const para = frag.get(0) as Y.XmlElement
    return para.toArray().find((c) => c instanceof Y.XmlElement) as Y.XmlElement
  }

  it('never enter the room', () => {
    const doc = new Y.Doc()
    prosemirrorToYXmlFragment(resolved(), doc.getXmlFragment('prosemirror'))
    const attrs = refElement(doc.getXmlFragment('prosemirror')).getAttributes()
    expect(attrs.id).toBe('TKT-U2R7GU')
    for (const key of LOCAL) expect(attrs).not.toHaveProperty(key)
  })

  it('are dropped when a peer stores them anyway', () => {
    const doc = new Y.Doc()
    const frag = doc.getXmlFragment('prosemirror')
    prosemirrorToYXmlFragment(resolved(), frag)
    refElement(frag).setAttribute('title', 'Planted')
    const node = yXmlFragmentToProseMirrorRootNode(frag, ctx.get(schemaCtx))
    let title: unknown = 'missing'
    node.descendants((n) => {
      if (n.type.name === 'entityRef') title = n.attrs.title
    })
    expect(title).not.toBe('Planted')
  })
})

function body(raw: string): string {
  if (!raw.startsWith('---\n')) return raw
  const end = raw.indexOf('\n---\n', 4)
  if (end === -1) return raw
  return raw.slice(end + 5).replace(/^\n+/, '')
}

function walk(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) walk(full, out)
    else if (name.endsWith('.md')) out.push(full)
  }
  return out
}
