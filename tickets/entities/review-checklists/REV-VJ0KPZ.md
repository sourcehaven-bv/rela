---
id: REV-VJ0KPZ
type: review-checklist
title: 'Review: Fuzz sweep files one issue per failing target'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — via `just ci`, exit 0
- [x] Lint clean (`just lint`) — via `just ci`
- [x] ~~Comment lint gate clean~~ (N/A: `just comment-lint` checks Go comments;
  this diff adds no Go code. The Go tree is untouched, so the gate's result is
  unchanged from develop.)
- [x] ~~Coverage maintained~~ (N/A: no Go code changed, so no package's coverage
  moves. `just ci` includes `coverage-check` and passed.)

`just ci` (check + coverage-check + build + docs-check) exited 0 on this branch.

Additionally, actionlint was run manually: clean, with shellcheck 0.11.0
present so the `run:` block is genuinely inspected — verified by injecting an
unquoted expansion into a copy and confirming SC2086 fires.

CI does run CodeQL's `Analyze (actions)` job (via GitHub's default CodeQL
setup, not the checked-in `codeql.yml`), and it passed. That pack covers
workflow security, not shell correctness, so it would not have caught either
critical finding — hence the manual actionlint run.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes — `git diff develop...HEAD
  --stat` is two files: the workflow and TKT-PCLGGL's superseded-decision note

**Review Responses:**

| ID | Severity | Status | Finding |
|---|---|---|---|
| RR-LERUY1 | critical | addressed | Failed dedup query filed a duplicate issue |
| RR-EAW6EL | critical | addressed | Last summary row dropped without trailing newline |
| RR-513JAO | significant | addressed | Unguarded `gh label create` could abort the step |
| RR-EKH2XM | significant | addressed | `error`-kind body pointed at non-existent crash files |
| RR-SWV734 | significant | addressed | Page-limit truncation could hide a match |
| RR-MUW7BA | significant | deferred | Step has no committed test (follow-up) |
| RR-DVGPS1 | minor | deferred | Post-close recurrence not linked to closed issue |
| RR-8Z0JVS | minor | deferred | Dedup key is a human-editable title |

Both critical findings were reproduced against the real step body before
fixing and re-verified after. RR-MUW7BA is deferred rather than addressed
because committing a test harness is a larger change than the fix it guards;
the coverage exists (11 scenarios) but is not yet committed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | Criterion | Status |
|---|---|---|
| 1 | N failing targets file N issues | PASS — 3-row summary files 3 |
| 2 | Recurrence comments, does not duplicate | PASS — comments on #1001 |
| 3 | Same target, different packages does not alias | PASS — 3 distinct issues |
| 4 | Setup error files nothing | PASS — no calls, exit 0 |

Regression suite re-run after the review fixes and again after the final
comment edits: 10/10 green.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked~~ (N/A: `kind=chore`, internal CI
  change with no user-facing surface)
- [x] ~~User-facing documentation updated~~ (N/A: same)
- [x] ~~Docs-checklist marked as done~~ (N/A: same)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Known limitation, stated rather than hidden: this step only executes on a
failing weekly sweep, so its first exercise against the real GitHub API is the
next sweep that finds something. Everything above is verified against a stubbed
`gh`.

Follow-up work identified (not in this ticket): add an actionlint CI job — the
existing CodeQL `actions` analysis covers workflow security, not shell
correctness — and commit the step-body test harness (RR-MUW7BA).

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
