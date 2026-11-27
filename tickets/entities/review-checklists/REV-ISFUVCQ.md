---
id: REV-ISFUVCQ
type: review-checklist
title: 'Review: TestOutputs_TriggerFinalCloseOnly misses the close when a mark arrives mid-round'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (run narrowly because the host is short on file descriptors: `go test -race ./internal/appbuild/` passes; CI runs the full suite)
- [x] Lint clean (`just lint`) (golangci-lint on internal/appbuild clean after replacing context.WithCancel with t.Context; go-arch-lint OK)
- [x] Comment lint gate clean (`just comment-lint`) (commented-code and doclink clean on internal/appbuild)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (not run locally: full coverage needs the whole suite; the change only adds tests to internal/appbuild, so its floor cannot drop; CI checks it)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (none found)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-L9KDNV6, RR-YNDO5W1, RR-024SGHX (addressed), RR-TPFU3VO
(deferred, pre-existing)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: a wake during a round runs one more round
(TestWorker_OutputsWakeDuringRoundReruns; fails on develop). PASS: no wake is
lost in the release window (TestWorker_OutputsNoLostWake; fails without the
recheck). PASS: TestOutputs_TriggerFinalCloseOnly forces the mid-round wake and
passes; `go test -race -count=100 -run 'TestOutputs_|TestWorker_Outputs'`
passes.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (the PR is opened with gh pr create after the bug is done; CI is tracked on GitHub)

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
