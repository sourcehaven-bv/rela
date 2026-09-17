---
id: TKT-9OFGH4
type: ticket
title: 'Section sort: plus enum ordering by declared value order'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Sorting on an enum property currently orders **alphabetically**, because
comparison is byte-wise on the string form. A `status` column sorts `blocked,
doing, done, todo` — which is close to useless for a workflow enum, where the
operator means "in workflow order, completed last".

Make an enum property sort by its position in `CustomType.Values` (the declared
order) instead, so writing the enum in workflow order gives the right sort for
free.

## Where it stands today

`sort:` is already a multi-key list (`internal/dataentryconfig/config.go:587`
and `:1229`, aliasing `metamodel.SortSpec` at `internal/metamodel/sort.go:7`),
so `sort: [{property: status}, {property: due, direction: asc}]` is expressible.
What is missing is only the **comparison rule** for enum values.

The in-Go comparison is at `internal/dataentry/api_v1.go:2148-2172`
(`applyV1Sorting`), and its comment records that byte-wise was a deliberate
choice:

> Byte-wise on the string form, a row WITHOUT the property sorting as the largest value (last ascending, first descending), id as the final tiebreak — the same order a store pages by (`store.GraphQuery.OrderBy`), so a request served either way reads the same.

That agreement is the crux: the in-Go path and the store pushdown path must
produce the same order, or a list paged by the store diverges from one sorted in
Go.

## Why this is bigger than one view section

Declared-order sorting has to hold on **both** sides of the pushdown decision:

- **In Go** (`applyV1Sorting`, and the view/section sorters) — straightforward: map value → index.
- **Pushed into SQL** (`internal/dataentry/listpushdown.go`, `store.GraphQuery.OrderBy`) — needs an ORDER BY that ranks by declared position rather than text. A `CASE` expression or a join against a values table, per backend, and `listpushdown.go` currently declines non-STRING-shaped properties precisely because byte order ≠ semantic order.

If only one side changes, a list reads differently depending on whether its
request happened to be pushdown-eligible. That is the defect to avoid, and it is
why this is its own ticket rather than a clause in another.

Also affects the derived static-query indexes: per `CLAUDE.md`, pushdown and
index inference must use the same `internal/queryplan` eligibility decision, and
an EXPLAIN test must prove any newly supported SQL shape uses its generated
index.

## Scope

In scope:

- Enum-aware comparison in the in-Go sorters.
- The matching SQL ORDER BY shape, on every backend that supports pushdown.
- A decision on whether `queryplan` eligibility widens to admit enum sorts, with the EXPLAIN test if it does.
- Consistency test proving pushdown and in-Go paths agree on an enum sort.

Out of scope unless it falls out cheaply:

- A per-view `values:` override naming an explicit sequence. Declared order is the default worth having; an override is only needed when a view wants to contradict the schema.
- Non-enum semantic ordering (e.g. natural-number sort on strings).

## Open questions

- Do rows whose value is absent from the enum (legacy data, a value removed from the schema) sort first, last, or alphabetically among themselves?
- Does this change the default for existing configs, or require opting in? Changing it silently reorders every list sorted on an enum — visible, arguably a fix, but a behaviour change on upgrade.
- Is `metamodel.SortSpec` the right home for the enum awareness, given it currently holds only property + direction and has no metamodel access?

## Also in scope: `sort:` on a view section

`ViewSection` has **no `Sort` field**
(`internal/dataentryconfig/config.go:1301-1311`) and nothing in the view path
sorts — no `sort.` call exists in `internal/dataentry/sections.go` or
`views.go`, so collection order is whatever traversal and the store produced.
The `sort: []SortSpec` fields that exist belong to `List` (`config.go:587`) and
dashboard cards (`config.go:1229`).

Section sorting was originally scoped into TKT-MJKZQ3 and then deferred here, on
the reasoning that building the plumbing against a comparison rule that is about
to change is wasted work: a `status` sort would ship ordering `blocked, doing,
done, todo`, which is not what any operator asking for it means.

So this ticket adds both halves together:

- `Sort []SortSpec` on `ViewSection`, reusing the existing `SortSpec` type so the
YAML shape matches lists and dashboards.
- A sort step for section collections, applied before any cap so the rows that
survive are the top-sorted ones rather than an arbitrary prefix.
- Sort properties validated against the source type the way `columns:` already
are (`validate.go:1519-1535`). Note `determineTargetType` (:1564-1591) returns
`""` for a multi-`to:` relation, so the type is not always statically known —
validate where it is, skip where it is not.
- The declared-order enum comparison below, so the first thing an operator writes
(`sort: [{property: status}]`) does the expected thing.

Whether every section display honours `sort:` or only some is a decision for
this ticket; the flat displays (`list`, `table`, `cards`) are the obvious
candidates alongside `nested`.

## Relationship to TKT-MJKZQ3

`display: nested` (TKT-MJKZQ3) is the motivating consumer: the original request
was children sorted by status with completed last, then due date ascending. That
ticket now ships with **unsorted** children and documents the gap, so it is not
blocked — but it is also not complete for the use case until this lands.

Ordering note for whoever picks this up: TKT-ZAD9PS (per-parent rollup bar) uses
declared enum order for its segments. If that ships before this ticket, a
section sorted by status would show a bar in declared order and rows in
alphabetical order. Doing this ticket first avoids that inconsistency ever being
visible.
