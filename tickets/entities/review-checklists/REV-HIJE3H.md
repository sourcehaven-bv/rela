---
id: REV-HIJE3H
type: review-checklist
title: 'Review: Enable gosec G703 (path traversal) and fix clone path containment'
status: done
---

## Automated Checks

- [x] All tests pass — the four affected packages' tests green
- [x] Lint clean — `golangci-lint run ./...` 0 issues with G703 enabled
- [x] Coverage maintained — 80 lines of regression tests added in
`internal/git/clone_test.go` alongside the fix

## Code Review

- [x] Run `/code-review`
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

The fix is defense-in-depth by construction: containment lives in `Clone` rather
than its single caller, so the guarantee does not depend on call-site discipline.
gosec's taint analysis does not recognise `containedPath` as a sanitizer, so the
config write still needs a `#nosec` — but it now names a barrier that genuinely
exists rather than asserting one that does not.

Adjacent and deliberately out of scope: the desktop clone flow stores a GitHub
OAuth token at rest in `.git/credentials`. Worth its own ticket.

## Acceptance Verification

- [x] Each acceptance criterion tested — see the evidence block in IMPL-V0XRBA
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- G703 enabled repo-wide — PASS (removed from `gosec.excludes`)
- Traversal in `ExtractRepoName` / `IsValidRepoURL` fixed — PASS (verified
pre-fix and post-fix on the same input)
- Clone cannot escape the operator-chosen directory — PASS (`containedPath` in
`Clone`, regression-tested)
- Credential write no longer reachable outside the base dir — PASS (follows from
the containment boundary)
- Remaining two findings annotated with narrow per-line reasons — PASS

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

**PR:** https://github.com/sourcehaven-bv/rela/pull/1247
