---
id: REV-KGINCN
type: review-checklist
title: 'Review: Enable gosec G706 (log injection), scope-exclude dataentry with a tested invariant'
status: done
---

## Automated Checks

- [x] All tests pass — `go test ./internal/dataentry/...` green
- [x] Lint clean — `golangci-lint run ./...` 0 issues with G706 enabled
- [x] Coverage maintained — 141 lines of new invariant tests; no production code
changed

## Code Review

- [x] Run `/code-review`
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

The design question here is exclusion-versus-annotation, and the reasoning holds
up: 20 `//nosec` comments in a codebase with zero prior uses would establish a
suppression pattern at scale for a provable non-issue. The path exclusion is the
better trade **only because** the invariant it depends on is now tested rather
than conventional — `sloglint` at default settings does not forbid a dynamic
message, so nothing previously enforced it.

Two verification steps deserve credit for being negatives rather than positives:
the `internal/mcp` canary proved the rule is still enforced elsewhere, and the
mutation test proved the AST check can fail.

**Known limitation, accepted:** the exclusion is correct for `TextHandler`, which
every current entry point installs, but `TestSlogTextHandlerEscapesNewlines` tests
that handler directly rather than the wiring — swapping in a non-escaping custom
handler later would not be caught. Tying the assertion to the actually-installed
handler would be a stronger follow-up.

## Acceptance Verification

- [x] Each acceptance criterion tested — see the evidence block in IMPL-4LNI05
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- G706 enabled repo-wide — PASS (removed from `gosec.excludes`)
- All 20 findings resolved — PASS (one path-scoped exclusion, no per-line
suppressions)
- Rule proven still enforcing outside the excluded path — PASS (`internal/mcp`
canary fired, then removed)
- Constant-message invariant enforced, not assumed — PASS
(`TestSlogMessagesAreConstant`, mutation-tested)
- Handler escaping guarantee pinned — PASS
(`TestSlogTextHandlerEscapesNewlines`, verified empirically)
- No `//nosec` pattern introduced — PASS (still zero in the codebase)

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

**PR:** https://github.com/sourcehaven-bv/rela/pull/1251
