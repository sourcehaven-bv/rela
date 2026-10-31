---
id: REV-0ZGPQ35
type: review-checklist
title: 'Review: Card wall board with optional drag-to-group'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (full Go suite via `just coverage-check`, exit 0; `npx vitest run` 361 files pass; e2e kanban-wall 48/48 repeated runs plus kanban and relation-order specs)
- [x] Lint clean (`just lint`) (golangci-lint on changed packages under all four build tags: 0 issues; eslint on changed frontend and e2e files: no errors; rela-components `npm run check` passes)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (total 85.6%, all thresholds PASS)

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
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (RR-9XPNXMP, RR-6TZ0ZNY, RR-L2BF2WL)
- [x] Self-reviewed the diff for unrelated changes (`dpia-sessie.html` and TKT-TDDO7G0 are not part of this work and stay out of the commit)

**Review Responses:** RR-9XPNXMP, RR-6TZ0ZNY, RR-L2BF2WL (significant,
addressed); RR-KIGKXRV, RR-9HAD39K, RR-D8VSSBU, RR-AO3BDOX, RR-ZR86Y3R,
RR-H4N62QI (minor, addressed); RR-BP7UYXD (minor, wont-fix); RR-JMPCVN5 (minor,
deferred: Firefox check); RR-7YBG6M0, RR-0G0DHFX (nit, addressed); RR-H16BGIW
(nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-VP7YPKZ)

**Acceptance Status:**
- AC1 plain wall: PASS (component test; e2e reorder and reload)
- AC2 drop on loose card makes a group: PASS (component tests; e2e)
- AC3 drop on grouped card joins: PASS (component test; e2e)
- AC4 drop on pile ungroups: PASS (component test; browser QA)
- AC5 group move by handle: PASS (component test; e2e with reload)
- AC6 groups look like tinted cards: PASS (browser QA; class and tint binding)
- AC7 inline rename with restore: PASS (component tests; browser QA)
- AC8 non-wall boards unchanged: PASS (existing KanbanView suites; non-wall drop test)
- AC9 config validation: PASS (Go table test, 8 cases)
- AC10 reduced motion: PASS (useBoardFlip test)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (`docs/data-entry.md`)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-BMSH7G2

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per the user's preference; the ticket carries the why)
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
