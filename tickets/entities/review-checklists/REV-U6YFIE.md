---
id: REV-U6YFIE
type: review-checklist
title: 'Review: Wire a policy-backed FieldWriteGate so MCP and Lua callers cannot write fields their policy hides'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): default, sqlite (touched packages) and the postgres build. `cmd/rela-desktop` TestChromeStyle fails locally only because the SPA is not built here; this diff touches neither.
- [x] Lint clean (`just lint`): golangci 0 issues, arch-lint OK, plimsoll OK
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): all floors pass after the branch picked up 28b0f2c9 (visibility 89.6%)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer, then a re-review of the redesign (no critical or significant findings)
- [x] All critical review-responses addressed: RR-OX8G0N
- [x] All significant review-responses addressed: RR-GBM0DP, RR-KLEWD5, RR-RC68LE, RR-XSU5T5, RR-7SOF5L and RR-9OUEE9 are addressed. RR-MSASEO is wont-fix: F8 parity, documented.
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-OX8G0N, RR-GBM0DP, RR-KLEWD5, RR-MSASEO, RR-RC68LE,
RR-XSU5T5, RR-7SOF5L, RR-9OUEE9, RR-H98KO1, RR-PLMEM6, RR-51SRNI, RR-WYEQBD,
RR-FRSVK9 (deferred to TKT-ZNS7V8), RR-4OUWXC (deferred to BUG-KABQT1),
RR-V6D75F, RR-MOR0TG. The resolver and lookup consolidation was deferred to
TKT-2U059N.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** After the review, the scope was narrowed. "MCP" means MCP
on rela-server, and "Lua" means scheduled Lua. The CLI's Lua and `rela mcp` over
stdio stay ungated.

1. PASS. Refusals on update and unset, enum options and undeclared fields: TestCheckFieldWrite, TestFieldGate_PolicyHoldsOnGatedHandle and TestFieldGate_LuaUpdateRaises.
2. PASS. Create is refused the same way: TestCreateEntity_FieldGate and TestFieldGate_PolicyHoldsOnGatedHandle.
3. PASS. The CLI stays ungated: TestFieldGate_OperatorSurfacesUngated and a manual `rela update` run. `rela scheduler` is gated through ScheduledLuaWriteDeps.
4. PASS. Automation output and elevated writes are not field-gated: TestGated_DropsFieldGate. The three constraint tests keep their assertions, but each now builds its manager through `FieldGated`. Without that change they would pass trivially.
5. PASS. dataentry is unchanged, and the existing affordance tests and TestFieldWriteRulesMatchWire pass.
6. PASS. The gate is permissive without a policy, and a resolver failure refuses construction (fieldPolicy tests).
7. PASS. `just arch-lint` passes.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: docs/acl-security.md
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-BR8D2N

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (stacked on #1794)

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
