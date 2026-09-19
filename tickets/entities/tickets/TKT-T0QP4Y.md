---
id: TKT-T0QP4Y
type: ticket
title: Style HTML comments as muted chips in the Milkdown editor
kind: enhancement
priority: medium
effort: s
status: review
---

## Description

HTML comments are how rela's templates carry authoring guidance. Every file
under `tickets/templates/entities/` opens its sections with one, and the
checklist automations copy them into real entities. A reader never sees them:
the entity view renders through `marked` + DOMPurify
(`frontend/src/utils/markdown.ts:107`), and DOMPurify strips comment nodes.

The editor is the one surface that shows them, and it shows them badly. The
commonmark preset's `html` node renders its value as a bare `<span>` of text
(`@milkdown/preset-commonmark/src/node/html.ts`), and no rule in
`milkdownEditor.css` or `relaEditorTheme.css` targets it. So `<!-- Document what
IS and IS NOT in scope -->` sits in the document wearing `.md-body` body
typography, indistinguishable from prose the author was supposed to have
written.

Two properties of the current behaviour are worth keeping, and this ticket
preserves both: a comment is a single atomic unit to delete, and the rendered
view hides it.

### Scope

IN:

- A `relaComment` ProseMirror node that claims comment-only `html` mdast nodes
and renders them as a muted chip showing the inner text without delimiters.
- Chip styling in both editor stylesheets (`milkdownEditor.css`,
`relaEditorTheme.css`), following the `entity-ref` precedent of one rule per
shell prefix.
- A distinct block treatment for a multi-line comment.

OUT:

- Any change to the stored markdown. The bytes round-trip verbatim.
- Any change to the render/view path. Comments stay hidden there.
- Styling raw HTML that is not a comment. `<img>`, `<div>` and `<script>` keep
rendering as visible text, which is what `rawHtmlPassthrough.test.ts`
deliberately pins.
- An affordance for authoring a new comment (no toolbar button, no slash-menu
entry). Templates supply them; this ticket is about reading.

### Approach

A comment-only `html` node is renamed to `relaComment` by a remark plugin before
Milkdown matches the tree against the schema. That is what makes the new node
win: Milkdown resolves a markdown node against `{...schema.nodes,
...schema.marks}` and takes the first match, and the preset is registered first
in both editors, so `parseMarkdown` alone would lose to the preset's `html`
node. Renaming makes the two matchers disjoint instead of competing.

`toMarkdown` writes the original bytes back as an `html` mdast node, so the
round-trip is byte-exact.

The inline/block distinction falls out of how remark already parses the two
shapes, confirmed by probing remark-parse directly:

- A standalone comment is an `html` child of `root`.
- A trailing comment (`**Research Doc:** <!-- Link RES-xxxx -->`) is an `html`
child of a `paragraph`.
- A multi-line comment is ONE `html` node with embedded `\n`.

The node is therefore inline and atomic, matching the preset node it replaces. A
block node would split the paragraph in the trailing case and change the
document's shape.

Files:

- `frontend/src/components/forms/milkdown/commentNode.ts` (new)
- `frontend/src/components/forms/milkdown/MilkdownEditor.vue` (register)
- `frontend/src/app-editor/relaEditor.ts` (register)
- `frontend/src/components/forms/milkdown/milkdownEditor.css` (chip style)
- `frontend/src/app-editor/relaEditorTheme.css` (chip style)

### Constraints

Two existing guards bound the implementation:

- `serializerContract.ts:101` excludes `html` from
`WHITESPACE_INSENSITIVE_LEAVES`, so the write-back guard treats any whitespace
change inside a comment as semantic drift and refuses the write. The multi-line
`Options` scaffold in `research.md` is the case this protects.
- `rawHtmlPassthrough.test.ts` pins that raw HTML never becomes live DOM. The
chip must render its text as a ProseMirror child (`createTextNode`), never via
`innerHTML`. A comment body is author-controlled but still untrusted, and
Milkdown has shipped this vulnerability class twice (CVE-2026-57530,
CVE-2026-57531).

### Acceptance Criteria

1. A standalone comment renders as a chip showing its inner text with no
`<!--` or `-->` visible. Test: mount the editor on a template body, assert the
rendered text excludes the delimiters and the chip element exists.
2. A trailing comment stays on the same line as the text preceding it. Test:
assert `**Research Doc:** <!-- ... -->` produces one paragraph containing both
the strong mark and the chip.
3. A multi-line comment round-trips byte-identically, including its interior
newlines. Test: load the `research.md` Options block through the editor and
assert `guardedValue()` returns the source unchanged with no emit.
4. Raw HTML that is not a comment is unaffected. Test: the existing
`rawHtmlPassthrough.test.ts` suite continues to pass unmodified.
5. A string that is not exactly one comment (`<!-- note --><div>`) falls
through to the preset's html node and stays visible as text. Test: assert no
chip element is created. Note `<!-- a --> text <!-- b -->` is NOT such a case:
remark splits it into two separate well-formed comment nodes before the matcher
sees it, so both correctly chip. Test: assert two chips and a byte-exact
round-trip.
6. The chip is deletable as a single unit. Test: assert the node is atomic and
selectable.
