---
id: REV-0TZIF0O
type: review-checklist
title: 'Review: Failed flow runs hide their error and blocked steps give no field feedback'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/flow... ./internal/appbuild/... ./internal/dataentry/... ./internal/writerule/... ./internal/metamodel/...`, vitest on the flows and forms tests, vue-tsc)
- [x] Lint clean (golangci-lint on the changed packages, `just arch-lint`, eslint; the full `just lint` could not load packages on the loaded host and runs in CI)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained~~ (N/A locally: new code has unit tests; CI checks the floors)

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

- [x] Code review done as a critical pass over the whole diff; findings recorded as review-responses
- [x] All critical review-responses addressed (RR-JEKORKF)
- [x] All significant review-responses addressed (RR-HH44NGN, RR-IG9MQ20)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-JEKORKF, RR-HH44NGN, RR-IG9MQ20, RR-O7F0NZL, RR-XNQYTEH, RR-Z41KQIK, RR-I6NT67V, RR-0P1EN9J, RR-4UM9N28, RR-VXEJS3F, RR-SEC1TTL, RR-SEC2JOB

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Job error for managers only: PASS (TestFlowJob_FailedJobForManagers, TestRedact)
- Retrying line: PASS (FlowRunCard tests)
- Failed runs on Flow tasks and the failed filter: PASS (TestFlowJob_FailedJobForManagers, FlowLists tests)
- Required markers, rule messages and pattern warnings at fields, blocked notice: PASS (TestFlowControl_CompleteStepFieldFeedback, TestCheck_Fields, FlowRunCard and FieldShell tests)
- failed_message and the friendlier default: PASS (TestParse_TitleAndFailedMessage, TestFlowJob_FailedJobForManagers, FlowRunCard tests)
- Run titles: PASS (TestRunTitle, TestFlowControl_CompleteStepFieldFeedback)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/flows.md and GUIDE-flows)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there

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
