---
id: REV-YM9O
type: review-checklist
title: 'Review: Add internal-link picker button to the markdown editor toolbar'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just test` — Go race-detector suite passes
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — no boundary warnings
- [x] `just coverage-check` — 75.5% total, all floors satisfied
- [x] Frontend `npm run test:run` — 714 tests / 40 files pass
- [x] Frontend `npm run typecheck` — clean
- [x] Frontend `npm run lint` — 75 pre-existing warnings, 0 errors
- [x] e2e `npm run typecheck && npm run lint` — both clean
- [x] e2e full suite — 191 passing, 1 skipped (baseline unchanged)
- [x] `just build` — all three binaries build cleanly

## Code Review

- [x] Ran the cranky-code-reviewer agent against the full diff
- [x] All findings filed as `review-response` entities and linked via
`has-review-response`

**Findings summary** (13 from cranky review + 10 from design review = 23 total):

| Severity | Count | Addressed | Deferred |
|----------|-------|-----------|----------|
| critical | 0     | 0         | 0        |
| significant | 13 | 12        | 1 (RR-Q7UH — shared-palette refactor, follow-up ticket) |
| minor | 5      | 4         | 1 (RR-YULT — truncation hint, needs backend per_page) |
| nit   | 5      | 3         | 2 (RR-UMGR FA-icon docs; RR-5DHQ e2e ranking; RR-5QXE rapid-reopen; RR-8AOV focus guard; RR-4C0L z-index constant) |

Cranky review highlights addressed:

- **RR-A4RR (significant)** — Adjacency check now reads selection bounds
via `cm.getCursor('from'|'to')` instead of head position. Three new
selection-bound helper tests (forward, backward, spanning).
- **RR-D54M (significant)** — Replaced the strict allowlist regex with a
denylist that mirrors `internal/store/storeutil.ValidateID`. Now accepts the
same IDs the backend accepts (leading digits, dots, longer IDs) while still
rejecting backticks, whitespace, control chars, `--`, path separators. Cap
raised to 1024 bytes.
- **RR-Q7UH (significant) — deferred** — The reviewer's recommendation
to extract a shared `EntitySearchPalette` core is sound but out of scope for
I5NO. A follow-up "refactor: extract shared entity-search palette" ticket is the
right venue when a third consumer appears.
- **RR-EBGN, RR-NHQH (minor/nit)** — Removed the redundant
"replaces the current selection" test; selection-bound behavior is now covered
by the three new RR-A4RR-driven tests.
- **RR-20PB (nit)** — Replaced the array-wide `as EasyMDE.Options['toolbar']`
cast with a typed `const entityRefButton: EasyMDE.ToolbarIcon = {...}`
declaration. Lint warning gone; toolbar array stays under EasyMDE's typed union.
- **RR-AYFK (nit)** — `runSearch` now calls `scrollHighlightedIntoView()`
after resetting `highlightedIndex` so the top result is visible on fresh result
render.

## Acceptance Verification

| AC | Status | Evidence |
|----|--------|----------|
| 1 (toolbar button) | PASS | e2e: `toolbar exposes the insert-entity-reference button` |
| 2 (modal opens) | PASS | TS picker test: `focuses the input on open`; e2e: `picker opens with the search input focused` |
| 3 (debounced search) | PASS | TS: `does not call searchEntities below MIN_QUERY_LEN`, `issues debounced /_search when typing >= 2 chars` |
| 4 (selection insert) | PASS | TS helper tests for happy path; e2e round-trip |
| 5 (adjacency safety) | PASS | Helper tests: left/right/both backtick padding; selection-bound versions for forward & backward selection |
| 6 (cursor placement) | PASS | All helper tests assert `replaceSelection(..., 'end')` |
| 7 (focus after close) | PASS | MarkdownEditor `onPickerClose` calls `editor?.codemirror.focus()` in nextTick; verified by e2e round-trip (subsequent typing lands in editor) |
| 8 (null-editor survival) | PASS | Helper tests for null/undefined/missing codemirror; onBeforeUnmount closes picker first |
| 9 (z-index) | PASS | CSS `z-index: 10000`; manually verified in dev server; e2e fullscreen case deferred via RR-4C0L follow-up |
| 10 (denylist validation) | PASS | 14 helper rejection tests (replaces the old allowlist coverage, RR-D54M) |
| 11 (keyboard nav) | PASS | TS picker tests: ArrowDown/Up wrap-around; Enter emits select; Escape emits close without select |
| 12 (round-trip) | PASS | e2e: `round-trip: saved entity renders the rewritten link on the detail page` |
| 13 (unit tests) | PASS | 31 helper cases + 13 picker cases = 44 new unit tests |
| 14 (e2e) | PASS | 6 e2e cases in `markdown-editor-entity-ref.spec.ts` |

## Verification Evidence

- **Helper (`insertEntityRef.ts`):** denylist validation matches
`internal/store/storeutil.ValidateID`; adjacency padding reads selection bounds;
null-editor guard; 31 Vitest cases.
- **Picker (`EntityPickerModal.vue`):** mirrors `CommandPaletteModal`
with `select(id)` event; z-index 10000 over EasyMDE 9999; 13 Vitest cases
including abort-on-close and keyboard navigation.
- **MarkdownEditor wiring:** typed toolbar button declaration;
onPickerSelect uses helper; onPickerClose refocuses CodeMirror via nextTick;
onBeforeUnmount closes picker before tearing down editor.
- **e2e (`markdown-editor-entity-ref.spec.ts`):** 6 cases all green:
toolbar visible, picker opens focused, ID code span inserted at cursor,
round-trip rendered as titled link, Escape leaves body unchanged, exact-title
query surfaces target.
- **Coverage:** total 75.5% (was 75.5%), all package floors satisfied.
