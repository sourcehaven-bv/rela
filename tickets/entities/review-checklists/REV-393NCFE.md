---
id: REV-393NCFE
type: review-checklist
title: 'Review: Structured questions in realtime channels'
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): after fixing the configedit allowlist for `cards:`
- [x] Lint clean (`just lint`), plus arch-lint and plimsoll
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): direct `batch.go` tests lift `internal/channels` to 89.7%

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
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** code review RR-SV2NT98, RR-8DNAYO7, RR-SN7X4IR
(significant, addressed); RR-BR9NHQC, RR-JG7MOSD, RR-SN5B1XN (minor, addressed);
RR-JH8JPTD (minor, deferred); RR-GV6ADZ7 (nit, deferred); RR-6Y932SW (nit,
wont-fix). Design-review responses are linked on the ticket.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Object list properties and choices from an object list: PASS (realtimecfg and channelValues tests).
- Several questions in one message flow (one message per question, posted as a batch): PASS (store conformance, realtime batch tests, HTTP/MCP/CLI tests).
- The session waits until every question is answered (`session.open_questions`): PASS (TestPostBatch_QuestionsMoveOnLastAnswer, e2e asked test).
- `display: cards` hint in data-entry.yaml: PASS (dataentryconfig tests, RlChannelCards stories).
- Reply quotes render or strip markdown: PASS (rela-components tests).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-VRLWHFZ

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

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
