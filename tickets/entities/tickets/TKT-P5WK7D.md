---
id: TKT-P5WK7D
type: ticket
title: 'predicatefns takes a consumer-side schema interface returning predicate types, not metamodel defs'
kind: refactor
priority: low
effort: m
status: backlog
---

## Goal

`internal/predicatefns` imports `internal/metamodel` to answer one question per
property: what `predicate.Type` is it, and how does a raw frontmatter value
bind to a `predicate.Value`? Express that as a consumer-side interface so the
package stops depending on the metamodel aggregate.

## What it actually uses

The whole surface, measured:

- `meta.GetEntityDef` (x3) — only ever for `def.Properties`
- `meta.Types[name]` (x1) — only to ask "is this a declared custom type?"
- per property: `Type`, `List`, `GetDateFormat()`

Nothing else. But note the *values* crossing the boundary are
`metamodel.PropertyDef` and the `PropertyType*` constants, so an interface that
hands back schema fields narrows the coupling without removing the import.

## The cut that does remove it

Return the **decision**, not the ingredients. The mapping
`PropertyDef -> predicate.Type` is a pure function, and `predicate` is already
metamodel-free (arch-fenced), so:

```go
type Schema interface {
    // FieldType is the predicate type of entityType.prop. ok=false means
    // unmodellable or unknown, and the caller omits it — a reference then
    // fails at compile rather than evaluating against a wrong type.
    FieldType(entityType, prop string) (predicate.Type, bool)
    Fields(entityType string) (map[string]predicate.Type, bool)
    BindField(entityType, prop string, raw any) predicate.Value
}
```

`List` collapses into the implementation (it wraps in `ListType`), the date
layout collapses into `DateTypeWithLayout`, and the `Type` switch goes with
them.

`BindField` must be in the same interface as `FieldType`: `Compile` needs
types, `Matches` needs values, and the two must agree or a value's runtime type
contradicts its compile-time type. Today those are two switches in separate
files (`ScalarType` in env.go, `coerceScalar` in bind.go) kept in agreement by
hand — `affordances/bindings.go:180` documents the invariant in a comment
rather than enforcing it. Colocating them is a improvement on its own.

## Why the type info is worth preserving exactly

It is what turns a config typo into a startup failure instead of a silent
no-match:

1. `entity.duedate` for `due_date` is a `CompileError` ("unknown attribute"),
   not an expression that evaluates nil forever.
2. `entity.due <= today()` compares dates, not strings — string comparison
   happens to work for ISO and breaks for every other layout, which is why the
   format is per-property.
3. An unmodellable property (`file`) is omitted, so referencing it fails at
   compile rather than lying at eval.

Any subset that loses one of these trades a loud failure for a quiet one.

## Migration hazard: the date fallback chain

`coerceDate` does not use `GetDateFormat()` directly — it calls
`metamodel.ParseDateValue(v, prop)`, which is `GetDateFormat()` **plus a
hardcoded fallback list** (RFC3339, ISO-with-Z, ISO-without-timezone,
date-only).

Move the layout without the fallbacks and a `date` property storing
`2026-03-01T10:00:00Z` stops parsing — it binds `Nil`, so the condition
silently stops matching. No error. The fallback chain has to travel with the
layout or become part of the injected contract; either way, pin it with a test
per fallback format.

## Do not regress RR-TBG91

`internal/affordances` imports `predicatefns.ScalarTypeForProp` on purpose, as
the single canonical adapter. Whatever implements `Schema` becomes that
adapter and `affordances` must use it — two copies of the mapping is the thing
RR-TBG91 exists to prevent.

## Not a blocker

[[TKT-N8XQ2R]] needs none of this: it keeps `predicatefns` behind a one-method
matcher supplied at the wiring site, so `internal/nextaction` never sees a
schema either way. Sequence this independently.

## Acceptance

- `internal/predicatefns` does not import `internal/metamodel` on the compile
  or bind path.
- `affordances` and `predicatefns` still share one type-mapping implementation.
- Date fallback formats are preserved, with a test per format.
- `predicatefns` tests can construct a schema fake without building a
  `*metamodel.Metamodel`.
