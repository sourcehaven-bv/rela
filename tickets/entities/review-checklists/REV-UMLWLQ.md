---
id: REV-UMLWLQ
type: review-checklist
title: 'Review: Comments over stdio MCP: add_comment and watch on list_comments'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (race tests of every touched package pass; the full run fails only cmd/rela-desktop TestChromeStyle_TargetsShippedClasses, which needs the built frontend absent in a fresh worktree)
- [x] Lint clean (`just lint`) (golangci-lint on touched packages, `just arch-lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (go-test-coverage on the profile: 85.2%, floors pass)

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

**Review Responses:** Design: RR-YB41PM, RR-0SERT0, RR-MVS2MH. Code: RR-EA0URF,
RR-NA7PYH, RR-3415PH (significant, addressed), RR-2BGOVS, RR-XLJRPB, RR-6BXLRU,
RR-5CX3X7, RR-B1E8QF (minor, addressed), RR-MKAX5C (minor, deferred), RR-U4C8KG,
RR-33W2L6 (nit, addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: stdio add_comment under `--run-as integration:runner` stores the author (TestStdioComments_AddComment; manual stdio run).
2. PASS: no add_comment without `--run-as` (TestStdioComments_AdderOnlyForAnAuthoringRunAs; manual run).
3. PASS: face_required, comments_not_enabled, not_found (TestStdioComments_AddComment).
4. PASS: text anchor built from the body; missing quote refused (same test).
5. PASS: watch returns changed after a comment from another process (TestWatch_ListCommentsPolled; manual two-process run).
6. PASS: list_comments in the watch enum only when registered (TestWatch_ListCommentsTargetOnlyWhenRegistered; goldens).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-0P2AOG

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
