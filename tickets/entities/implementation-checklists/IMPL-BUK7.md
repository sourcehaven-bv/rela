---
id: IMPL-BUK7
type: implementation-checklist
title: 'Implementation: Analyze view shows ID-derived placeholder instead of entity title'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: vitest `mount()` already integration-shaped; no backend changes mean no e2e test required — manual verification covers the full flow)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**Implementation:** Replaced the body of `getEntityTitle()` in
`frontend/src/views/AnalyzeView.vue` with `return issue.title ||
issue.entityId`, removed the stale placeholder comment. No backend changes — the
backend already populates `AnalysisIssue.Title` via `s.Meta.DisplayTitle(...)`
in `internal/dataentry/analyze.go` and serializes it to the wire as `title` via
`APIIssue` in `internal/dataentry/api_v1.go:1272`.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

**Tests added** to `frontend/src/views/AnalyzeView.test.ts` in a new
`describe('AnalyzeView entity title rendering', ...)` block, using the existing
`makeIssue` / `makeResult` fluent builders:

1. *renders the backend-supplied title on the title line and ID below* —
AC 1
2. *falls back to the entityId on the title line when title is empty* —
AC 2
3. *falls back to the entityId on the title line when title is omitted* —
AC 2 (optional `title` path)

The script-error / load-error path (AC 3) is already covered by the existing
test *renders an em-dash when entity cell or type cell is empty* (line 145) —
that path hits the `entity-empty` branch (the outer `v-if="issue.entityId"`), so
`getEntityTitle` is never called. No regression risk.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Started `rela-server` against the in-repo `tickets/` project on `:9999`, opened
`/analyze` in puppeteer, queried `.issue-row .entity-title` / `.issue-row
.entity-id`. Sample of rendered rows:

| Title (rendered) | ID line |
|------------------|---------|
| Planning: Analyze view shows ID-derived placeholder instead of entity title | PLAN-WAM6 |
| Cranky #11: relation-delete error swallowing | RR-W8ZR |
| Architect #9: workspace still imports automation | RR-YR4B |
| Analyze view shows ID-derived placeholder instead of entity title | TKT-JMIS |
| Implementation: Analyze view shows ID-derived placeholder instead of entity title | IMPL-BUK7 |
| Add back button to search results in data entry | FEAT-004 |
| Add back button to search results in data entry | FEAT-007 |

AC 1 confirmed (real titles render, IDs render below). ID Gaps rows continue to
render `.entity-empty` em-dash — no regression. Server stopped after
verification.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Pattern check: function-style helper next to `getEntityTypeLabel` (one line per
call site, easy to extend later). Vue text interpolation `{{ }}` HTML-escapes —
no XSS surface. No new imports, no new collaborators, no new dependencies.

Local checks all green:
- `npm run test:run` — 790 tests pass (3 new + 787 existing).
- `npm run lint` — 0 errors (75 warnings pre-existing in unrelated files).
- `npm run typecheck` — clean.
- `npm run build` + Go `just build` — clean.
