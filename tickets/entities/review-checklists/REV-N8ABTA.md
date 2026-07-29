---
id: REV-N8ABTA
type: review-checklist
title: 'Review: Enable gosec G702 (command injection) with reviewed exec-site annotations'
status: done
---

## Automated Checks

- [x] All tests pass — full `go test ./...` green
- [x] Lint clean — `golangci-lint run ./...` 0 issues with G702 enabled
- [x] Coverage maintained — package floors unaffected; `commands_test.go` updated
alongside the change

## Code Review

- [x] Run `/code-review`
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

Suppression style reviewed: 10 of the 11 findings are annotated rather than
changed, so the review focus was whether each annotation names a boundary that
actually exists. The `commands.go` script site is gated by `authorizeCommand`
and selected by config *key*, not by request text. The launcher sites take argv
arrays with constant program names, so the only live shape was argument
injection, and the two `open(1)` calls now carry `--` to close that structurally.

## Acceptance Verification

- [x] Each acceptance criterion tested — see the evidence block in IMPL-6PIFBQ
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- G702 enabled repo-wide — PASS (removed from `gosec.excludes`)
- All 11 findings resolved — PASS (1 fixed, 10 annotated with per-line reasons)
- Rule proven enforcing, not silently excluded — PASS (remove-`#nosec` probe
makes the finding reappear)
- No blanket suppressions introduced — PASS (every annotation is per-line and
names its trust boundary)
- `renderCommand` safe against a forgetful future caller — PASS (validation moved
to the point of use)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked~~ (N/A: internal security-lint work,
`kind=refactor`, no user-facing behaviour change)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — full matrix green; the `Rela Tickets` gate is resolved
by this done-transition
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1248
