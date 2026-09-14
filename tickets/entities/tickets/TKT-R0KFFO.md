---
id: TKT-R0KFFO
type: ticket
title: 'condition: predicate expressions on list views (store primitives + list read path)'
kind: enhancement
priority: medium
effort: l
status: done
---

## What shipped

A `condition:` key on `lists:` holding a predicate expression, ANDed with
`filters:`, so a list's membership rule can use `or`, grouping, negation of a
compound, and the date-arithmetic host functions — none of which a flat ANDed
`filters:` list can express.

```yaml
lists:
  actieve_taken:
    entity_type: taak
    condition: >-
      entity.status ~= 'gereed'
      or (entity.afgerond_op ~= nil
          and days_between(today(), entity.afgerond_op) <= 2)
```

This is the first landable slice of **[[TKT-LPLZ1V]]**, which stays open for the
kanban read path, disjunctive pushdown, and the remaining SQL-evaluation work
(its steps 5-8). Split out because that arc cannot reach `done` for some time,
and the list capability is complete and useful on its own.

## Store primitives (steps 1-3)

Three additions to `store.GraphQuery`, each landing alone and each verified by
deliberately breaking it:

1. **`PropNotEqualOrEmpty`** — the sound lowering target for Lua `~=`.
`internal/predicate` documents Lua equality (`nil == anything -> false`), so
`nil ~= 'x'` is TRUE; `internal/propmatch` (shared with `internal/filter`)
answers the filter-DSL question, where an unset property is not in the
population. Both are right for their own contract, so `propmatch` is untouched
and `PropOp` gained an operator instead. Lowering `~=` to `PropNotEqual` would
have dropped every row whose property is unset — rows the Go pass keeps.
2. **`PropGreaterEqual` / `PropLessEqual`** — byte-wise ordered comparison,
the contract `GraphQuery.OrderBy` already carries. The caller must gate on
`queryplan.stringComparableOnEveryType` (integers excluded: byte order is not
numeric order). Empty and LIST values never match; the list refusal is
load-bearing, since Go renders `[]string{"a","b"}` as `[a b]` where postgres
`->>` gives `["a", "b"]` — a byte comparison over a list would answer
differently per backend.
3. **`Narrowing []NarrowBranch`** — caller-supplied narrowing, kept as a
DISTINCT TYPE from the ACL-owned `Any`. Appending caller branches to `Any`
(which is OR-ed internally) would yield `acl_a OR acl_b OR caller_x` where the
meaning must be `(acl_a OR acl_b) AND caller_x` — a caller branch becoming an
alternative route to authorization. The types make that a compile error rather
than a review catch.

## Read path

- Compiled at config load (`conditionlint.CompileViewConditions`), so a
condition that does not compile is a **startup error** naming the view and the
attribute — not a view that is silently empty forever.
- Reaches the server as `?list_id=<id>`; the server resolves the expression
from its OWN config, so the predicate never crosses the wire.
- No `list_id` means no condition: `/api/v1/{plural}` stays the generic entity
endpoint. A condition is presentation, not authorization, so omitting it yields
the ACL-scoped superset — never more than the principal may see.
- Evaluated after the ACL scope and every filter, but BEFORE paging and the
count, so the page and total describe the same population.
- **Pushdown declines when a condition is present**, explicitly.
`planListPushdown` inspects only `filter[...]` params and `continue`s on
everything else, so a condition it never saw would not make it decline — it
would push a paged query and return the unfiltered superset, which is
`BUG-F1LTP1`'s failure shape.

## Kanban: refused for now

`Kanban.Condition` exists on the struct (shared with `List`) and the compiler
handles it, but the board still filters client-side, so config validation
**rejects** it. A key that validated and then did nothing is the silent no-op
`BUG-F1LTV0` and `BUG-MYN56J` are both about. The guard and its test name what
to delete when the board gains a server read path.

## Documented traps (verified against the live engine)

`filters:` and `condition:` sit adjacent in one block and are different
languages. Every documented sample was executed, not assumed:

| | `filters:` | `condition:` |
|---|---|---|
| not-equal | `!=` | `~=` (`!=` is a parse error) |
| unset property, not-equal | excluded | **included** |

Plus two that produce a wrong answer with no error:

- `days_between(a, b)` counts from **b to a**, so an age is
`days_between(today(), prop)`. The reverse yields a negative number, and `-10 <=
2` is true — every old row matches and the filter appears inert.
- A date function on an unset property raises an **evaluation error**, not
`false`. The `~= nil` guard is required; the truthiness shorthand does not
compile.

Writing the docs also caught two errors of my own before they shipped: the
`is_current_user` sample needed `CompileWithCurrentUser`, and the quoted error
message was missing its real `predicate: compile error at line 1:` prefix.

## Verification

- `storetest` conformance on memstore, fsstore, sqlitestore and **pgstore**
(the parity suite is the only enforcement of the backend-parity rule, and SQL
changed here).
- Each store primitive mutation-tested: reverting the SQL reproduces the exact
defect the test exists to prevent.
- End-to-end tests drive the real HTTP handler with the production compiler —
a disjunctive rule filters the response, the total describes the filtered
population, an absent or unknown `list_id` is unconstrained.
- `go test ./...`, frontend 2458 tests, `vue-tsc`, ESLint (0 errors),
`arch-lint`, `comment-lint`, `markdownlint`, docs regeneration idempotent.

## Not in this slice

Kanban server read path; disjunctive (`or`) pushdown into SQL; ordered-`PropOp`
constant folding; the `sha256` rename; field-to-field comparison; refusing
`rrule_next` in favour of `computed:`. All tracked on [[TKT-LPLZ1V]].
