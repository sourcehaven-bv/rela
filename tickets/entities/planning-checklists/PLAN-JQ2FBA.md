---
id: PLAN-JQ2FBA
type: planning-checklist
title: 'Planning: Replace EasyMDE with Milkdown (ProseMirror) in data-entry forms'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: the `MarkdownEditor` used by `DynamicForm` in the SPA; entity references as
first-class nodes; `@` completion; server-side `mentions` on single-entity GET;
toolbar (formatting, block types, table controls).

OUT: the sandboxed app editor (`src/app-editor/`, `<rela-editor>`). It keeps its
own EasyMDE build and its own `relaBacktick.ts`. Tracked separately as
TKT-D2JML7, held back because the app CSP carries no `'unsafe-inline'` on
`style-src`, which blocks the `style` attribute some editor chrome wants.

**Acceptance Criteria:**

1. Opening an entity and saving it without edits emits no change.
Test: corpus round-trip over every entity body in the repo, plus
`guardWriteBack` returning `unchanged`/`churn-suppressed`.
2. An entity reference renders as a titled link in the editor and serializes
back to the exact code span it came from. Test: `entityRefNode` round-trip
tests; an e2e test asserting the stored markdown does NOT contain the title.
3. A reference the principal may not read never leaks a title.
Test: Go tests substituting an allow-all reader to prove the assertions are
non-vacuous.
4. The app-editor bundle still builds on EasyMDE and the SPA bundle contains
no EasyMDE. Test: grep the built assets.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-AV732E

**Existing Solutions:**

- **Milkdown 7.22** (chosen). ProseMirror + remark, so the serializer is the
same mdast pipeline the rest of rela already reasons about. `@milkdown/kit`
bundles commonmark/gfm/history/listener/slash/block/cursor/trailing, so the
whole feature needs one dependency.
- **@milkdown/crepe** (rejected). The batteries-included distribution. It
ships its own theme and opinionated chrome, which fights the token system in
`styles/tokens.css` and `scales.css`, and it does not expose the serializer
tuning this needs.
- **Tiptap** (rejected). Also ProseMirror, comparable capability, but its
markdown support is a community extension rather than the core representation.
rela stores markdown, so a markdown-native serializer is the property that
matters most.
- **Keep EasyMDE** (rejected). Cannot render a reference as a title in place,
which is the point of the change.
- Prior art in-tree: `styles/markdown-content.css` is the documented single
source of truth for rendered markdown (TKT-W3OPRX, TKT-YYZRGW). The editor wears
`.md-body` and inherits it rather than restyling.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Phase 0 first, as a gate: pin `RELA_STRINGIFY_OPTIONS` and run the whole corpus
through parse → serialize, comparing a *semantic projection* of the mdast rather
than bytes. Byte churn is expected and suppressed at write-back; semantic drift
must be zero. Doing this before any UI work means the risky part is answered
with evidence rather than hope.

An entity reference is an atomic inline `$node('entityRef')` with `id`
serialized and `title`/`entityType`/`inaccessible` view-only. It matches
`inlineCode` whose value looks like an ID. Node-before-mark ordering in
`@milkdown/transformer`'s `#matchTarget` means a node wins over the built-in
`inlineCode` mark by being a node, independent of `priority`.

Titles arrive from the server's per-principal `mentions` map and are copied onto
nodes by a ProseMirror plugin whose transactions are marked so they do not count
as user edits.

**Files to modify:**

- new `frontend/src/components/forms/milkdown/` (editor, node, resolution,
guard, mention menu, toolbar, commands, availability, table commands)
- `frontend/src/components/forms/DynamicForm.vue` (swap the import, feed the
resolver)
- `frontend/src/components/entity/EntityDetail.vue` (share the resolver)
- `frontend/src/utils/entityRefResolver.ts` (new, shared)
- `internal/apiwire/v1/responses.go`, `internal/dataentry/api_v1.go`
(`mentions` on single-entity GET)
- delete `MarkdownEditor.vue`, `BacktickAutocompletePopup.vue`,
`insertEntityRef.ts`, `useBacktickAutocomplete.ts`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *Entity body markdown* (stored, author-controlled). Parsed by remark into a
ProseMirror document; no HTML is executed by the editor. Rendered output on the
read side keeps its existing DOMPurify path.
- *Entity IDs reaching `entityRef`* — validated by `isValidEntityRefId`,
which mirrors `storeutil.ValidateID` (rejects `--`, control characters, `/`,
`\`, backtick, space, >1024 bytes). An ID failing this is not turned into a node
and stays an ordinary code span.
- *Search input for the `@` menu* — sent to the existing `searchEntities`
endpoint, which is already ACL-scoped. No new query surface.

**Security-Sensitive Operations:**

- **Reference titles are ACL output, not derived data (BUG-R9EHKV).** The
editor renders only what the server's `mentions` map contains and never consults
the graph itself. `collectMentions` routes through `visibility.Reader.Filter`,
so an unreadable entity produces no mention (the span stays plain) and a
redacted display title falls back to the ID with `inaccessible: true`.
- `mentions` is per-principal, so the response is `no-store` and deliberately
not folded into the ETag.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. Round trip → `serializerContract.test.ts` over the corpus, full sweep
under `RELA_FULL_CORPUS=1` in CI.
2. Reference rendering/serialization → `entityRefNode.test.ts`,
`entityRefResolution.test.ts`, plus e2e asserting stored bytes.
3. ACL → `mentions_entity_get_test.go`, including a leak test proven
non-vacuous by substituting `visibility.AllowAllReader`.
4. Bundle boundary → grep the built output for `EasyMDE`.

**Edge Cases:**

- Empty document (placeholder must show, and must not need an inline `style`
because of the app CSP).
- Table normalization at load: ProseMirror's table plugin rewrites column
widths during load, which is indistinguishable from a user edit unless guarded.
Handled by an `armed` flag set once loading settles.
- A reference inserted by the picker cannot be in the load-time mentions map,
so resolution must preserve a title the node already carries.
- ID shapes that pass `looksLikeEntityRef` but are not real entities: render
as the bare ID, never an error.
- `@` inside an email address must not trigger the menu (boundary character
required before the trigger).

**Negative Tests:**

- An unreadable reference must produce no mention and no title.
- A redacted display title must fall back to the ID, never the hidden value.
- Serializing must never emit the title into the markdown.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Silent corruption of stored bodies** (highest). A WYSIWYG editor rewrites
everything it opens. Mitigated by the Phase 0 corpus gate plus `guardWriteBack`,
which refuses to write when it detects drift rather than guessing.
- **Bundle size.** Mitigated by measuring rather than estimating: net −247KB,
because dropping Font Awesome 4.7 more than pays for Milkdown.
- **Scope creep into the app editor.** Mitigated by holding Phase 4 back and
asserting the app-editor bundle still builds on EasyMDE.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/data-entry.md — the editor is user-facing; toolbar and `@`
completion need describing.
- [x] CLAUDE.md (frontend) — the editor's `.md-body` inheritance and the
round-trip guard are conventions new code must not break.
