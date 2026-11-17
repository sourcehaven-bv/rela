---
id: REV-O61PKPH
type: review-checklist
title: 'Review: LLM actions: follow-ups (auto-apply, create outputs, batch runs)'
started: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (changed packages with -race: dataentry, appbuild, dataentryconfig, dataentrywire, llmtask, archguard, mcp, cmd/rela-server; frontend vitest and typecheck)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages, arch-lint, plimsoll, eslint)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained~~ (N/A locally: the full run timed internal/dataentry out at 10m under load while every test passed; CI enforces the floors)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed (RR-QXZEXOU deferred to TKT-BHYJZCV with reason)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-WYC7Z81, RR-WJ42HC6, RR-RA8BG6T, RR-HM56I91, RR-CPNYIS8,
RR-UWUI00L, RR-8Y3PXNS, RR-J02GVLA, RR-QXZEXOU, RR-6CVFZEO, RR-HR1ALK5, RR-DF9B2OM,
RR-DV69SVA, RR-DBQ0NUO, RR-WN3PJDM, RR-4E91D00, RR-ZXPYDQJ

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Auto-apply via jobs: PASS (TestLLMAction_AutoQueues, TestLLMActionRunner_Applies, TestHandleLLMActionJob, TestLLMActionJob_CeilingAndRedelivery)
- Create outputs: PASS (TestLLMActionRunner_Creates, TestCreate_Schema/Parse/Room/RequiredProperty, dialog tests)
- List and flow runs: PASS (TestFlowActionUsable_LLM, list action tests)
- Server provider config: PASS (TestWithServerAIConfig)
- Candidate pushdown and max lowering: PASS (earlier work, RR-ZXPYDQJ)
- Not run against a live provider.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-1T12H4L

## Final Checks

- [x] ~~Commit message explains the why~~ (N/A: not committed in this session; the user commits)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command~~ (N/A: the user opens the PR)

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
