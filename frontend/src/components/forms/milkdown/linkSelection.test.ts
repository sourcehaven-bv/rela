import { describe, it, expect } from 'vitest'
import { Schema, type Node as PMNode } from '@milkdown/kit/prose/model'
import { EditorState, TextSelection } from '@milkdown/kit/prose/state'
import { findLinkAt, canApplyLink } from './linkSelection'

// Shaped like Milkdown's, carrying only what these functions name. `code_block`
// keeps `marks: ''` because that is the property the code-block case relies on:
// the schema refuses the mark, so no hand-written guard is needed.
const schema = new Schema({
  nodes: {
    doc: { content: 'block+' },
    paragraph: { group: 'block', content: 'inline*' },
    code_block: { group: 'block', content: 'text*', marks: '' },
    text: { group: 'inline' },
  },
  marks: {
    link: { attrs: { href: {}, title: { default: null } } },
    strong: {},
  },
})

const link = (href: string) => schema.marks.link.create({ href })

function docOf(...blocks: PMNode[]) {
  return schema.node('doc', null, blocks)
}

function para(...content: PMNode[]) {
  return schema.node('paragraph', null, content)
}

/** A state whose selection spans [from, to); collapsed when they are equal. */
function stateWith(doc: PMNode, from: number, to = from) {
  const state = EditorState.create({ schema, doc })
  return state.apply(state.tr.setSelection(TextSelection.create(state.doc, from, to)))
}

describe('findLinkAt', () => {
  it('returns null in a plain paragraph', () => {
    const state = stateWith(docOf(para(schema.text('hello'))), 2)
    expect(findLinkAt(state)).toBeNull()
  })

  it('finds the link under a collapsed cursor', () => {
    const doc = docOf(para(schema.text('docs', [link('https://a.test/')])))
    // doc(0) > paragraph(1) > text starts at 1, so 2 is inside "docs".
    const found = findLinkAt(stateWith(doc, 2))
    expect(found).toMatchObject({ href: 'https://a.test/', text: 'docs' })
  })

  it('returns the FULL extent, not the selected part', () => {
    // The property that makes retargeting safe: selecting one character of a
    // link still yields the whole link's range.
    const doc = docOf(para(schema.text('documentation', [link('https://a.test/')])))
    const found = findLinkAt(stateWith(doc, 2, 4))
    expect(found).not.toBeNull()
    expect(found?.text).toBe('documentation')
    expect(found?.to).toBe((found?.from ?? 0) + 'documentation'.length)
  })

  it('finds a link when the selection only partially overlaps it', () => {
    // THE case behind this module. `see [docs](url) here` with a selection
    // running from inside the plain text across part of the link: toggleMark
    // would delete the link here, so the caller must see it and retarget.
    const doc = docOf(
      para(
        schema.text('see '),
        schema.text('docs', [link('https://a.test/')]),
        schema.text(' here')
      )
    )
    const found = findLinkAt(stateWith(doc, 2, 7))
    expect(found).toMatchObject({ href: 'https://a.test/', text: 'docs' })
  })

  it('spans text nodes split by another mark', () => {
    // A link containing a bold word is several text nodes carrying one link
    // mark; the extent must cover all of them.
    const href = 'https://a.test/'
    const shared = link(href)
    const doc = docOf(
      para(
        schema.text('read ', [shared]),
        schema.text('this', [shared, schema.marks.strong.create()]),
        schema.text(' doc', [shared])
      )
    )
    const found = findLinkAt(stateWith(doc, 2))
    expect(found?.text).toBe('read this doc')
    expect(found?.href).toBe(href)
  })

  it('does not merge two adjacent links with different targets', () => {
    const doc = docOf(
      para(schema.text('one', [link('https://one.test/')]), schema.text('two', [link('https://two.test/')]))
    )
    const found = findLinkAt(stateWith(doc, 2))
    expect(found).toMatchObject({ href: 'https://one.test/', text: 'one' })
  })

  it('finds a link whose text is empty of surrounding context', () => {
    const doc = docOf(para(schema.text('x', [link('https://a.test/')])))
    expect(findLinkAt(stateWith(doc, 2))?.href).toBe('https://a.test/')
  })

  it('returns null for a selection that misses every link', () => {
    const doc = docOf(
      para(schema.text('plain '), schema.text('link', [link('https://a.test/')]))
    )
    // Selection covering only "pla".
    expect(findLinkAt(stateWith(doc, 1, 4))).toBeNull()
  })
})

describe('canApplyLink', () => {
  it('allows a link in a paragraph', () => {
    expect(canApplyLink(stateWith(docOf(para(schema.text('hello'))), 2, 4))).toBe(true)
  })

  it('refuses a link inside a code block', () => {
    // Not a hand-written guard: `marks: ''` means the schema itself refuses,
    // which is what keeps this out of the special-case list.
    const doc = docOf(schema.node('code_block', null, [schema.text('const x = 1')]))
    expect(canApplyLink(stateWith(doc, 2, 6))).toBe(false)
  })

  it('refuses a collapsed cursor inside a code block', () => {
    const doc = docOf(schema.node('code_block', null, [schema.text('const x = 1')]))
    expect(canApplyLink(stateWith(doc, 3))).toBe(false)
  })
})
