---
id: DOCS-O1DQ98
type: docs-checklist
title: 'Documentation: condition expressions (TKT-8GD41J)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

Non-obvious decisions carry their rationale at the call site: why `condition:`
is a separate key from `when:` (the two syntaxes overlap without erroring), why
`days_between` returns `Int` not `Number` (ordered comparison needs matching
types, and an integer property is the motivating case), why day counts are
computed on Unix seconds (`time.Duration` saturates at ~292 years), why
`date_add` refuses month/year, and why `*Result` is threaded through the match
chain rather than stashed on the `Engine` (it would race).

Two doc comments that asserted behaviour the code did not have were corrected
rather than left to mislead.

## Project Documentation

- [x] README updated (if applicable)
- [x] CLAUDE.md updated (if new patterns)
- [x] Help text accurate (if CLI changes)

`CLAUDE.md` records the two-dialect rule — `condition:`/`when_condition:` take
expressions, `when:`/`then:` take filter clauses, they are separate keys on
purpose, and don't add dialect sniffing because the key IS the declaration of
intent. Also records that a failing `condition:` is a load error.

README needs nothing: no project-level change. No CLI surface changed, so help
text is unaffected — `--filter` already existed.

## External Documentation

- [x] Changelog entry added
- [x] API docs updated (if applicable)

The repo has **no CHANGELOG file** — release notes are generated from commit
messages by GoReleaser (`docs/releasing.md`). The upgrade-relevant information
therefore went where an operator will actually meet it: an explicit **Upgrade
note** in `docs/metamodel.md` beside the fail-loud behaviour it describes,
covering what changed, why it can only affect an already-broken clause, the
three shapes the filter parser rejects, the plausible `- "status: todo"` typo,
and how to read the error. The commit messages carry the same reasoning.

`docs/metamodel.md` (generated from `docs-project/entities/guides/`) gained:

- the `condition:` trigger key in the trigger table
- an "Expression Conditions" section with the available host functions
- `when_condition:`/`then_condition:` on validation rules, and a
"`when:` vs `when_condition:`" section explaining the two dialects
- a **"Guard optional date properties"** section — a date function on a missing
property is an eval error, not `false`, so an unguarded condition silently skips
exactly the entity an operator often most wants flagged. Shows the guarded and
inverted forms, and notes that `when_condition:` and `then_condition:` fail in
opposite directions from that same cause.
- the upgrade note above

`docs/cli-reference.md` gained the date-function table and worked `--filter`
examples (`days_between`, `date_add`), plus notes that every date value is
UTC-normalized, that `date_add` takes only day/week, and that `rrule_next`
distinguishes a malformed rule from an exhausted one.

Both regenerated with `./scripts/generate-docs.sh` from the `docs-project/`
sources, as CI's freshness check requires.
