---
id: TKT-0N16TD
type: ticket
title: 'Traverse condition: predicate expressions on view traverse steps'
kind: enhancement
priority: medium
effort: m
started: "2026-10-15"
completed: "2026-10-16"
status: done
---

## Problem

A view traverse step takes one `where:` clause in the filter DSL (`status =
gereed`, `type = taak`). It cannot combine clauses, so a section cannot show
only the finished tasks of a project, or the unfinished tasks together with
every sub-project.

An unparseable `where:` was also dropped at render time, which showed every
traversed entity (BUG-WHEREWIDE).

## Solution

A traverse step takes a `condition:`: a predicate expression in the same
language as a list `condition:`, ANDed with `where:`. `where:` is unchanged.

```yaml
- from: entry
  follow: bestaat_uit
  collect_as: lopend
  condition: "entity.type == 'project' or (entity.type == 'taak' and entity.status ~= 'gereed')"
```

- A separate key, not a richer `where:`, per `.claude/rules/predicate.md`: the
two syntaxes overlap without erroring, so the key declares the dialect.
- Compiled at load once per type the step can collect
(`dataentryconfig.TraverseTargetTypes`) by
`conditionlint.CompileViewConditions`, so `rela validate` and the server refuse
a condition that does not compile.
- `predicatefns.Evaluator.CompileForCollection` adds `entity.type` and
`entity.id`, and lets a property another collected type declares read as nil.
Limited to this entry point because `type` and `id` are not properties for SQL
lowering.
- Evaluated against the redacted entity, as a list condition is.
`related(...)` is refused: the step runs in the view's fixpoint loop.
- Side panels refuse `condition:`.
- An unparseable `where:` on any traverse step (views and side panels) is now
a load error, which closes BUG-WHEREWIDE.
