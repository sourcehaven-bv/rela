---
id: TKT-9OFGH4
type: ticket
title: 'Section sort: plus one declared order per enum, on every sort path'
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Two related defects, one feature.

The **defect**: rela has two in-Go sorters that disagree about enum properties.
A dashboard card or a search sorts `status` in declared (schema) order; a list
view or `/api/v1/entities?sort=status` sorts it alphabetically. Same schema,
same sort key, two answers, both live today.

The **feature**: `ViewSection` has no `sort:` key at all, so a `display: nested`
section renders its children in whatever order traversal produced.

This ticket unifies the sorters, then adds `sort:` to view sections on top of
the now-correct comparison rule.

Scope decision (2026-09-18): **one order per enum**, taken from the schema's
declared `values:`. A per-use-site `values:` override was considered and dropped
— see "Rejected: per-site ordering" below, which records the measurements,
because the reasoning is not recoverable from the code.

## Correction: declared-order enum sort already exists

The previous version of this description said enum sort ordered alphabetically
everywhere and had to be built. That was wrong, and anyone planning from it
would have rebuilt working code.

`internal/filter/sort.go` already implements it: `buildEnumIndex` (`:65`) maps
value → declared position, reading `propDef.Values` for an inline enum **or**
`meta.Types[...].Values` for a custom type; `compareEnums` (`:88`) compares by
that index. `compareByPropDef` (`:333`) also routes date, integer and boolean
properties to type-aware comparisons.

Verified by probe rather than by reading:

```
declared order: todo, doing, blocked, done
sorted output:  todo  doing  blocked  done  legacy
```

So the work is not "add declared-order sorting". It is "make the other sorter
agree, and push the same order into SQL".

## The two sorters

| Path | Sorter | Sorts `status` |
| --- | --- | --- |
| Search, dashboard cards (`queryservice.go:306`) | `filter.SortMulti` (`sort.go:181`) | declared order |
| `/api/v1/entities?sort=`, list views, export (`api_v1.go:2214`) | `applyV1Sorting` | **alphabetical** |

`applyV1Sorting` formats each value with `fmt.Sprintf("%v", …)` and compares
byte-wise. Its comment defends this as matching `store.GraphQuery.OrderBy` so a
request reads the same whether or not it was pushed down — which is true, and is
exactly why fixing the Go side alone is not enough.

Five sort implementations exist in total: `filter.SortMulti`, `applyV1Sorting`,
`graphquerynaive.Order`, pgstore's `ORDER BY`, and `filter.Sort` (CLI).

## Approach

1. **One query-sort comparator, defined by what SQL can express.** Both
`applyV1Sorting` and `filter.SortMulti` use it: byte order on the stored string
form, enum rank where declared values exist, nulls last ascending / first
descending, id ascending as the tiebreak in both directions.

Scope decision (user, 2026-09-20): **sorting stays in SQL**, because otherwise
paging does not work properly and loading a whole type is too much overhead.
Sort semantics become whatever sqlite/postgres can support, and the Go
comparator conforms to SQL rather than the reverse.

This replaces an earlier plan to simply delegate to `filter.SortMulti`. Design
review showed the two sorters differ on strings, dates, ids, lists, undeclared
properties and the meaning of descending, so delegating would have swapped one
divergence for several. Accepted consequence: string sorts and `sort=id` become
byte order, which moves 99% of positions across this repo's own 4,308 ticket
titles. Release note required.
2. **`queryplan` emits a `CASE` rank** for an enum sort key, so the pushed query
orders by declared position.
3. **The enum's declared values participate in `listIndexName`'s hash** — see
"The stale-index hazard" below. Non-negotiable.
4. **pgstore emits the matching `CASE`** in `ORDER BY` and in
`createListIndexDDL` (`derivedschema.go:115`).
5. **EXPLAIN test** proving the new shape uses its generated index, as
`CLAUDE.md` requires for any newly supported SQL shape.
6. **`Sort []SortSpec` on `ViewSection`** (`config.go:1411`), reusing the
existing type so the YAML shape matches `List.Sort` (`:649`) and
`DashboardCard.Sort` (`:1339`).
7. **Differential test**: same data, same spec, Go path and pushed path agree.

`metamodel.SortSpec` needs **no change** — no new field, no metamodel access.
`filter.SortMulti` already takes `*metamodel.Metamodel` as a parameter. That
closes the third open question below.

Note `store.OrderSpec` DOES change: it grows an optional ordered-value list so
the enum rank reaches both pgstore's `CASE` and `graphquerynaive`. Those values
must also enter `listIndexName`'s hash and `StaticIndexSpecs`' dedup key.

## Measurements (Postgres 18, 200k rows, rela's real index shape)

Against `createListIndexDDL`'s shape `(type, ((properties->>'status') COLLATE
"C"), id) WHERE face = ''`:

| Approach | Plan | Buffers | Time |
| --- | --- | --- | --- |
| Byte-wise (today) | Index Scan | 5 | 0.07ms |
| `CASE` rank, **matching** `CASE` index | Index Scan | 5 | 0.08ms |
| `CASE` rank, ASC | Index Scan | 4 | 0.06ms |
| `CASE` rank, DESC | Index Scan **Backward**, same index | 5 | 0.03ms |
| `CASE` rank, byte-wise index | Parallel Seq Scan | 2,398 | 45.9ms |
| `array_position(ARRAY[…], col)` | Parallel Seq Scan | 2,531 | 38.0ms |
| Join to a rank lookup table | Hash Join + Seq Scan | 2,370 | 43.3ms |
| Native pg enum column + index | Index **Only** Scan | 6 | 0.06ms |

Three things follow.

**A `CASE` rank is indexable**, and one index serves both directions via a
backward scan. So pushdown is kept, at full speed, with no new pushdown shape.

**The two most-recommended web idioms are the worst options.** `array_position`
and the lookup join both compute the rank per row, so neither can be indexed;
`array_position` is additionally O(N) per call. Do not reach for either.

**Native pg enums win on speed and are still unusable here.** Postgres sorts
them by declaration order for free, but per the docs the sort order "cannot be
changed, short of dropping and re-creating the enum type" — and rela's enum
values live in operator-edited `schema.yaml` while properties are stored in
`jsonb`, not typed columns. Adopting them means a real column per enum property
and a type rewrite per schema edit.

## The stale-index hazard

Inserting a value into an enum shifts every later rank. Measured: the query
silently drops to a Parallel Seq Scan (2,398 buffers) while the old index
remains present and healthy-looking.

`derivedschema.go:69-90` already solves this class of bug. `uniqueIndexShape`
participates in the index-name hash so that changing a definition **renames**
the index; the reconciler then creates the correct one and drops the stale one,
"because a name it no longer computes is by definition no longer desired". That
pattern exists because of BUG-HC6I2T.

So `listIndexName` (`:95`) must hash the enum's declared values for any enum
sort key. Reorder `values:` in `schema.yaml` and the index is rebuilt
automatically. Omitting this ships a silent 650× regression triggered by an
unrelated schema edit.

## Rejected: per-site ordering

A per-use-site `values:` list (`sort: [{property: status, values: [...]}]`) was
specced and rejected on measurement.

An expression index matches only the literal expression, so each distinct site
order needs its own index. Measured: the same index serving one order falls to a
Parallel Seq Scan (2,398 buffers, 46.1ms) for a different order over the same
property. Three views sorting `status` three ways means three expression indexes
on the entities table, each one write amplification on every entity write, and
each rebuilt whenever an operator edits a `values:` list.

No prior art solves per-site ordering in SQL. The rank has to be either baked
into the index (one index per site) or computed per row (seq scan); there is no
third option in the `ORDER BY`.

If per-site order is ever genuinely needed, the design that works is to order by
raw value in SQL and reorder in Go, paging via one bounded indexed read per enum
value (measured at 5-7 buffers per read regardless of selectivity, including a
value holding 9 of 200,000 rows). That is a new pushdown shape and should be its
own ticket with a real use case behind it.

## Closed questions

**Out-of-enum values already sort last.** `compareEnums` (`sort.go:101-107`):
known beats unknown, two unknowns fall back to byte order. Verified by probe
(`legacy` sorted after all four declared values). No decision needed; document
it.

**`SortSpec` is the right home.** It needs no change at all, because
`filter.SortMulti` already receives the metamodel.

## Open question for the implementer

**Does this reorder existing lists on upgrade?** Yes — every enum-sorted list
view changes order. I read that as a fix rather than a regression: dashboards
and search have sorted by declared order all along, so alphabetical list views
are the outlier. It is still a visible behaviour change and belongs in release
notes.

A config opt-in was considered and is hard to justify: the flag would mean "sort
my enum wrongly", and it would keep two sorters alive, which is the defect.

## Also in scope: `sort:` on a view section

`ViewSection` (`config.go:1411`) has no `Sort` field and nothing in the view
path sorts, so collection order is whatever traversal and the store produced.
This ticket adds:

- `parent_sort:` and `child_sort:` on a nested `ViewSection`, plain `sort:` on a
flat one. NOT one ambiguous `sort:`: a nested section holds two collections at
two levels, and the existing `ParentColumns`/`ChildColumns` pair already solved
that problem the same way.
- A sort step for section collections, applied **before any cap**, so the
surviving rows are the top-sorted ones rather than an arbitrary prefix.
- Sort properties validated against the source type the way `columns:` already
are. `determineTargetType` (`validate.go:1805`) returns `""` for a multi-`to:`
relation, so validate where the type is statically known and skip where it is
not.

Which displays honour `sort:` is a decision for this ticket; the flat displays
(`list`, `table`, `cards`) are the obvious candidates alongside `nested`.

## Non-enum custom types

`buildEnumIndex` reads `meta.Types[...].Values`, so a custom type declaring
`values:` gets declared order too, and `queryplan.StringShaped`
(`queryplan.go:119`) already admits custom types. The SQL side must rank those
identically or the two paths diverge for exactly the properties the Go side
handles most quietly. Cover both in the differential test.

## Relationship to TKT-MJKZQ3

`display: nested` (TKT-MJKZQ3, shipped) is the motivating consumer: the original
request was children sorted by status with completed last, then due date
ascending. That ticket ships with unsorted children and documents the gap, so it
is not blocked, but it is not complete for the use case until this lands.

TKT-ZAD9PS (per-parent rollup bar) uses declared enum order for its segments. If
it ships first, a section sorted by status would show a bar in declared order
and rows in alphabetical order. Doing this ticket first avoids that
inconsistency ever being visible.
