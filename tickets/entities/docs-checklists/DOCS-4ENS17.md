---
id: DOCS-4ENS17
type: docs-checklist
title: 'Docs: TKT-TX961Z'
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] README updated (if applicable)
- [x] CLAUDE.md updated (if new patterns)
- [x] Help text accurate (if CLI changes)

## External Documentation

- [x] Changelog entry added
- [x] API docs updated (if applicable)

## Notes

- Scope: the Basecamp example syncs every to-do the account can see, mirrors to-do lists and projects as `todolist` and `project`, links them with `in-list` and `in-project`, and converts bodies with `rela.md.from_html` / `to_html`. Out of scope: pushing list or project changes, modelling to-do list groups.
- Approach: Basecamp's recordings listing (type Todo and Todolist, oldest first) instead of one list's to-dos; projects come from each list's bucket. New todos post to the linked list or the optional `basecamp_todolist_id`. Lossy bodies are never pushed and are recorded as a `body:` conflict.
- Security: the connector role gets the three types and a `basecamp-links` relation grant; no delete. Basecamp HTML is sanitized by htmlmd before it reaches rela.
- Tests: `TestBasecampExample` against the extended stub: mirrors and links, moves, renames, HTML body conversion, attachment and local-image conflicts, list targeting, deleted mirror, plus the existing cases. Passes with -race -count=3.
- Review: cranky-code-reviewer found three significant issues (deleted mirror stops the pull, cost on large accounts, cached next link), all addressed; minor findings addressed or documented.
- Docs: examples/basecamp/README.md.
