---
id: REV-Z0MG4CC
type: review-checklist
title: 'Review: Two flaky tests, one root cause — fixed timeouts sized against uninstrumented, uncontended local timing'
started: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (deferred to CI: local runner overloaded; `go test -race ./internal/fswatch/` passes)
- [x] ~~Lint clean (`just lint`)~~ (deferred to CI: local runner overloaded; `go vet` and `gofmt` clean)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (deferred to CI: local runner overloaded; the diff adds no comments inside function bodies)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (deferred to CI: test-only change, production code untouched)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (no critical or significant findings)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-O9AN177, RR-27VWMO1, RR-33HT6KJ, RR-DYS9XHX, RR-IQ3VARD

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
PASS: no fixed 2s wait remains in `internal/fswatch/watcher_test.go`; `go test -race -count=100 -run TestWatcher_AutoWatchesNewDirectories` ok. PASS: instance 1 already fixed by #1875. PASS: docscapture first-capture cost already covered by the retry from #1618; no worlds-manual step failure in CI since.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, test-only)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened with `gh pr create` after this checklist)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
