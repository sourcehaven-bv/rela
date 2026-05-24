---
id: REV-7PXR
type: review-checklist
title: 'Review: Analyze view shows ID-derived placeholder instead of entity title'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

**Evidence:**

- Go: `just test` — all packages pass (race detector on). Sample tail:
`internal/store/fsstore` 80.0%, `internal/store/memstore` 96.0%,
`internal/tracer` 87.1%, `internal/validator` 82.4%.
- Frontend: `npm run test:run` — 791 tests pass across 45 files
(4 newly added in `AnalyzeView.test.ts`).
- Go: `just lint` — 0 issues.
- Frontend: `npm run lint` — 0 errors (75 warnings pre-existing in
unrelated files: `stress/**`, etc.).
- Frontend: `npm run typecheck` — clean.
- Coverage: `just coverage-check` — package floor PASS, total 77.0%
(16974/22044), well above the 65% threshold.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] ~~All critical review-responses addressed~~ (N/A: no critical findings)
- [x] ~~All significant review-responses addressed~~ (N/A: no significant findings)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

| ID | Severity | Status | Title |
|----|----------|--------|-------|
| RR-LFXN | minor | addressed | Whitespace-only title leaks through `\|\|` fallback |
| RR-YDA4 | nit | addressed | Third test omits `.entity-id` assertion present in the first two |
| RR-AOPF | nit | deferred | Duplicated `beforeEach` + `mountWith` setup across three describe blocks |

Cranky's verdict: "ship it." No critical/significant findings. The two nits the
reviewer flagged were straightforward, so both got fixed (`.trim()` in the
fallback, symmetric `.entity-id` assertion, +1 vitest case for whitespace
input). The third (test-setup duplication) is pre-existing and deferred to a
future test-quality sweep.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| 1 — entity-linked row shows real title + ID | PASS | Vitest case 1 (line ~280); manual verification table in IMPL-BUK7 (rendered titles for PLAN-WAM6, RR-W8ZR, TKT-JMIS etc.) |
| 2 — empty/omitted/whitespace title falls back to ID | PASS | Vitest cases 2, 3, 4 (lines ~298, 316, 333) |
| 3 — script-error / load-error rule-name rows unchanged | PASS | Existing test "renders an em-dash when entity cell or type cell is empty" (line 145) still passes; that path doesn't call `getEntityTitle` |
| 4 — Frontend unit tests cover | PASS | Four vitest cases added in new `describe` block |
| 5 — Manual verification across check types | PASS | Server run against `tickets/` project; titles render for Properties / Cardinality / Validations / Orphans / Duplicates check types; ID Gaps rows continue to render em-dash |

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing docs documented the placeholder; nothing to update)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A — visible behaviour change with no documented prior
behaviour. No CLI / API / CLAUDE.md changes.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

The stale `// For now, capitalize first letter as title approximation`
+ `// In v1, this comes from the entity properties` comment block was
removed alongside the placeholder body — no remaining TODO in this area.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (Deferred: PR creation is the explicit next user step via `/pr`, outside the `/ticket` auto-flow)
- [x] ~~All CI checks pass~~ (N/A until the PR is opened; all local checks green — `just test`, `just lint`, `just coverage-check`, `npm run test:run`, `npm run lint`, `npm run typecheck`)
- [x] ~~PR URL documented below~~ (N/A until the PR is opened)

**PR:** *(pending — to be created via `/pr` after this checklist is committed)*
