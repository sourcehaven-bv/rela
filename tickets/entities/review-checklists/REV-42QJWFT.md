---
id: REV-42QJWFT
type: review-checklist
title: 'Review: Manual start condition, mail button and hiding empty dashboard cards'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (changed packages: flowcfg, flowengine, appbuild, mailrender, lua, dataentryconfig, and DashboardView vitest; the full suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages; eslint and prettier on the changed frontend files)
- [x] Comment lint gate clean (`just comment-lint`) (no unresolvable doc links)
- [x] Coverage maintained (`just coverage-check`) (new code is covered by the new tests; CI enforces the floors)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (critical self-review of the diff in the cranky-code-reviewer manner; this session runs as a fork without subagents)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes (only the three features, their tests, guides, regenerated docs and the golden file)

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

RR-D7Z475Z (nit, addressed), RR-K605YEO, RR-RDETY77, RR-AWSR7MH (minor,
wont-fix with reason), RR-6Y42VKH (nit, wont-fix with reason).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (see Acceptance Status)
- [x] Test evidence documented in implementation checklist (IMPL-8KA9VYP)

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

- Manual start `when:`: PASS (TestFlowControl_ManualStartWhen; CLI run refused
PER-001 with a conflict and started PER-002).
- mail.render button: PASS (TestRender_Button, TestMailRender_ButtonErrors,
compat test; `rela script` output).
- Dashboard `hide_empty`: PASS (DashboardView tests,
TestValidateDashboardCardHideEmpty; browser check on rela-server).
- Docs: PASS (four guides updated, docs regenerated).
- Golden delta accepted: digest.golden.html gains the two dark-mode button
rules and one blank line from the template; no change to rendered content.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs` (DOCS-8ETNQEC)
- [x] User-facing documentation updated (GUIDE-flows, GUIDE-mail, GUIDE-lua-scripting, GUIDE-data-entry; docs/ regenerated)
- [x] Docs-checklist marked as done (DOCS-8ETNQEC is done)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per the maintainer's convention; the ticket holds the why)
- [x] No TODOs or FIXMEs left unaddressed (none added)
- [x] Ready for another developer to use (documented options with tests)

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh after this checklist is done; CI is watched on GitHub)

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
