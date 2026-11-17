---
id: TKT-8HY05WB
type: ticket
title: 'Store query contract: or, not and ordered comparisons'
kind: enhancement
priority: medium
effort: l
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

## Description

`store.GraphQuery` is a flat AND of property predicates plus relation
predicates. `queryplan.LowerCondition` therefore declines `or`, `not` over
anything but a traversal, and ordered comparisons, and those conditions are
answered by reading the whole type and filtering in Go
(`internal/mcp/tools_entity.go` `listPage.filterInGo`, TKT-9BKVQ9I;
`internal/desktopnotify/evaluate.go` `evaluation.scan`, TKT-I3MGCRT).

This is a limit of rela's store contract, not of SQL. Extend the contract so the
store answers these shapes as a query on every backend.

## Scope

- A boolean expression on `GraphQuery` (`Where *store.Cond`): And, Or, Not over
property predicates and relation predicates. Caller-supplied, ANDed with
everything else, never written by the ACL.
- Ordered operators `<`, `<=`, `>`, `>=` with an explicit value kind: byte-wise
over a stored string, and numeric over a stored integer.
- `LowerCondition` lowers `or`, `not` and ordered comparisons on string-shaped
and integer properties into that expression.
- Every backend: graphquerynaive (reference, fs and memory), pgstore,
sqlitestore. Conformance tests plus the randomized differential.

Out of scope, filed separately:

- Date and datetime ordered comparisons: TKT-4Z2BIRG (needs canonical date
storage first).
- Refusing conditions that do not lower, at load time or with a clear error,
instead of the Go scan: TKT-W0DYCEC.
- Pushing a condition over a field hidden on some rows by rewriting each leaf
against the field scope: TKT-FKE3VP3.

## Design

### Store contract (`internal/store/graphquery.go`)

- `GraphQuery.Where *Cond`: a caller narrowing, ANDed with every other field,
like `Narrowing` and `Related`. The ACL never writes it.
- `Cond{Op, Prop, Rel, Args}` with `Op` one of `CondProp`, `CondRelated`,
`CondAnd`, `CondOr`, `CondNot`. `CondAnd` with no arguments is true, `CondOr`
with none is false, `CondNot` takes exactly one. Nesting is capped
(`MaxCondDepth`); deeper is `ErrInvalidQuery`. A `CondRelated` leaf is a
`DirectedRelation` with the same shape rules as `Related` (checked by
`CheckEndpointShape`).
- Every leaf is two-valued: true or false, never unknown. SQL backends wrap the
operand of NOT in `COALESCE(..., FALSE)`, so an absent key never makes `NOT
leaf` drop a row.
- `Where` is tested on the row the world picks (the prime), like `Props`.
Relation leaves depend on the id only, so that placement is exact for them too.
- New ops `PropLess`, `PropGreater` next to `PropLessEqual`, `PropGreaterEqual`.
New field `PropPredicate.Kind`, valid only on the four ordered ops:
  - `OrderText` (zero): today's byte-wise rule on the text form; empty and list
values never match. Unchanged.
  - `OrderString`: matches only a stored string, the empty string included,
compared byte-wise (`COLLATE "C"` in postgres). This is exactly the Go rule for
a string-typed property: a non-string value binds nil.
  - `OrderInteger`: matches only a stored integer, compared numerically. A
stored integer is what `predicatefns` binds as `Int`: a Go int or int64, an
integral float64 in [-2^63, 2^63), or a string that `strconv.ParseInt` accepts.
`Value` must parse as int64 or the query is `ErrInvalidQuery`. The rule lives in
`internal/propmatch` (`propmatch.Integer`) and both `predicatefns` and
graphquerynaive call it.
- Dates are out: TKT-4Z2BIRG adds `OrderDate` once dates have one stored form.

### Null semantics (`internal/predicate`)

Opt-in, scoped to lowered surfaces (design review RR-R15R2DT). The
`predicate.NilOrderedIsFalse()` eval option makes an ordered comparison (`<`,
`<=`, `>`, `>=`) with a nil operand false instead of an `ErrNilOperand` error.
Unset, off-type and unparseable values are nil, so under the option they never
match an ordered comparison, and `not (x < v)` is true for them. Every store
answers the same way. `predicatefns.Evaluator.LoweringSemantics()` returns an
evaluator with the option. Only the surfaces whose conditions are lowered use
it: view conditions (conditionlint), query scopes (appbuild) and MCP
`list_entities` filtering. ACL `when:`, write rules, flows, triggers and
computed properties keep the error and fail closed. `==` and `~=` keep the
equality table (`nil == v` false, `nil ~= v` true).

### Backends

- graphquerynaive (fs, memory): evaluates the tree recursively with the
existing `matchesProp` and `matchesPredicate`.
- pgstore: renders the tree as one post conjunct; relation leaves reuse
`buildPredicateSQL` with a unique CTE prefix per leaf and become `EXISTS (...)`.
`OrderInteger` renders the propmatch rule in SQL: a JSON number whose text is an
int64 literal, else its float8 value when integral and in range (converted
through int8, never through numeric, which rounds to 15 digits); a JSON string
matching `^[+-]?[0-9]+$` in int64 range.
- sqlitestore: renders the tree and `OrderString`; a query with an
`OrderInteger` leaf marks the builder unsafe and runs on graphquerynaive, as
unsafe names already do. sqlite performance does not drive the design.
- Conformance tests for each node and kind (unset, empty string, list,
number-as-string, off-type, NOT over an absent key, relation under OR and NOT).
`RunGraphDifferential` generates random `Where` trees with ordered leaves of
both kinds and relation leaves.

### Lowering (`internal/queryplan`, `internal/predicate`)

- `predicate.Program.ConditionTree(spec)` returns the boolean structure of a
bool program: and, or, not, and the same leaves `ConditionConjunction` admits,
plus ordered comparisons of a record attribute against a string or integer
constant (operands may be in either order). A value-selecting `and`/`or` is not
boolean structure and declines.
- `queryplan.LowerCondition` returns one `ConditionLowering`: the plain
top-level conjuncts go to `Props` and `Traversals`, the rest to `Where` (leaves:
property predicates and bound traversal terms). Every caller handles `Where`, so
an `or` never widens to the whole type (RR-I92MF14). Trees over `MaxCondDepth`,
`MaxCondLeaves` or `MaxCondRelations`, and comparisons on rollup properties,
decline.
  - ordered on a string-shaped, non-date, non-integer property: `OrderString`.
  - ordered on an integer property: `OrderInteger`.
  - `==` and `~=` on an integer property: `x >= v and x <= v`, and its NOT. Both
agree with Go on nil (`nil == v` false, `nil ~= v` true).
  - anything else (decimal, boolean, date, datetime, list ordered) declines.
- Consumers: `appbuild.lowerQueryCondition` gates each traversal leaf and
builds `frag.Where`; a denied traversal becomes false and the tree is simplified
(a constant-false tree is the existing `empty` outcome). `listPlan.narrowed`
ANDs `Where`. MCP `list_entities` passes the tree on
`visibility.PageQuery.Where`, and the page reader gates its traversal leaves the
same way and checks field visibility for every property in it.

### Hidden fields

The rule stays: a condition is pushed only when every property it reads is
visible to the principal on every row (`FieldVisibleOnEveryRow`), so the stored
row and the redacted row give the same answer. `LowerScope` and the `visible:`
field scopes are not touched. Pushing over a field hidden on some rows by
rewriting each leaf as `(visible AND leaf) OR (NOT visible AND leaf(nil))`
against the compiled field scope is TKT-FKE3VP3.

### Fallbacks

`listPage.filterInGo` (MCP) and `desktopnotify` keep their Go path for
conditions that still do not lower; their allowlist entries stay. MCP now pushes
`or`, `not` and ordered comparisons, so its Go path shrinks to hidden fields,
dates, decimals and refused traversals. Removing the paths is TKT-9BKVQ9I and
TKT-I3MGCRT; refusing unlowerable conditions outright is TKT-W0DYCEC.
