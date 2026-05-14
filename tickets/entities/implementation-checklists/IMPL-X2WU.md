---
id: IMPL-X2WU
type: implementation-checklist
title: 'Implementation: Add internal-link picker button to the markdown editor toolbar'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (via Playwright)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- **Helper unit tests:** `insertEntityRef.test.ts` — 28 cases covering happy
path (3), adjacency padding (5: left, right, both, no-pad space, empty buffer),
allowlist (14: valid + 13 rejection paths including >256 chars, control chars,
leading-digit, slash/dot, non-string), null-editor guard (3).
- **Picker component tests:** `EntityPickerModal.test.ts` — 13 cases:
rendering (4), search debounce + below-MIN_QUERY_LEN (2), selection via click +
Enter + Escape + overlay (4), abort-on-close (2: in-flight signal aborted;
pending debounce cancelled), keyboard nav (1 with wrap-around).
- **Frontend full suite:** `npm run test:run` — 711 passing across 40 files
(was 666; +41 new). Typecheck clean, lint clean (75 pre-existing warnings
unchanged).
- **e2e:** `markdown-editor-entity-ref.spec.ts` — 6 cases all green:
toolbar button visible, picker opens with focused input, ID code span inserted
at cursor, round-trip rendered as titled link on detail page, Escape leaves body
unchanged, exact-title query surfaces target on top.
- **Full e2e suite:** `npx playwright test --project=chromium` — 191 pass,
1 skipped (unchanged baseline).
- **Go suite:** `go test -race ./...` — all packages green, no failures.
- **Go lint:** `just lint` — 0 issues.
- **Build:** `just build` — all three binaries build cleanly.

## Quality

- [x] Code follows project patterns (mirrored CommandPaletteModal for
picker; helper uses the same Pick<> structural typing as other forms components)
- [x] No security issues introduced (allowlist regex on the inserted ID;
no new server-side endpoints)
- [x] No silent failures — helper no-ops are deliberate (null editor /
invalid id); picker search errors surface via `errorMsg` to the UI
- [x] No debug code left behind
