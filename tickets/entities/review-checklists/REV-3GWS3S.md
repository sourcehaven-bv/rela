---
id: REV-3GWS3S
type: review-checklist
title: 'Review: MCP tools for comments'
started: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`; run as `just coverage-check`, which runs the same suite with coverage)
- [x] Lint clean (`just lint`, all four build tags: 0 issues; `just arch-lint` OK; `just plimsoll` OK)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`: total 82.8%, thresholds pass)

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
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-MAA6TM, RR-03LS0Q, RR-NRYP5Q, RR-WWZKJK)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-MAA6TM, RR-03LS0Q, RR-NRYP5Q, RR-WWZKJK (significant,
addressed); RR-O2HMF4, RR-7HQR9V, RR-SF1PVQ, RR-T3O79I (minor, addressed);
RR-8Q0EW9 (minor, deferred); RR-9E92N3 (nit, addressed); RR-W7AE8W (nit,
wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS for AC1-AC9 per PLAN-Q62CDC; evidence in
IMPL-6WB4AP. Stdio registers only `list_comments` (verified by hand and by
`TestCommentTools_Registration` and the `tools_list` golden).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (docs/mcp-server.md, docs/comments.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-G4LKTW

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step, after `done`)

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
