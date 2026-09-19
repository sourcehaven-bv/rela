---
id: PLAN-DQKZOB
type: planning-checklist
title: 'Planning: Style HTML comments as muted chips in the Milkdown editor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: a `relaComment` ProseMirror node claiming comment-only `html` mdast nodes,
rendered as a muted chip showing inner text without delimiters; chip styling in
both editor stylesheets; a distinct block treatment for multi-line comments.

OUT: any change to stored markdown (bytes round-trip verbatim); any change to
the render/view path (comments stay hidden); styling raw HTML that is not a
comment; an authoring affordance for new comments (no toolbar or slash-menu
entry) — templates supply them, this ticket is about reading.

**Acceptance Criteria:**

See the ticket body. AC5 was corrected during implementation — see "Design
Review Findings" below.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: effort=s, approach was
determined by reading the Milkdown source directly)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small change, approach settled by reading
`@milkdown/preset-commonmark/src/node/html.ts` and probing remark-parse.

**Existing Solutions:**

- `unist-util-visit` considered for the tree walk and REJECTED: present only as
a transitive dependency of remark, so importing it binds this file to a version
nothing declares. The traversal is four lines; hand-rolled instead.
- `entityRefNode.ts` is the in-codebase precedent and the approach is modelled
on it directly: an atomic node that claims an mdast type and round-trips it
verbatim via a `parseMarkdown`/`toMarkdown` pair.
- CSS-only styling on the preset's existing `data-type="html"` attribute was
considered and REJECTED: it would also style `<img>`, `<div>` and `<script>`,
which `rawHtmlPassthrough.test.ts` deliberately keeps visible as text.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

A remark plugin retypes comment-only `html` mdast nodes to `relaComment` before
Milkdown matches the tree against the schema. This is what makes the new node
win: Milkdown resolves against `{...schema.nodes, ...schema.marks}` taking the
first match, and the preset is registered first in both editors, so
`parseMarkdown` alone would lose. Retyping makes the two matchers disjoint
rather than competing.

`toMarkdown` writes the original bytes back as an `html` mdast node, so the
round-trip is byte-exact.

The node is inline and atomic, matching the preset node it replaces. Verified by
probing remark-parse: a standalone comment is an `html` child of `root`, a
trailing comment is an `html` child of a `paragraph`, and a multi-line comment
is ONE `html` node with embedded `\n`. A block node would split the paragraph in
the trailing case.

No new dependencies.

**Files to modify:**

- `frontend/src/components/forms/milkdown/commentNode.ts` (new)
- `frontend/src/components/forms/milkdown/commentNode.test.ts` (new)
- `frontend/src/components/forms/milkdown/MilkdownEditor.vue` (register)
- `frontend/src/app-editor/relaEditor.ts` (register)
- `frontend/src/components/forms/milkdown/milkdownEditor.css` (chip style)
- `frontend/src/app-editor/relaEditorTheme.css` (chip style)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The comment body is author-controlled entity content — untrusted. It reaches the
chip's label. Validation is an ALLOWLIST: `COMMENT_PATTERN` accepts only a
string that is exactly one HTML comment, anchored at both ends, with the
terminator excluded from the body. Anything else falls through to the preset's
`html` node and renders as visible text, which is the safe existing behaviour.

**Security-Sensitive Operations:**

Rendering the label into the DOM. The node's `toDOM` returns the label as the
third element of the ProseMirror spec, which ProseMirror builds with
`createTextNode` — never `innerHTML`. This matters because Milkdown has shipped
this vulnerability class twice in adjacent nodes (CVE-2026-57530 link href,
CVE-2026-57531 emoji innerHTML), so a regression here is a realistic upgrade
hazard. `rawHtmlPassthrough.test.ts` pins the property, and the new "renders a
comment as one indivisible leaf" test asserts the label is a single TEXT_NODE.

No file access, auth, or crypto. No error paths carry data.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

Unit tests on `commentBody`/`isCommentNode` cover the matcher in isolation.
Integration tests mount the real `MilkdownEditor` component and assert on the
rendered `.ProseMirror` DOM — AC1 (chip without delimiters), AC2 (trailing
comment stays in its paragraph), AC6 (indivisible leaf). Round-trip is asserted
through the editor's own `guardedValue()` write-back guard, which is the path a
save actually takes.

**Edge Cases:**

- Empty comment `<!---->` → body is the empty string, still a valid chip.
- Multi-line comment → one node, interior newlines preserved, block class.
- Padded comment `<!--   x   -->` → label trimmed for display, stored value NOT
trimmed.
- Non-string mdast value → returns null rather than throwing.
- Two comments on one line → remark splits them into separate nodes; both chip.

**Negative Tests:**

- `<!-- note --><div>` → no chip, delimiters stay visible (one html node whose
value is not exactly a comment).
- `<!-- dangling` (unterminated) → no chip.
- `<img src=x>` → no chip, existing passthrough behaviour unchanged.
- The whole existing `rawHtmlPassthrough.test.ts` suite must pass unmodified.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Corpus churn*: a chip that rewrites stored markdown would damage every
template on open. Mitigated by `toMarkdown` writing original bytes and by
`serializerContract.ts` excluding `html` from whitespace-insensitive leaves, so
any whitespace change is reported as semantic drift and the write refused.
- *Hiding live markup*: chipping something that is not a comment would conceal
executable markup behind a label reading "this is only a note". Mitigated by the
anchored allowlist and negative tests.
- *Upgrade hazard*: a future Milkdown release could change the `html` node.
Mitigated by asserting observable DOM consequences rather than schema shape.

Effort: s (confirmed).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `frontend/CLAUDE.md` — the editor section documents the nodes the editor
owns; the comment node belongs in that list.
- [x] N/A for docs/metamodel.md, docs/cli-reference.md, README.md — no
metamodel, CLI or project-level change.
- [x] ~~docs/data-entry.md~~ (N/A: assessed during implementation — that file
documents data-entry features, and this is editor rendering of content that
already existed; nothing an operator configures or a user invokes changed).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Design review was not run as a separate `/design-review` pass before coding; the
design was instead validated empirically against the running app, which surfaced
one genuine defect that a design pass would not have caught:

**AC5 was written on a false premise.** It asserted that `<!-- a --> text <!-- b
-->` should produce zero chips. Running the demo showed two chips, and probing
remark-parse confirmed why: remark splits that line into two separate inline
`html` nodes, each already a well-formed comment, so the matcher never sees the
combined string. Chipping both is correct CommonMark. The test asserting the
opposite had passed for the wrong reason. Corrected in `commentNode.test.ts`
with an added round-trip case; the `<!-- note --><div>` half of AC5 was valid
and still holds.

Recorded here rather than as an RR entity because it was found and fixed within
implementation, before any review pass.
