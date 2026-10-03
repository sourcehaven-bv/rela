---
paths:
  - "internal/predicate/**"
  - "internal/predicatefns/**"
  - "internal/filter/**"
  - "internal/automation/**"
  - "internal/validation/**"
  - "internal/statemachine/**"
  - "internal/affordances/**"
  - "internal/conditionlint/**"
  - "internal/metamodel/**"
  - "internal/dataentryconfig/**"
  - "internal/scopes/**"
  - "internal/mailtemplate/**"
  - "internal/appbuild/viewconditions.go"
  - "internal/cli/list.go"
---

# Condition engine: `internal/predicate` + `internal/predicatefns`

`internal/predicate` is the shared **typed expression engine** — a sandboxed
Lua-expression subset with no I/O and fixed depth/step budgets. `Compile`
retains the boolean condition profile; `CompileValue` accepts an explicit
context profile for scalar computations. Programs expose exact static record
dependencies and conservative SQL-portability metadata. Context profiles may
enable or refuse language features, but an accepted IR node must keep identical
semantics across evaluators and future targets. `internal/predicatefns` is its
metamodel-aware glue: the `ScalarType`/`EntityRecordType` type adapter, the
host-fn stdlib (`match`/`regex`/`fuzzy`/`contains`/`len`/`today`), the
`FromFilter` transpiler, and the `Evaluator` (compile-once, metamodel-scoped
Program cache). New condition/`when:`-style code evaluates through `predicate`.

These surfaces are on predicate: ACL affordance `when:`
(`internal/affordances`), state-machine transition `When:`
(`internal/statemachine`), wizard-form condition lint
(`internal/conditionlint`), automation `on.when:`/`validate:`
(`internal/automation`), metamodel validation `When:`/`Then:`
(`internal/validation`), and the CLI `--filter` flag (`internal/cli/list.go`).

Automation `on.condition:` and validation `when_condition:`/`then_condition:`
take predicate **expressions** as written, ANDed with the filter-syntax
`when:`/`then:` keys beside them. They are separate keys because the two
syntaxes overlap without erroring: `filter.Parse` accepts
`days_between(entity.due, today()) <= 7` as a filter on a property named
`days_between(entity.due, today())`, which matches nothing, silently. Don't add
dialect sniffing — the key IS the declaration of intent. A `condition:` that
fails to compile is a **load error** (`NewEngineFromMetamodel` returns one), as
is an unparseable `when:` clause: dropping a constraint widens the automation,
so failing the load is the safe direction.

`internal/filter` is NOT frozen — it remains the **query-filtering** DSL (the
`--where` string syntax and metamodel legacy filter-strings). Legacy
`--where`/`When:`/`Then:` inputs are transpiled to predicate via
`predicatefns.FromFilter` on load (`--where` is deprecated in favor of
`--filter`). `filter.Match` still directly backs query-filtering in
`internal/dataentry` (SPA view/feed `where:`), `internal/lua` (script queries),
`internal/search/searchparser`, and `internal/cli/analyze.go` — these were
**not** migrated (they filter result sets, they don't gate conditions). Don't
describe filter as "removed" or "frozen"; it's the query-filter DSL, predicate
is the condition/policy engine.
