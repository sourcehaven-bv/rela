---
id: REV-UT073PE
type: review-checklist
title: 'Review: Configure: edit flows.yaml triggers'
started: "2026-11-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test for configedit, appbuild, flowcfg/..., acl/..., cmd/rela-server and the configure frontend tests pass; the commit hook runs the full suite)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: floors are checked in CI; every new file has its own tests)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer, after the TKT-CB1YV7D work too)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** linked to TKT-ZT2KREN via `has-review-response`. The
trigger ACL check round: RR-F9JV1S7, RR-15MSZEY, RR-QHL18DR, RR-823CLRS,
RR-25REQKG, RR-45P8KC3, RR-VRK09VX, RR-CJ57B4Z, RR-PMW0EJO, RR-A7WRMCY
(addressed), RR-HHJ15NW, RR-LRARZQ9 (nits, wont-fix). RR-1E63BRL,
RR-MKK1FG6, RR-3O36CF7, RR-3HQ215K and RR-SYGQ4TR are addressed by the
policy check.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- flows.yaml is a Configure file with a per-key allowlist: PASS
  (TestCheckEdit_Flows, TestSave_Flows*).
- Automations view restored in the SPA: PASS (configure frontend tests).
- Save keeps --config-editing, config:edit, fs build and audit: PASS
  (TestConfigure_Flows*).
- Trigger edits are judged against acl.yaml and the editor's grants: PASS
  (TestCheckTriggerACL, TestCheckTriggerACL_FailsClosed,
  TestConfigure_FlowsTriggerACL).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-M61UORB

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after this checklist; see TKT-UFV01M)

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
