---
id: DOCS-S8CF2L
type: docs-checklist
title: 'Docs: lua: reject non-string filter options in the gated rela.get_relations'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious

The rejection branch in `relationQuery` explains *why* a non-string raises
rather than being skipped, and now covers both callers: on the elevated path
nothing gates an over-broad query, while on the gated path peer-gating bounds
the rows but the script is still silently answering the wrong question.

The call site in `luaGetRelations` records that it deliberately shares
`relationQuery` with `admin.get_relations` so the two surfaces cannot drift.

- [x] Function/type docs if public API

`relationQuery`'s godoc was rewritten when it lost the `elevated` prefix: it now
states that it is shared, and that callers differ only in what they do with the
result. `luaGetRelations`'s doc comment gained the "each an optional string"
contract.

## Project Documentation

- [x] ~~README updated~~ (N/A: no new command, flag, or top-level capability)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern — this REMOVES a duplicate
implementation rather than introducing a convention)
- [x] ~~Help text accurate~~ (N/A: no CLI surface changed; `rela.get_relations`
is a Lua binding)

## External Documentation

- [x] API docs updated

`docs-project/entities/guides/GUIDE-lua-scripting.md` — added an options-table
contract block after the Query Functions table: keys are optional and must be
strings, omitting a key means no constraint, and a worked example. Two failure
cases are shown explicitly, including the deliberate asymmetry that a bare id
(`rela.get_relations(e.id)`) is NOT a filter and returns every relation, since
`opts` is documented as optional and only values inside the table are typed. The
elevated-access section now records that both bindings share one rule.

`docs-project/entities/guides/GUIDE-scheduled-tasks.md` — corrected the
orphan-report example, which called `rela.get_relations(e.id)`. Measured against
the live graph it returned 2246 relations instead of 2, so its `#rels == 0`
check could never fire and the documented scheduled task would have reported "No
orphaned entities found" forever. The fix carries an inline comment naming the
trap so the example teaches the contract instead of modelling the bug.

Both regenerated via `just docs`; `just docs-check` passes in the pre-push hook,
which is what gates `docs/*.md` (generated) against these sources.

- [x] ~~Changelog entry added~~ (N/A: repo keeps no CHANGELOG — release notes
are derived from PR titles; #1239 is titled for this change)

## Note on the earlier N/A

The review checklist initially marked this whole section N/A on the grounds that
the docs edits were small and inline. That was wrong: the change is user-facing
behavior on a documented public binding, and one of the edits fixed a broken
example. The `analyze_validations` rule "Done enhancement tickets must have
completed docs checklist" caught it, which is the rule working as intended.
