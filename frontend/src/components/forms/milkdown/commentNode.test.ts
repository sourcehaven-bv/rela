// HTML comments rendered as chips in the Milkdown editor (TKT-T0QP4Y).
//
// Templates carry authoring guidance in HTML comments, and the rendered view
// drops them. The editor used to show them as raw unstyled text, so a section
// prompt read exactly like prose the author had written. These tests pin the
// chip rendering AND, more importantly, the two properties the chip must not
// break: the stored bytes round-trip verbatim, and raw HTML that is not a
// comment keeps rendering as visible text.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import MilkdownEditor from './MilkdownEditor.vue'
import { commentBody, isCommentNode } from './commentNode'

const searchEntities = vi.fn()
vi.mock('@/api', () => ({
  searchEntities: (...args: unknown[]) => searchEntities(...args),
  listEntities: vi.fn(),
}))

async function mountEditor(modelValue: string) {
  const wrapper = mount(MilkdownEditor, {
    props: { modelValue },
    attachTo: document.body,
  })
  await flushPromises()
  await flushPromises()
  return wrapper
}

/** Reads the editor's write-back value without emitting, as the guard does. */
function guarded(w: ReturnType<typeof mount>) {
  return (
    w.vm as unknown as { guardedValue: () => { value: string; verdict: string } }
  ).guardedValue()
}

describe('commentBody', () => {
  it('extracts the inner text of a single comment', () => {
    expect(commentBody('<!-- hello -->')).toBe(' hello ')
  })

  it('keeps interior newlines', () => {
    expect(commentBody('<!-- a\nb -->')).toBe(' a\nb ')
  })

  // Anchoring matters: these are raw markup, not comments, and must stay
  // visible as text rather than being hidden behind a chip.
  it.each([
    ['a comment followed by markup', '<!-- note --><div>'],
    ['two comments with text between', '<!-- a --> text <!-- b -->'],
    ['markup before a comment', '<div><!-- note -->'],
    ['plain markup', '<img src=x>'],
    ['an unterminated comment', '<!-- dangling'],
  ])('rejects %s', (_label, src) => {
    expect(commentBody(src)).toBeNull()
  })

  it('accepts an empty comment', () => {
    expect(commentBody('<!---->')).toBe('')
  })

  it('returns null for a non-string value', () => {
    expect(commentBody(undefined)).toBeNull()
    expect(commentBody(42)).toBeNull()
  })
})

describe('isCommentNode', () => {
  it('claims only html nodes holding one comment', () => {
    expect(isCommentNode({ type: 'html', value: '<!-- x -->' })).toBe(true)
    expect(isCommentNode({ type: 'html', value: '<div>' })).toBe(false)
    expect(isCommentNode({ type: 'text', value: '<!-- x -->' })).toBe(false)
  })
})

describe('MilkdownEditor comment chips', () => {
  beforeEach(() => {
    searchEntities.mockReset()
    searchEntities.mockResolvedValue({ data: [] })
  })
  afterEach(() => {
    document.body.innerHTML = ''
  })

  // AC1: the chip shows the guidance, not the delimiters.
  it('renders a standalone comment as a chip without delimiters', async () => {
    const w = await mountEditor('<!-- Document what IS and IS NOT in scope -->\n')
    const editor = w.find('.ProseMirror')

    const chip = editor.find('.rela-comment')
    expect(chip.exists()).toBe(true)
    expect(chip.text()).toBe('Document what IS and IS NOT in scope')
    expect(editor.text()).not.toContain('<!--')
    expect(editor.text()).not.toContain('-->')
    w.unmount()
  })

  // AC2: a trailing comment must not split the paragraph it follows. This is
  // why the node is inline rather than block.
  it('keeps a trailing comment in the paragraph it follows', async () => {
    const w = await mountEditor('**Research Doc:** <!-- Link RES-xxxx, or N/A -->\n')
    const paragraphs = w.find('.ProseMirror').findAll('p')

    expect(paragraphs).toHaveLength(1)
    expect(paragraphs[0].find('strong').text()).toBe('Research Doc:')
    expect(paragraphs[0].find('.rela-comment').text()).toBe('Link RES-xxxx, or N/A')
    w.unmount()
  })

  // A multi-line comment gets the block treatment, driven by the rendered
  // class rather than a second node type.
  it('marks a multi-line comment as block', async () => {
    const w = await mountEditor('<!-- For each option:\n- **Pros**: good\n-->\n')
    const chip = w.find('.ProseMirror').find('.rela-comment')

    expect(chip.exists()).toBe(true)
    expect(chip.classes()).toContain('rela-comment-block')
    w.unmount()
  })

  // AC5: not-quite-a-comment falls through to the preset's html node and
  // stays visible. Hiding this would conceal live markup behind a chip that
  // says "this is only a note".
  //
  // `<div>` opens an HTML block, so CommonMark puts the comment and the tag in
  // ONE html node whose value is not exactly a comment — which is the case
  // COMMENT_PATTERN's end anchor rejects.
  it('does not chip a comment followed by markup', async () => {
    const w = await mountEditor('<!-- note --><div>\n')
    const editor = w.find('.ProseMirror')

    expect(editor.find('.rela-comment').exists()).toBe(false)
    expect(editor.text()).toContain('<!--')
    w.unmount()
  })

  // Two comments on one line are TWO chips, not zero.
  //
  // `commentBody` rejects the combined string, but it never sees it: remark
  // splits `<!-- a --> text <!-- b -->` into separate inline html nodes per
  // comment, so each arrives already well-formed. That is correct CommonMark —
  // they genuinely are two comments — and this test exists because an earlier
  // version of it asserted the opposite and passed for the wrong reason.
  it('chips each of two comments on one line', async () => {
    const w = await mountEditor('Raw: <!-- a --> text <!-- b -->.\n')
    const editor = w.find('.ProseMirror')

    const chips = editor.findAll('.rela-comment')
    expect(chips.map((c) => c.text())).toEqual(['a', 'b'])
    // The prose between and around them is untouched.
    expect(editor.text()).toContain('text')
    expect(editor.text()).not.toContain('<!--')
    w.unmount()
  })

  // AC6: one unit to select and delete, which is the gesture for clearing a
  // prompt once the section is written.
  //
  // Asserted through the DOM rather than the schema spec, for the reason
  // rawHtmlPassthrough.test.ts gives: the observable consequence keeps holding
  // however the node is implemented. An atom renders as a single contenteditable
  // leaf, so the caret cannot be placed between two characters of the label —
  // which is what stops an edit turning a comment into stray visible markup.
  it('renders a comment as one indivisible leaf', async () => {
    const w = await mountEditor('<!-- guidance -->\n')
    const chip = w.find('.ProseMirror').find('.rela-comment')

    expect(chip.attributes('contenteditable')).toBe('false')
    // One text child, not per-character spans ProseMirror could edit into.
    expect(chip.element.childNodes).toHaveLength(1)
    expect(chip.element.childNodes[0].nodeType).toBe(Node.TEXT_NODE)
    w.unmount()
  })
})

// The round-trip is the property that protects the stored corpus. A chip that
// renders beautifully but rewrites research.md on open is a regression, not a
// feature, so these assert through the editor's own write-back guard.
describe('MilkdownEditor comment round-trip', () => {
  beforeEach(() => {
    searchEntities.mockReset()
    searchEntities.mockResolvedValue({ data: [] })
  })
  afterEach(() => {
    document.body.innerHTML = ''
  })

  // AC3: the case serializerContract.ts protects by excluding `html` from the
  // whitespace-insensitive leaves. Interior newlines and indentation are
  // content here.
  it('round-trips a multi-line comment byte-identically', async () => {
    const src = `## Options

<!-- For each option:
### Option N: Name
- **Approach**: What would we do?
- **Effort**: Rough estimate
-->
`
    const w = await mountEditor(src)

    expect(guarded(w).value).toContain('<!-- For each option:\n### Option N: Name')
    expect(w.emitted('update:modelValue')).toBeUndefined()
    w.unmount()
  })

  it('round-trips a trailing comment without moving it', async () => {
    const src = '**Research Doc:** <!-- Link RES-xxxx, or N/A -->\n'
    const w = await mountEditor(src)

    expect(guarded(w).value).toContain('**Research Doc:** <!-- Link RES-xxxx, or N/A -->')
    expect(w.emitted('update:modelValue')).toBeUndefined()
    w.unmount()
  })

  // Two chips in one paragraph must serialize back to two comments with the
  // prose between them intact, not merge or reorder.
  it('round-trips two comments on one line', async () => {
    const src = 'Raw: <!-- a --> text <!-- b -->.\n'
    const w = await mountEditor(src)

    expect(guarded(w).value).toContain('<!-- a --> text <!-- b -->')
    expect(w.emitted('update:modelValue')).toBeUndefined()
    w.unmount()
  })

  // Display trims the body for the chip label; the stored value must not be
  // trimmed with it.
  it('preserves the comment spacing exactly as written', async () => {
    const src = '<!--   padded   -->\n'
    const w = await mountEditor(src)

    expect(guarded(w).value).toContain('<!--   padded   -->')
    w.unmount()
  })
})
