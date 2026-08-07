---
id: REV-KZPRL5
type: review-checklist
title: 'Review: Enable gosec G705 (XSS), add nosniff to feeds, render help via html/template'
status: done
---

## Automated Checks

- [x] All tests pass — `go test ./internal/dataentry/... ./internal/calfeed/...`
green including `-race`
- [x] Lint clean — `golangci-lint run ./...` 0 issues with G705 enabled
- [x] Coverage maintained — 286 lines of new XSS tests across two files

## Code Review

- [x] Run `/code-review`
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

The notable review outcome is a rejected annotation. The `handlers.go` finding
*looked* like a verified-sanitizer case, but tracing the upstream showed goldmark
running with `html.WithUnsafe()` and no sanitizer in the repo, so annotating it as
sanitized would have recorded a barrier that does not exist. The safety argument
actually rests on input trust, which is a weaker and more fragile property — so
the fix routes the fragment through `html/template` instead, replacing ~15
hand-escaped `Fprintf` calls with contextual auto-escaping.

Both new suites were mutation-tested, so neither is vacuous.

**Follow-up worth its own ticket:** `simpleMarkdownToHTML`'s `WithUnsafe()` is
safe only for operator-authored input. Routing user-authored entity content
through it would be a real stored-XSS bug. The constraint is documented at the
call site; the converter is unchanged here because changing it would alter
rendering behaviour well beyond this PR's scope.

## Acceptance Verification

- [x] Each acceptance criterion tested — see the evidence block in IMPL-VMX574
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- G705 enabled repo-wide — PASS (removed from `gosec.excludes`)
- All three findings resolved — PASS (2 hardening fixes, 1 documented
operator-trusted seam)
- `nosniff` set on both feed formats — PASS (asserted, and mutation-tested by
removing the header)
- Help fragment auto-escaped rather than hand-escaped — PASS (rendered via
`html/template`)
- Feed escaping proven non-corrupting — PASS (JSON round-trip check)
- Description/Help raw-markup status pinned against silent change — PASS

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

**PR:** https://github.com/sourcehaven-bv/rela/pull/1249
