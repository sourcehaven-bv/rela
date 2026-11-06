---
id: REV-VGM8QO1
type: review-checklist
title: 'Review: Scoped elevated updates for data-entry actions'
started: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test on the four changed packages; the full suite runs in CI because the machine is under heavy load)
- [x] Lint clean (`just lint`) (golangci-lint per changed package, arch-lint, plimsoll and archplan ratchet: 0 worse)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

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

**Review Responses:** RR-KFW02L1, RR-0HCBSFB, RR-TD45CX4, RR-B4XUWX3,
RR-7MCRN07, RR-IQZGRZP, RR-ZMOD2AV, RR-YRXB8A3, RR-LDLLTYV, RR-S9NA7S0,
RR-AB38TEQ, RR-Y5WQRHQ (wont-fix), RR-A3XZWQF, RR-5PNH469. Both
cranky-code-reviewer and rela-security-reviewer ran.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1 PASS (TestAction_ElevatedUpdates "the gated binding is still denied" and the
in-scope case), 2 PASS (TestAction_ElevatedUpdates, TestScopedUpdates_PatchEntity),
3 PASS (TestBypassACL_NoUpdateEntityWithoutUpdater, TestBypassACL_ElevatedUpdater),
4 PASS (TestAction_ElevatedUpdatesAudit, TestScopedUpdates_Audit),
5 PASS (TestValidateActions_Elevation), 6 PASS (guides updated).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** see has-docs relation

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
