---
id: REV-KGV0
type: review-checklist
title: 'Review: Resolve entity-ID code spans to titled links in data-entry views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just test` passes — all packages green (Go race-detector on; `internal/dataentry` 2.6s, `internal/lua` 6.7s)
- [x] `just lint` passes — 0 issues
- [x] `just arch-lint` passes — no boundary warnings
- [x] `just coverage-check` passes — total 75.5%, all package floors satisfied
- [x] Frontend `npm run test:run` passes — 666 tests / 38 files
- [x] Frontend `npm run typecheck` passes — clean
- [x] Frontend `npm run lint` passes — no new errors (pre-existing warnings unchanged)
- [x] e2e `npm run typecheck && npm run lint` passes — both clean
- [x] `just build` produces all three binaries

## Code Review

- [x] Ran the cranky-code-reviewer agent against the full diff
- [x] All findings filed as `review-response` entities and linked via
`has-review-response`

**Findings summary** (19 total):

| Severity | Count | Status |
|----------|-------|--------|
| critical | 1     | 1 addressed |
| significant | 4 | 4 addressed |
| minor | 7      | 7 addressed |
| nit   | 7      | 6 addressed, 1 deferred (RR-A9DU — shared scanner refactor, out of scope) |

- **RR-OKQD (critical)** — `viewContentBlobs` missed entity-card content;
fixed by walking `sec.Entities` + `sec.Groups[].Entities`.
- **RR-58F4** — switched `Mention.Title` from `ent.Title()` to
`meta.DisplayTitle(...)` so entities with non-title primary properties
(concept's `name`, etc.) resolve correctly.
- **RR-47N9** — partial-lock entities no longer flip into the lock
affordance: new `lockedReasonFor` only checks the display-property field or the
`InaccessibleFieldContent` sentinel.
- **RR-07K2** — `mentionsMarkdown` now enables GFM/Table/Strikethrough/
TaskList extensions so GFM table cells produce CodeSpans the scanner expects.
- **RR-ZK8N** — non-NotFound store errors logged via `slog.WarnContext`;
context cancellation honored.
- All minor and nit findings either addressed or explicitly deferred
with a documented reason. No open critical/significant findings remain.

## Acceptance Verification

| AC | Status | Evidence |
|----|--------|----------|
| 1 (known-ID → titled link) | PASS | Go: `TestCollectMentions/known short-ID code span resolves`; TS: `rewrites a known-ID code span into a titled link`; manual `curl /_views/ticket/TKT-77JD4` returns the expected mention |
| 2 (manual-ID concept) | PASS | Go: `manual-ID concept resolves via DisplayTitle (name)`; TS: `rewrites a manual-ID code span into a titled link` |
| 3 (unknown-ID unchanged) | PASS | Go: `unknown ID is dropped`; TS: `leaves unknown-ID code spans as <code>`; e2e: `unknown-ID code spans remain as <code> (no link)` |
| 4 (multi-token unchanged) | PASS | Go: `multi-token code span is not collected`; TS: `does not rewrite multi-token code spans (exact-match only)` |
| 5 (code block / link text untouched) | PASS | Go: `ID inside fenced code block`, `ID inside indented code block`, `link text containing an ID is not a code span`; TS: `does not rewrite IDs inside fenced code blocks`, `does not rewrite IDs inside existing link text`; e2e: `IDs inside fenced code blocks are not linkified` |
| 6 (DOMPurify-safe) | PASS | TS: `renders dangerous titles as escaped text without breaking the link` (DOMParser-based), `produces only same-origin entity hrefs`, `cannot inject script via a malicious inaccessible-reason tooltip` |
| 7 (self-reference) | PASS | Go: `TestCollectMentions_SelfReference`; TS: `rewrites a self-reference like any other entity link` |
| 8 (inaccessible target) | PASS | Go: `inaccessible target carries inaccessible + reason`, `partially-locked entity keeps its readable title and is NOT inaccessible`; TS: `renders inaccessible targets with a lock affordance and tooltip`, `keeps the readable title alongside the lock when one is supplied`, `falls back to "inaccessible" tooltip when reason is missing` |
| 9 (manual end-to-end) | PASS | Built `rela-server` against `tickets/`, hit `GET /api/v1/_views/ticket/TKT-77JD4`; response carried the expected `mentions` entry |
| 10 (Playwright e2e) | PASS | `e2e/tests/entity-refs.spec.ts` with two happy-path cases (link rendered, click navigates) and two negative cases (unknown ID, fenced block) |

## Verification Evidence

- See `IMPL-L5OQ` for the implementation-phase verification trail; the
code-review cycle on top added the GFM extension wiring, the `meta.DisplayTitle`
switch, the locked-property refinement, error logging, and additional test
coverage. All of those have unit-test evidence above.
- Coverage floor for `internal/dataentry` unchanged; new code is
well-covered by the table-driven tests in `mentions_test.go` plus the
view-response tests in `api_v1_test.go`.
