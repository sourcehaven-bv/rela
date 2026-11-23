---
id: REV-FJQRBMW
type: review-checklist
title: 'Review: Sankey view for data-entry (sankeys: config, layout, view)'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
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

**Review Responses:** none created as entities. A cranky-code-reviewer and a rela-security-reviewer ran on the diff. The security review had no critical or significant findings. The code review had four significant findings, all fixed: pips read the whole pip type (now an id-bounded gated read), the budget test did not measure that read (now `AssertScaleFree` over padded evidence rows), the ACL tests did not pin post-redaction and post-filter order (new `TestSankey_TruncatedIsPostFilter` and `TestSankey_HiddenFieldIsJudgedAfterRedaction`), and the layout collapsed above about 500 nodes in a column (height now follows rows and gaps). Fixed minors: id change kept old data, ResizeObserver lost its element, missing sort key order, quadratic hover, hardcoded colors, unencoded id in the API path, unsorted config errors. Deferred minors, with reasons: left-to-right truncation (document or share the budget in a follow-up), sankey depends on `*ganttHandler` and the constructor does not reject nil (shares the gantt seam; follow-up), edges that join no column types are not rejected at load, a faced pip type keeps the first face (never leaks, only order-dependent).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Renders nodes and ribbons: PASS (`sankey_handler_test.go`, `SankeyView.test.ts`, browser).
2. Hidden entities never appear or count: PASS (ACL, truncation and field-redaction tests).
3. Detail panel and scroll into view: PASS (`SankeyView.test.ts`, browser).
4. Pip cost does not grow with nodes or store: PASS (`TestSankey_PipsCostFollowsTheDiagramNotTheStore`, mutation-checked).
5. Bad config fails at load: PASS (`sankey_test.go`).

Run here: `go test -cover` on `dataentry` (88.7%), `dataentryconfig` (93.6%), `apiwire/v1`, `archguard`; golangci-lint on `dataentry` and `dataentryconfig`; `just arch-lint`; `just comment-lint`; vitest for `SankeyView`, `sankeyLayout` and i18n; eslint (no errors), prettier, `vue-tsc`. The full `just test` and `just coverage-check` run in CI.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-9T0A0J7

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
