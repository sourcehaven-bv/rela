---
id: DOCS-DD8HLA5
type: docs-checklist
title: 'Docs: TKT-08ZI5XN'
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why date variables are not pushed down (`dateOnAnyType`), why `$now` takes no offset, why `now()` must bind the read time and never SQL `now()`, why `rebindFor` replaces the read time and `IdentityBinder` keeps it
- [x] Function/type docs if public API: godoc on the `readtime` package, `filter.DateVariable`/`ResolveDateVariable`/`BindDateVariables`, `Evaluator.ReadTime`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] `.claude/rules/predicate.md` lists `now` among the host functions
- [x] Help text accurate (if CLI changes): `--where` guidance in the CLI reference uses single quotes for `$today`

## User-facing Documentation

- [x] `GUIDE-data-entry.md` (variables table, "Dates, times and the current day", corrected overdue and `days_between` examples), `GUIDE-cli-reference.md`, `GUIDE-flows.md`, `GUIDE-mail.md` (`due < $today`); regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change; the read time is not a principal input)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated~~ (N/A: no request or response shape changed)
