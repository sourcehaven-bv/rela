---
id: PLAN-26GJ
type: planning-checklist
title: 'Planning: Add internal-link picker button to the markdown editor toolbar'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- A new custom toolbar button on the EasyMDE instance in
`frontend/src/components/forms/MarkdownEditor.vue` that opens an entity picker.
- A picker modal scoped to the project, searching via the existing
`/api/v1/_search` endpoint (reusing `searchEntities` from `api/entities.ts`).
- Inserting `` `<id>` `` at the cursor position (or replacing the current
selection) on selection. Editor focus restored; cursor lands after the inserted
span.
- Keyboard accessibility: input focused on open, Arrow keys to navigate
results, Enter to insert, Esc to dismiss.
- Unit tests for the insertion helper and the picker component, plus a
Playwright e2e of the happy path.

Out of scope:

- A new server-side endpoint — `/_search  ` already does what we need.
- Auto-completion as the user types in the editor (no inline suggestion
popup; this is the deliberate-insertion path triggered by the toolbar button or
its keyboard shortcut).
- Wiki-style `[[...]]  ` syntax (IDEA-011).
- Replacing the existing `link  ` button (we keep it — they cover different
use cases: external URL vs. internal entity reference).

**Acceptance Criteria:**

1. **Toolbar button present.** EasyMDE's toolbar carries a new icon (with
`title  ` tooltip "Insert entity reference"). Placed AFTER the existing inline
group (`link `, `code `, `quote `) with a `'|' ` separator preceding it, so the
toolbar's visual grouping stays intact (RR-91NT).
2. **Modal opens.** Clicking the button opens the entity picker modal
focused on its search input.
3. **Search works.** Typing two or more characters issues a debounced
`/_search?q=<query>  ` and renders up to 50 results sorted by relevance
(server-side ordering preserved). Empty/short queries clear the list.
4. **Selection inserts a code span.** Clicking a result OR highlighting +
pressing Enter inserts `` `<id>` `` at the cursor position. If the editor has a
non-empty selection at the moment the modal opens, the selection is replaced.
5. **Adjacency safety.** If the cursor sits immediately adjacent to existing
backticks, the inserted text is padded with a single space on the adjacent
side(s) so the new code span parses as its own inline token (RR-NKV5). Tests
cover: cursor right before `` ` ``, right after `` ` ``, between two `` ` ``
(neither side gets padded twice).
6. **Cursor placement.** After insertion the editor cursor lands
immediately after the closing backtick (plus the padding space, if any) so the
user can keep typing.
7. **Editor focus after close.** After the picker closes — whether by
selection, click-outside, or Esc — the CodeMirror textarea is focused, not the
toolbar button (RR-SKX3). The parent calls `editor.codemirror.focus() ` in
`nextTick ` after the modal's own focus-restore runs.
8. **Picker survives no editor.** When the picker emits `select ` but the
parent's `editor ` reference is null (e.g. MarkdownEditor unmounted mid-pick),
nothing happens — no throw, no console error (RR-032O). MarkdownEditor closes
the picker in `onBeforeUnmount ` as belt-and-braces.
9. **z-index above fullscreen.** With EasyMDE in fullscreen mode (z-index
9999), the picker overlay still renders above the editor. Picker overlay sets
`z-index: 10000 ` explicitly (RR-WMG2). Verified by an e2e case that toggles
fullscreen before opening the picker.
10. **Allowlist ID validation.** `insertEntityRef(editor, id) ` validates `id `
matches `/^[A-Za-z][A-Za-z0-9_-]{0,255}$/ ` (RR-O620): rejects empty, rejects
anything with backticks/whitespace/control chars, caps at 256 chars. The picker
itself never produces an invalid ID — the helper is the defensive boundary.
11. **Keyboard nav.** Arrow up/down navigates results; Enter inserts;
Escape closes the modal without inserting; focus returns to the editor.
12. **Round-trip.** The inserted code span renders as a titled link via
TKT-747O's resolver when the entity is saved and the detail page is loaded.
13. **Unit tests.** A Vitest suite covers: helper (empty/replace/adjacent
backticks/allowlist rejection/null editor), picker (debounce, click-to-insert,
keyboard-driven insert, escape-without-insert, in-flight abort on close).
14. **e2e test.** A Playwright spec opens a form, opens the picker, types
to find a seed entity, selects it, asserts the textarea contains `` `<id>` ``,
submits the form, and asserts the detail page renders the resulting link.
Additional cases: (a) pasting an exact ID surfaces the matching entity as a top
result (RR-Z9C1); (b) opening the picker with EasyMDE in fullscreen still shows
the overlay above the editor (RR-WMG2).

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **EasyMDE custom toolbar API** — `easymde.d.ts  ` defines `ToolbarIcon  `
with `action: string | ((editor: EasyMDE) => void)  ` (line 145). We pass a
function that opens the picker; the function receives the EasyMDE instance so it
can call `editor.codemirror.replaceSelection(text, "end")  ` after the user
picks.
- **CommandPaletteModal** — `frontend/src/components/ui/CommandPaletteModal.vue  `
is essentially the picker we need: it already handles debounced `/_search  `,
abortable in-flight cancellation, ARIA `activedescendant  ` listbox semantics,
focus restoration, escape handling, modal stack registration. Its only
difference is the **action**: it navigates via `router.push  `. We refactor that
part into an action callback so the same component can be reused with a
different "what to do with the selection" prop, OR (simpler) create a new
sibling component that shares the underlying picker logic. Plan B (sibling) is
cheaper for this ticket; Plan A is the right long-term move but out of scope.
- **`searchEntities  `** in `frontend/src/api/entities.ts  ` is the API
function we'll call — already takes a `signal  ` for abort.
- **Modal stack composable** — `useModalStack(open)  ` in
`frontend/src/composables/modalStack.ts  ` is how every modal currently stands
down list-shortcut handlers while open. We reuse it.
- **Picker patterns** — `RelationPicker.vue  ` is a different shape
(inline cards/list, not a modal) so it's not a fit for "from inside the editor,
pop a search."

**Decision: shared picker component or sibling?**

Two options:

| Option | Pros | Cons |
|---|---|---|
| A: refactor `CommandPaletteModal  ` to take an action callback | Long-term DRY; one focus-trap, one ARIA impl | Touches a working component used elsewhere; risk of regression in the Cmd+K palette; bigger PR |
| B: new `EntityPickerModal.vue  ` sibling | Self-contained, easy to test, no regression risk in Cmd+K | Some duplicated logic (debounce, abort, key handling) |

**Choice: B (sibling).** Faster ship, no regression risk in the Cmd+K palette.
Note in IMPL checklist that a future refactor is warranted once a third picker
consumer appears.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **New picker component** `frontend/src/components/forms/EntityPickerModal.vue  `
modeled on `CommandPaletteModal.vue  `:
   - Props: `open: boolean  `. Emits `close  ` and a `select  ` event carrying
**only the entity ID as a string** (RR-L5UX). Narrow contract — the parent
decides what to do with the ID; consumers needing type/title can re-query.
   - Internals copy the search-debounce, abort, results-list, ARIA
`activedescendant  `, modal-stack patterns from the command palette — but the
Enter/click handler emits `select(id)  ` rather than calling `router.push  `.
   - **Abort on close (RR-S7I8):** the `open ` watcher's `open→closed `
branch calls `cancelInflight() ` (debounce timer + AbortController) before
emitting any final state. The pattern is copied verbatim from
CommandPaletteModal line 126; we keep it in lock-step. Unit test asserts the
inflight controller is aborted when `open ` transitions to false.
   - Uses `searchEntities(query, undefined, signal)  ` (no type filter for
v1; "search across all entity types" matches the command palette's UX).
   - **z-index (RR-WMG2):** overlay has `z-index: 10000 ` to land above
EasyMDE's `9999 ` fullscreen layer. CSS comment documents the +1 above the
EasyMDE constant.
2. **MarkdownEditor wiring** — `MarkdownEditor.vue  `:
   - Add a `pickerOpen  ` ref and an `<EntityPickerModal>  ` to the template.
   - Add a custom toolbar entry to the EasyMDE toolbar array:
     ```ts
     {
       name: 'entity-ref',
       action: () => { pickerOpen.value = true },
       className: 'fa fa-at',  // FontAwesome's @ icon is the cleanest
                               // "mention/reference" affordance
       title: 'Insert entity reference',
     }
     ```
Placed AFTER the inline group with its own `'|' ` separator (RR-91NT): `...
'link', 'code', 'quote', '|', <entity-ref icon>, '|', 'preview', ... `.
   - On `@select(id)  ` from the picker:
     - Guard on `editor != null ` (RR-032O); no-op if the editor was torn
down while the modal was open.
     - Call `insertEntityRef(editor, id) ` (see §3) to do the actual
insertion with adjacency-aware padding.
     - Close the modal (sets `pickerOpen.value = false `), then on the next
tick call `editor.codemirror.focus() ` to restore the editor's text-input focus
rather than the toolbar button (RR-SKX3). The `nextTick ` is required because
the modal's own `previouslyFocused ` restore runs first; we override after.
   - Hide the modal on `@close  ` (same `nextTick `-then-focus path so
dismissing without selecting also lands cursor in the editor).
   - **Unmount safety (RR-032O):** `onBeforeUnmount ` sets `pickerOpen.value
= false ` before nulling `editor `. The modal teardown then aborts any in-flight
fetch (its own watcher cleanup) and we never emit a `select ` against a null
editor.
3. **Insertion helper** — `insertEntityRef(editor, id): void ` in
`frontend/src/components/forms/insertEntityRef.ts `:
   - **Allowlist validation (RR-O620):** match `/^[A-Za-z][A-Za-z0-9_-]{0,255}$/ `.
Reject anything else as a no-op (returns without calling the editor). The regex
covers both short IDs (`TKT-LXYHQ `) and manual IDs (`data-entry-ui `) while
excluding backticks, newlines, whitespace, and anything over 256 chars.
   - **Null-editor guard (RR-032O):** if `editor ` is null/falsy, no-op.
   - **Adjacency-aware insertion (RR-NKV5):** read the character to the
left and right of the current cursor (or selection bounds). If the adjacent char
is ` ` `, pad the inserted text on that side with a single space. Resulting
text: ``<leftPad>`<id>`<rightPad> ``. Then
`editor.codemirror.replaceSelection(text, "end") ` so the cursor lands
immediately after the right padding (or the closing backtick when no pad was
needed). Tested with mocked editor returning crafted cursor positions.
4. **No new sanitization.** The output is plain backticked text;
`renderMarkdown  ` already handles it via TKT-747O.

**Files to modify:**

- New: `frontend/src/components/forms/EntityPickerModal.vue  `
- New: `frontend/src/components/forms/EntityPickerModal.test.ts  `
- New: `frontend/src/components/forms/insertEntityRef.ts  ` (helper +
small unit test in `insertEntityRef.test.ts  `)
- Edit: `frontend/src/components/forms/MarkdownEditor.vue  ` — add toolbar
button, picker instance, insertion wiring
- No MarkdownEditor-level Vitest test (RR-X7Q2): CodeMirror v5 measures DOM
geometry at init and behaves erratically in JSDOM. The helper carries the
testable logic; the Playwright e2e covers integration. A future ticket can
revisit when there is a clear way to mock EasyMDE wholesale.
- New: `e2e/tests/markdown-editor-entity-ref.spec.ts  ` — Playwright happy
path
- Edit: `e2e/pages/forms.page.ts  ` (or equivalent) — picker helpers if
needed

**Alternatives considered (rejected):**

- **Inline autocomplete on `  ` `** — much richer, but out of scope and
considerably more design work (CodeMirror hint popups, partial-token matching,
dismiss semantics). Park for a follow-up.
- **Insert `[Title](/entity/<type>/<id>)  ` directly** — explicit URL in
source, but breaks the symmetry with the Lua side and the renderer; if the
entity title changes the markdown source rots. The backticked-ID shape lets the
renderer always show the current title.
- **Refactor CommandPaletteModal** — see Research §; deferred.

**Dependencies:** None new. EasyMDE custom-toolbar API is already typed;
`searchEntities  ` is already in the API layer.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Search query**: user-typed; passed to `/_search?q=...  `. The endpoint
already exists and is exercised by the command palette; no new attack surface.
- **Inserted ID** (the only value that lands in document content): runs
through the **allowlist** regex `/^[A-Za-z][A-Za-z0-9_-]{0,255}$/ ` in the
helper (RR-O620). Reject everything else as a no-op. The picker only ever emits
server-supplied IDs so in practice the regex never trips, but the helper is the
defensive boundary for future callers (Lua actions, scripts). The 256-char cap
prevents an absurdly long ID from blowing up the editor or producing a code span
that confuses goldmark.
- **No script paths**: this is a pure editor-side feature. No file I/O,
no credentials, no auth surface.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Layer |
|----|------|-------|
| 1 (toolbar) | e2e: button visible with the right title | e2e (button selector by title) |
| 2 (modal opens) | Picker test: opening sets focus on the search input | TS (EntityPickerModal) |
| 3 (search) | Picker test: typing issues debounced `searchEntities `; results render | TS (mock fetch) |
| 4 (insert) | Helper test: `replaceSelection ` invoked with `` `id` `` and `"end" `; non-empty selection replaced | TS (mock editor) |
| 5 (adjacency) | Helper tests: cursor right-before/right-after/between backticks; padding correctly added on each side | TS (mock editor) |
| 6 (cursor) | Helper test: post-insert cursor lands after the closing backtick (or padding space) | TS |
| 7 (focus) | Picker test: closing the modal triggers parent focus restore via `editor.codemirror.focus() ` (verified by spy) | TS |
| 8 (null editor) | Helper test: `insertEntityRef(null, id) ` is a no-op | TS |
| 9 (z-index) | e2e: toggle EasyMDE fullscreen, open picker, assert the overlay is above `.editor-toolbar.fullscreen ` | e2e |
| 10 (allowlist) | Helper tests: valid IDs pass; empty, whitespace, backtick, newline, >256 chars all return without calling the editor | TS |
| 11 (keyboard) | Picker test: ArrowDown highlights next, Enter emits select, Escape closes without emitting | TS |
| 12 (round-trip) | e2e happy path: insert via picker, save form, navigate to detail, assert rendered `<a> ` | e2e |
| RR-S7I8 | Picker test: closing the modal aborts the in-flight `AbortController ` | TS |
| RR-Z9C1 | e2e: typing an exact known ID surfaces it as a top result | e2e |

**Edge Cases:**

- Modal opened while editor has a multi-line selection → `replaceSelection  `
replaces the whole selection cleanly (CodeMirror semantic).
- Picker opened, no result chosen, Escape pressed → no insertion, focus
back to editor.
- Network error from `/_search  ` → error message shown in modal; no
insertion; user can dismiss.
- Two pickers attempted concurrently (rapid double-click on button) →
second click is a no-op (open is already true).
- Editor in fullscreen mode → modal renders on top (z-index already
9999 via the existing fullscreen CSS; verified in browser).

**Negative Tests:**

- `insertEntityRef(editor, '')  ` → no-op; assert no call to
`replaceSelection  `.
- `insertEntityRef(editor, 'has  `backtick')` → no-op (defensive).
- Search input below MIN_QUERY_LEN → no API call, empty list rendered.

**Integration approach:**

- TS: Vitest with JSDOM. Mock the EasyMDE instance via a thin shim;
CodeMirror itself isn't initialized in tests (we never assert real CodeMirror
DOM, only that our wiring calls the right methods).
- e2e: build a `markdown-editor-entity-ref.spec.ts  ` that seeds two
entities (one with a clean ID, target of the picker), opens a feature edit form,
opens the picker via the toolbar button, types to filter, selects with Enter,
asserts the textarea contains the expected code span, saves the form, navigates
to detail, asserts the rendered link.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **EasyMDE FontAwesome dependency.** EasyMDE bundles FontAwesome v4 for
toolbar icons. Verified that the existing toolbar buttons already use FA classes
(`fa-link  `, `fa-code  `, etc.) — `fa-at  ` is shipped in the same set so no
new asset.
- **CodeMirror v5 quirks.** CodeMirror 5's `replaceSelection(text,
"end")  ` is documented (the EasyMDE-bundled CodeMirror is v5). The helper is
small enough that a unit test against a thin mock pins down the contract.
- **Selection-vs-cursor wording.** If `replaceSelection  ` is called when
no selection is active, it inserts at the cursor (CodeMirror semantic). Verified
by reading CodeMirror docs; behavior is correct.
- **Picker UX drift from Cmd+K.** Since we duplicate logic, the two
pickers can drift. Mitigation: keep the picker minimal and document in comments
that a future refactor should merge them.

**Effort:** `s  ` (matches ticket's recorded effort).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — this is a small UI affordance discoverable from the toolbar.
No CLI changes, no API surface, no internal architecture changes. Existing
data-entry guide already covers the markdown editor at a high level; the new
button is self-documenting via its tooltip.

## Design Review

- [x] Run `/design-review  ` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

10 review-responses filed via `/design-review `:

| ID | Severity | Title | Resolution |
|----|----------|-------|------------|
| RR-032O | significant | Editor unmount during open modal | Addressed: null-editor guard in helper; `pickerOpen=false ` in `onBeforeUnmount `; AC 8 + unit test |
| RR-NKV5 | significant | Adjacent backtick corruption | Addressed: adjacency-aware padding in helper; AC 5 + tests for 3 cursor positions |
| RR-O620 | significant | Blocklist → allowlist validation | Addressed: explicit regex `[A-Za-z][A-Za-z0-9_-]{0,255} ` in helper; AC 10 + tests for all rejection paths |
| RR-WMG2 | significant | z-index above fullscreen | Addressed: explicit `z-index: 10000 ` on overlay; AC 9 + e2e fullscreen case |
| RR-S7I8 | minor | Abort-on-close race | Addressed: open-watcher closes `cancelInflight() ` explicitly; covered by picker unit test |
| RR-SKX3 | minor | Focus restoration ordering | Addressed: parent calls `editor.codemirror.focus() ` in `nextTick ` after modal close, overriding the modal's own `previouslyFocused ` restore; AC 7 |
| RR-Z9C1 | minor | Exact-ID ranking | Addressed: e2e asserts exact-ID match surfaces as top result; no code change needed (Bleve's relevance handles short queries adequately — tested, not promised) |
| RR-91NT | nit | Toolbar grouping | Addressed: button placed AFTER the inline group with its own `'|' ` separator; AC 1 |
| RR-X7Q2 | nit | MarkdownEditor unit test | Addressed: dropped — helper unit tests + Playwright e2e cover the behavior; JSDOM/CodeMirror would burn a day |
| RR-L5UX | nit | Over-specified select event | Addressed: picker emits `select: [id: string] ` only — narrow contract |
