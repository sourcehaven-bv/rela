---
id: REV-BUKDTM
type: review-checklist
title: 'Review: rela channel: run message actions and show session state'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (full run inside `just coverage-check`: no failures; internal/cli race run passes)
- [x] Lint clean (`just lint`) (golangci-lint on internal/cli, `just arch-lint`, `just plimsoll`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (85.4%, floors pass)

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

**Review Responses:** Design: RR-0UW3B0 (significant), RR-7RQD65, RR-3OUAAH.
Code: RR-WSRKNQ, RR-M8ZHYX, RR-7NY5AZ (significant), RR-75QGWA, RR-7O1H90,
RR-UK6IJJ, RR-6C1TFS, RR-9L5WNO (minor, the last from the security review),
RR-CQ9TI9 (nit). All addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: promote by a member, text and JSON (TestChannelCLI_Action; manual DEC-001 on a rela-mission copy).
2. PASS: undeclared action, missing seq, role without the action refused with typed denials (TestChannelCLI_Action).
3. PASS: session line, transitions, JSON (TestChannelCLI_Session; manual run).
4. PASS: no_session, realtime off path shared via cs.svc(), hidden anchor equals rela show (TestChannelCLI_Session, TestChannelCLI_HiddenAnchorTextEqualsShow).
5. PASS: --release routing and output (TestChannelCLI_ActionOutput); refusals action_not_pending and usage errors (TestChannelCLI_Action).
6. PASS: identityWrites entries; the completeness test passes.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-5FR0I2

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
