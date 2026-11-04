---
id: REV-42HJLOS
type: review-checklist
title: 'Review: Group quiet channel messages across session notices'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): frontend 5897 unit tests, 690 library story tests, 17 channel and kanban-busy e2e specs; no Go change, so the Go suites run in CI
- [x] Lint clean (`just lint`): frontend eslint 0 errors (203 warnings, same as develop), e2e lint and typecheck, lint-md, vue-tsc for app and library
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: the gate covers Go; no Go change)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: Go coverage; no Go change, CI runs it)

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

**Review Responses:**

- Design review (planning): RR-KT4LFHB, RR-NLALX2S, RR-5DWURFY (significant,
addressed); RR-BVI8KCY, RR-UD94CIR, RR-UT8ZDEK, RR-DZ5PIQQ, RR-XJX65R3 (minor,
addressed); RR-TMPC2DD, RR-H72E2JU, RR-6ICLAYD (nit, addressed).
- Code review (cranky-code-reviewer): RR-MOKG4FT (significant, addressed:
header decisions kept their reply to the cause); RR-IS8WRB2, RR-XIM06AF,
RR-4ZHC86U, RR-15BB0AH (minor, addressed); RR-GJKIWOD, RR-A67TE3M (nit,
addressed); RR-UDX26LC (nit, wont-fix: to check in a VoiceOver session).
- Security review (rela-security-reviewer): no findings. Only the SPA is
touched; output is escaped interpolation, and folding cannot hide a move into a
waiting or final state.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS for every criterion in PLAN-Y5X8W48. Evidence per
criterion is in IMPL-GLG831U: channelFold table cases (fold rule, hidden and
removed members, kept and shown-all rows), ChannelsPanel quiet types tests
(decisions, announcements, focus, reply quote, unread), useSessionDecisions
tests (placement and reply_to), the RlChannel stories (scroll anchor on re-fold)
and the e2e spec on a real server.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-SAKR2AH

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
