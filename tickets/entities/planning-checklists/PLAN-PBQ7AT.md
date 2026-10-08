---
id: PLAN-PBQ7AT
type: planning-checklist
title: 'Planning: Comments across markdown block boundaries'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Root causes (verified):

1. Highlight: `applyHighlights` (`frontend/src/utils/commentHighlight.ts:89`)
splices one `<mark>` pair into the source. Reproduced with marked: `##
<mark>Heading\n\nBody</mark> text` renders as
`<h2><mark>Heading</h2><p>Body</mark> text</p>`; only the heading is marked.
2. Re-anchoring: textanchor v0.2.0 phase 1 (exact, whole document) already
finds a cross-block quote, but phase 2 (fuzzy) is confined to one paragraph
(`resolve.go:185`, `fuzzy.go:148`). Any edit inside a cross-block range detaches
it.

Creation already works: no client or server check refuses a plain comment across
blocks (`TestCommentSuggestion_ResolveCheckAcrossBlocksIsNotSuggestable`).

**Scope:**

In scope:
- Per-block highlight segments for a text anchor (server computes, client
emits one `<mark>` per segment, same comment id).
- New textanchor resolve phase for multi-block quotes (block alignment + endpoint
fuzzy match), released as textanchor v0.3.0 and adopted by rela.
- Headings, paragraphs, list items (incl. tight lists), blockquotes, table
cells crossing into neighbouring blocks.
- e2e coverage of a selection dragged across blocks.

Out of scope:
- Suggested replacements across blocks (stay refused by `crossesBlock`).
- Diff-based anchor mapping through stored versions (postgres-only; possible
later fast path).
- Full tree edit distance (APTED/Zhang-Shasha) or learned probabilistic TED.
- Highlighting inside code blocks/spans (still skipped, as today).
- Changing the stored `TextAnchor` shape. No migration.

**Acceptance Criteria:**
1. Selecting from a heading into its first paragraph and commenting highlights
BOTH the heading text and the paragraph text, each block with its own mark;
clicking either opens the same thread.
2. Same for a range across two paragraphs, and across three tight list items.
3. A cross-block comment still resolves (exact band) after an unrelated edit
elsewhere and after fsstore's 80-column reflow.
4. A cross-block comment still resolves (uncertain band or better) after a
small edit inside the heading, inside the body, or a paragraph inserted inside
the range.
5. A cross-block comment detaches when the quoted blocks are deleted or
rewritten beyond the threshold; it is never silently placed on unrelated text.
6. A range covering a fenced code block in its middle highlights the prose
segments and skips the code segment (no raw `<mark>` text visible).
7. Single-block anchors resolve exactly as before (existing textanchor and rela
tests unchanged and passing).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-XRYX18 (existing anchoring research). A fresh survey of
tree-matching algorithms was done for this ticket; summary below.

**Existing Solutions:**
- W3C `RangeSelector` (start and end selectors resolved independently) is the
model for cross-block ranges.
- Hypothesis: quote + prefix/suffix + Myers fuzzy search; no structure.
- Phelps & Wilensky 2000 "Robust intra-document locations": redundant
structural + text descriptors with fallback. The approach here is that idea.
- Tree diff: GumTree (top-down isomorphic subtrees + bottom-up dice), Chawathe
LaDiff, APTED/RTED/Zhang-Shasha, X-Diff, XyDiff. Rejected as the primary
mechanism: markdown ASTs are shallow and wide, nearly all signal is in leaf
text, APTED has no moves and is O(n^3), and no Go implementation exists.
Block-sequence alignment with fuzzy similarity captures the useful part.
- Probabilistic/learned TED (Bernard et al.): needs training data, no
implementations. Rejected.
- ProseMirror Mapping / Yjs relative positions: need every edit to pass through
the editor; rela does not observe edits (RES-XRYX18 Option E). Rejected.
- Brush et al. CHI 2001: users prefer an orphan to a wrong placement. Kept as a
design rule (thresholds, uncertain band).
- In-tree: textanchor (`internal/comments/textresolve.go`), quotefind goldmark
walk, `mentions.go:150` goldmark segment access,
`closest('mark[data-comment-id]')` click handling (`EntityDetail.vue:684`)
already works with several marks.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

(Revised after design review; see the review responses on TKT-U32AUB.)

Part 1, textanchor v0.3.0 (`~/Work/textanchor`):

- Chunks, not an AST, on the resolve path. Both the quote and the document are
split with the existing `splitParagraphsWithOffsets` (blank-line chunks, the
unit phase 2 already uses). No parser enters the resolve path, so
`collapse.go`'s parser-free contract holds, and a source fragment is never
re-parsed out of context. Tight lists and a heading followed directly by text
are already one chunk and keep resolving through phase 2. A quote with one chunk
(including library callers that pass newline-free quotes) never enters phase 3.
- Phase 3, endpoint matching (W3C RangeSelector shape), for quotes of k >= 2
chunks. It runs ALONGSIDE phase 2 when phase 1 finds nothing, and the candidates
compete on score, so a body-only phase-2 candidate cannot shadow the full-range
one:
  1. Start endpoint: fuzzy-match q[0] against the best SUFFIX window of each
doc chunk i (token-overlap prefilter skips chunks sharing no tokens).
  2. End endpoint: fuzzy-match q[k-1] against the best PREFIX window of chunks
j in (i, i+k+2] (bounded slack for inserted/deleted chunks).
  3. Middle coverage: similarity of q[1..k-1) joined against doc chunks
(i, j) joined; empty middles on both sides score 1.
  4. Length-weighted similarity over the three parts must be >= 0.6
(phase 2's `fuzzyMinSimilarity`), and each endpoint must be >= 0.6, or the
candidate is discarded. This floor is what keeps context terms alone from
placing an anchor.
  5. Candidate score = weighted similarity x 0.8 (fuzzy penalty), then the
existing context scoring. Keep the best few per start chunk.
  6. Endpoint windows reuse `bestSubstringMatch` with its comparison budget.
Library cap: phase 3 skipped for quotes over 32 chunks.
- Prepared document: new `textanchor.NewDocument(doc)` holds the collapsed form
and chunk table; `Document.Resolve(anchor, opts)` reuses them. `Resolve` and
`ResolveAll` become thin wrappers, so the public API stays compatible.
- `quotefind.Segments(doc, start, end) []Range` (new) for highlighting. It
uses goldmark with the GFM Table, TaskList and Strikethrough extensions to match
marked's `gfm: true`; `RenderedTextWithMapping` gets the same extensions so
quote-finding and segmenting see the same blocks. Each segment is the range
intersected with one leaf block (heading, paragraph, text block, table cell),
clamped to that block's first and last `ast.Text` node so no segment edge falls
inside emphasis or link markup, and split around `CodeSpan` nodes. Code blocks
and HTML blocks produce no segment. `quotefind.NewDocument(doc)` parses once for
many segment calls.
- Tests + benchmark (50 anchors on 100 KB); tag v0.3.0.

Part 2, rela:

- `go.mod`: textanchor v0.3.0.
- `internal/comments/textresolve.go`: a `Body` type built once per entity read
(prepared textanchor document + quotefind document); `ResolveText`,
`ApplyReplacement` and `Acceptable` take it, so a list read parses the body
once, not once per comment. `TextMatch` gains `Segments []Range`.
- `internal/dataentry/comments_handler.go`: build the `Body` once in the
anchor context; `anchorWire` gets `segments` WITHOUT omitempty for located text
anchors. `[]` means "nothing markable" (all code).
- `frontend/src/api/comments.ts`, `EntityDetail.vue`, `commentHighlight.ts`:
`HighlightRange.segments` optional. Present: mark each segment separately
(back-to-front over all segments); `[]` marks nothing; absent (older server)
keeps current single-mark behaviour. Code check stays as a per-segment backstop.
Range-level overlap logic unchanged. Link chip after the first segment that
contains a link.
- `docs/comments.md`: cross-block comments, the single-block limit for
suggestions, and that a comment starting in a heading is disambiguated by its
surrounding text, not by heading context.

Known limitation (kept out of scope): an anchor whose selection starts in a
heading stores an empty `HeadingContext` (`anchor.go` sees the partial line `##
`). Changing extraction would change how existing stored anchors score, so
repeated sections are disambiguated by prefix/suffix, which tests pin.

**Alternatives rejected:**
- Client-side splitting with marked's lexer: duplicates block logic in a second
parser and ties highlight correctness to marked internals. The server is already
the authority on offsets.
- DOM highlighting after render (Range/CSS Highlight API): no source positions
in the rendered DOM; mermaid/PlantUML rewrite subtrees.
- Storing per-endpoint block fingerprints in `TextAnchor`: needs a storage
change and migration; the stored quote already carries the block structure.
- AST alignment of quote blocks on the resolve path (the first draft):
re-parsing a source fragment out of context mis-parses ordered lists, tables,
indented continuations and unclosed fences.

**Files to modify:**
- textanchor: `resolve.go`, `fuzzy.go`, new `document.go`, new
`crossblock.go` + `crossblock_test.go`, `quotefind/quotefind.go`, new
`quotefind/segments.go` + `segments_test.go`, `README.md`.
- rela: `go.mod`, `go.sum`, `internal/comments/textresolve.go`,
`internal/comments/textresolve_test.go`,
`internal/dataentry/comments_handler.go`,
`internal/dataentry/comments_handler_test.go`, `frontend/src/api/comments.ts`,
`frontend/src/utils/commentHighlight.ts`,
`frontend/src/utils/commentHighlight.test.ts`,
`frontend/src/components/entity/EntityDetail.vue`, `e2e/pages/comments.page.ts`,
`e2e/tests/comments.spec.ts`, `docs/comments.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Quote/prefix/suffix from the browser: unchanged validation (5 runes min,
`MaxQuoteBytes` 2000).
- Entity body: already readable by the caller; segments are offsets into it.

**Security-Sensitive Operations:**
- Resolve runs on every comment list read. Phase 3 must be bounded (block cap,
token prefilter, existing comparison budget) so a crafted quote or body cannot
make reads quadratic-expensive. Benchmark pins the bound.
- Segments are byte offsets clamped to the body; out-of-range or inverted
segments are dropped, and the client keeps its existing defensive checks.
- Mark HTML: no new tags or attributes, so the DOMPurify allowlist is unchanged.
- ACL: unchanged; offsets are computed on the face the reader may see
(BUG-R1PQY9 path).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1/2: textanchor `quotefind.Segments` table tests (heading+paragraph, two
paragraphs, tight list items, blockquote, table cells); rela handler test
asserts `segments` on the wire; vitest `applyHighlights` cross-block case
rendered through `renderMarkdown` (jsdom) asserts one mark per block with the
same id; e2e selects heading-into-body, comments, asserts two marks, clicks the
second, thread opens.
- AC3: textanchor test reflow + unrelated edit, including a multi-line
blockquote paragraph; rela `ResolveText` test.
- AC4: textanchor table tests: typo in heading, word changed in body,
paragraph inserted inside the range, list item reordered at the edge. e2e: edit
body text inside the range, reload, highlight still present.
- AC5: blocks deleted, blocks rewritten -> orphaned; quote rewritten but
prefix/suffix intact -> orphaned (similarity floor); similar text in another
section -> not chosen over the true one, or orphaned.
- Phase competition: heading + long body with one body edit, where a
body-only phase-2 candidate exists -> the full heading+body range wins.
- AC6: vitest with a fence inside the range, and a paragraph containing
inline code (prose around it still marked); `Segments` tests for emphasis at
block edges, links, tables (GFM) and task lists; vitest renders a table through
marked and asserts one mark per cell. e2e asserts marks via a
`highlightsFor(id)` locator.
- AC7: full existing textanchor and rela suites.

**Edge Cases:**
- Quote starts or ends mid-block; quote ends exactly at block end.
- Multibyte text in segments (byte offsets).
- Repeated identical sections (prefix/suffix decide; heading context is empty
when the selection starts in a heading).
- Quote of exactly 2 blocks where one is very short (heading of 3 runes).
- Nested lists, blockquote containing a list.
- Very large body (100 KB) with many comments: benchmark.
- Overlapping cross-block comments: the second is dropped from highlighting
as today, still listed.

**Negative Tests:**
- Middle blocks deleted and endpoints rewritten: detached, not misplaced.
- Quote with more blocks than the cap: phase 3 skipped, no panic.
- Empty body; segment outside body; inverted range.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- False-positive re-anchoring. Mitigation: phase 3 only after exact phases fail,
fuzzy penalty, existing 0.5 floor, uncertain band shown in UI, negative tests.
- goldmark (server) and marked (client) disagree on a block boundary, so a mark
could still straddle blocks. Mitigation: segments come from inline content lines
only; table-test the common shapes against marked in vitest.
- Read-path cost. Mitigation: block cap, token prefilter, benchmark.
- Library release coupling. Mitigation: textanchor change is additive; rela
bumps only after the tag exists.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/comments.md: cross-block comments, limits table, heading-start
disambiguation note.
- [x] textanchor README: phase 3 and `quotefind.Segments`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 12 findings (3 critical, 4 significant, 4 minor, 1
nit), linked to TKT-U32AUB via has-review-response. All addressed in this plan:
chunk-based phase 3 that competes with phase 2, 0.6 similarity floor per
endpoint and overall, GFM extensions + text-node clamping + code-span splits for
segments, prepare-once documents, explicit `[]` segments, chip after the first
link segment, blockquote reflow test, single-chunk quotes skip phase 3,
`highlightsFor(id)` in e2e. Heading context is recorded as a known limitation
with prefix/suffix tests instead.
