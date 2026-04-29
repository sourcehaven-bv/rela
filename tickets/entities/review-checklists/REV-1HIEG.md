---
id: REV-1HIEG
type: review-checklist
title: 'Review: Remove dead htmx templates and vendor-js justfile target after Vue migration'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

Re-run after addressing review findings: all green. `just build` and `just test`
clean; `just lint` reports `0 issues.`; `just coverage-check` PASS at 74.2%
total. `arch-lint` has only pre-existing `.ignored/` notices (verified by `git
stash` baseline). `go test -tags e2e -run NONEXISTENT ./internal/dataentry/`
compiles clean after the form-specific tests were removed.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

| ID | Severity | Status |
|----|----------|--------|
| RR-DSIWT | critical | addressed (deleted dead `ResolvedField` struct) |
| RR-H5E9A | significant | addressed (deleted form-specific chromedp tests; kept Lua doc test) |
| RR-7U8DD | significant | addressed (resolved by deleting struct in RR-DSIWT) |
| RR-Y0KEU | minor | addressed (concept description acknowledges legacy /api/* + second SSE) |
| RR-RB0EJ | minor | addressed (component list replaced with frontend/src/components tree pointer) |
| RR-XOT97 | minor | wont-fix (reviewer's analysis was incorrect; htmltemplate.HTMLEscapeString is actively used at handlers.go:147-148, 164-166) |

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| AC-1 (templates dir gone) | PASS | `test ! -d internal/dataentry/templates` |
| AC-2 (vendor-js gone) | PASS | `grep -n vendor-js justfile` empty |
| AC-3 (no htmx refs in live code) | PASS | `grep -rn 'htmx\|hx-' internal/ frontend/src/ justfile cmd/ \| grep -v _test.go` empty |
| AC-4 (concept updated) | PASS | tickets/entities/concepts/data-entry-ui.md describes Vue/Pinia/Vite + accurate route surface |
| AC-5 (build/test/lint/coverage) | PASS | re-run after fixes — all green |
| AC-6 (server still serves) | PASS | manual smoke test — `/`, `/static/favicon.svg`, `/static/v2/favicon.svg` all 200; project loaded with 14 entities, 32 relations |

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: kind=chore, not enhancement; no user-facing docs)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing change. The `data-entry-ui` concept update is internal docs and is in scope of this ticket itself)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** None — internal-only chore.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- to be filled after running /pr -->
