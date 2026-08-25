---
id: TKT-HFEKVN
type: ticket
title: Consolidate temporal parsing behind one definition
kind: refactor
priority: medium
effort: m
status: backlog
---

## Description

Three copies of the same temporal-layout list exist in the codebase, and a
fourth was nearly added:

| Site | What it does |
| --- | --- |
| `metamodel.ParseDateValue` (validation.go:642) | Declared property format, then 4 fallbacks |
| `predicate.parseDateLiteral` (dateparse.go:29) | Declared layout, then the same 4 fallbacks |
| `dataentry.parseTemporal` (helpers.go:78) | 5 layouts, **no** declared-format awareness |

The first two carry comments saying they mirror each other **by hand** —
`predicate`'s notes it "mirrors the fallback set
internal/metamodel.ParseDateValue accepts ... so a date literal that
internal/filter's --where would accept also coerces here — keeping predicate a
true superset of filter (RR-BNRMU)".

That invariant is real and load-bearing, and it is currently maintained by
someone remembering to edit two files. TKT-IG54YO added the third when
`compareValues` turned out to parse only `2006-01-02`, which silently excluded
every `datetime` row from a range-filtered query.

## Why it matters

The failure mode is invisible. A layout present in one list and missing from
another does not error — it changes which rows come back. The bug that prompted
this shipped unnoticed until a calendar over a `datetime` property rendered
completely empty.

The three sites also **disagree today**, which is the part worth acting on:

- `dataentry.parseTemporal` accepts `2006-01-02T15:04` (minute precision); the
other two do not.
- `dataentry.parseTemporal` ignores the property's declared `format:` entirely,
because `compareValues` receives two bare strings and has no `PropertyDef` in
scope. The other two try the declared layout first.

So a `date` property with a custom `format:` compares correctly under `--filter`
and incorrectly under a `filter[x][gte]` query param.

## Scope

- One exported parser owning "what is a temporal value" — layouts, the
declared-format-first rule, and the fallback order.
- The three call sites delegate to it. `predicate` keeps its compile-time-only
guarantee (RR-A3EZR: parsing never happens at Eval).
- A test that FAILS if the layout sets drift apart again, replacing the
hand-maintained mirror comments.

**Where it lives** needs deciding, and is the main design question. `metamodel`
is the natural owner (it defines property types and already exports
`ParseDateValue`), but `predicate` must not gain a dependency that breaks its
sandboxing, and `internal/dataentry` importing `metamodel` for this is fine
while the reverse is not. Check `just arch-lint` before committing to a home.

## Out of scope

**Property-type-aware comparison.** `compareValues` infers type from the string
shape, which is why `"2026"` compares as a number rather than a year. The
metamodel already knows `due_date` is a `date`; threading that through would
make comparison exact instead of heuristic. That is a larger change with its own
design, and consolidating the parsers is a prerequisite for it rather than a
substitute.

## Acceptance criteria

1. One parser is the single definition of the accepted temporal layouts.
2. `metamodel.ParseDateValue`, `predicate.parseDateLiteral` and
`dataentry.parseTemporal` all resolve through it; no layout list is duplicated.
3. A property's declared `format:` is honoured on **every** path, including
`filter[x][gte]` query params — the case that is wrong today.
4. A test fails if one site's accepted layouts diverge from another's.
5. `predicate` still parses only at compile time (RR-A3EZR).
6. Behaviour is otherwise unchanged: existing filter, predicate and validation
tests pass without modification.
