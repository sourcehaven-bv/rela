---
id: REV-3M0QZL
type: review-checklist
title: 'Review: Basecamp connector syncs all to-dos and models projects'
started: "2026-10-09"
completed: "2026-10-09"
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

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

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

## Notes

- Scope: the Basecamp example syncs every to-do the account can see, mirrors to-do lists and projects as `todolist` and `project`, links them with `in-list` and `in-project`, and converts bodies with `rela.md.from_html` / `to_html`. Out of scope: pushing list or project changes, modelling to-do list groups.
- Approach: Basecamp's recordings listing (type Todo and Todolist, oldest first) instead of one list's to-dos; projects come from each list's bucket. New todos post to the linked list or the optional `basecamp_todolist_id`. Lossy bodies are never pushed and are recorded as a `body:` conflict.
- Security: the connector role gets the three types and a `basecamp-links` relation grant; no delete. Basecamp HTML is sanitized by htmlmd before it reaches rela.
- Tests: `TestBasecampExample` against the extended stub: mirrors and links, moves, renames, HTML body conversion, attachment and local-image conflicts, list targeting, deleted mirror, plus the existing cases. Passes with -race -count=3.
- Review: cranky-code-reviewer found three significant issues (deleted mirror stops the pull, cost on large accounts, cached next link), all addressed; minor findings addressed or documented.
- Docs: examples/basecamp/README.md.
