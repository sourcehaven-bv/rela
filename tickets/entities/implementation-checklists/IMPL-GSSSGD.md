---
id: IMPL-GSSSGD
type: implementation-checklist
title: 'Implementation: Basecamp connector syncs all to-dos and models projects'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

## Notes

- Scope: the Basecamp example syncs every to-do the account can see, mirrors to-do lists and projects as `todolist` and `project`, links them with `in-list` and `in-project`, and converts bodies with `rela.md.from_html` / `to_html`. Out of scope: pushing list or project changes, modelling to-do list groups.
- Approach: Basecamp's recordings listing (type Todo and Todolist, oldest first) instead of one list's to-dos; projects come from each list's bucket. New todos post to the linked list or the optional `basecamp_todolist_id`. Lossy bodies are never pushed and are recorded as a `body:` conflict.
- Security: the connector role gets the three types and a `basecamp-links` relation grant; no delete. Basecamp HTML is sanitized by htmlmd before it reaches rela.
- Tests: `TestBasecampExample` against the extended stub: mirrors and links, moves, renames, HTML body conversion, attachment and local-image conflicts, list targeting, deleted mirror, plus the existing cases. Passes with -race -count=3.
- Review: cranky-code-reviewer found three significant issues (deleted mirror stops the pull, cost on large accounts, cached next link), all addressed; minor findings addressed or documented.
- Docs: examples/basecamp/README.md.
