---
id: DOCS-YKMKFV
type: docs-checklist
title: 'Docs: condition: predicate expressions on list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code documentation

- [x] Exported symbols documented
- [x] Non-obvious decisions explained where they live

New exported surface, each carrying the reasoning rather than a restatement of
the signature: `store.PropNotEqualOrEmpty`, `store.PropGreaterEqual` /
`PropLessEqual`, `store.GraphQuery.Narrowing`, `store.NarrowBranch`,
`conditionlint.CompileViewConditions` / `ViewConditionMatchers`,
`dataentry.ViewConditionMatcher` / `Lookup` / `Func`, `SetViewConditions`,
`AdaptViewConditions`, `appbuild.ViewConditions`.

Three docs are load-bearing beyond the symbol they sit on:

- `PropNotEqualOrEmpty` explains why TWO near-identical operators are correct
— picking the wrong one is a silent wrong answer in one direction or the other,
so the doc says how to choose (by which dialect authored the comparison) rather
than what the operator does.
- `PropGreaterEqual` records why the list refusal exists at BOTH the caller
gate and the store, since each catches something the other cannot: a DECLARED
list at config load (where the useful error is), and a list VALUE under a scalar
DECLARATION at runtime (legacy rows, imports) where only the store keeps the
backends agreeing.
- `GraphQuery.Narrowing` spells out the privilege escalation the type split
prevents, because a future reader's obvious simplification is to reuse `Any`.

`queryplan.stringComparableOnEveryType` now states that it is the MANDATORY gate
for the ordered ops — the store cannot consult the metamodel, so a caller
skipping it would push `"10" < "9"`.

## Project documentation

- [x] `docs/data-entry.md` — new "Conditions (`condition:`)" section
- [x] `docs-project/entities/guides/GUIDE-data-entry.md` — the SOURCE; `docs/`
is generated from it, so the edit was made here and regenerated
- [x] List field table gains a `condition` row linking to the section
- [x] ~~`docs/metamodel.md`~~ (N/A: the host-function set is unchanged)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI surface in this slice)
- [x] ~~`CLAUDE.md`~~ (N/A: deferred with the pushdown work — the
condition-engine section's surface list changes when the kanban path and SQL
evaluation land, so amending it now would describe a half-state)
- [x] ~~README.md~~ (N/A: no project-level change)

The section documents four things, in order of how likely they are to bite:

1. the key itself, with the atlas rule as the worked example
2. that a kanban `condition:` is **refused at startup**, so nobody relies on it
3. the startup-error behaviour, with the real message text
4. the three traps — `~=` vs `!=`, the unset-property divergence between the
two adjacent keys, and `days_between` argument order

## External documentation

- [x] ~~Release notes / changelog~~ (N/A: no changelog in this repo)
- [x] ~~Migration notes~~ (N/A: purely additive — an absent `condition:` means
no constraint, so every existing config behaves exactly as before)

## Verification

- [x] Every code sample executed against the live engine, not assumed

This caught two errors before they shipped: the `is_current_user` sample needs
`CompileWithCurrentUser` (views use it; my first test used plain `Compile`), and
the quoted startup error was missing its real `predicate: compile error at line
1:` prefix.

- [x] `markdownlint` clean on both files
- [x] Regeneration idempotent, so `docs-check` passes in CI
