---
id: REV-ES9I5FA
type: review-checklist
title: 'Review: Flows M1j: db load copies unfinished runs into sqlite'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

## Automated Checks

- [x] All tests pass (`just test`): targeted tests pass on default, postgres (-race) and sqlite tags; full `go test -race` passed except sqlitestore, which timed out under load and passes alone
- [x] Lint clean (`just lint`); `just arch-lint` clean too
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (skipped: coverage thresholds were not run locally; CI runs the coverage gate on the PR)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** None for this item (the findings are linked to the related
tickets).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: db load into an existing sqlite database keeps
unfinished runs (TestMarkdownData_FlowRunsRoundTrip).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-postgres-backend, regenerated docs/postgres-backend.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-2ZS63DW

## Final Checks

- [x] Commit message explains the why, not just what (commit f2f2dac83; minimal message by convention)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred: the PR is opened after the ticket is done, per TKT-UFV01M; not part of this bookkeeping)
