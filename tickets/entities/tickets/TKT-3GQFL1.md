---
id: TKT-3GQFL1
type: ticket
title: Per-type columns and parent columns for the nested view section
kind: enhancement
priority: high
effort: m
status: review
---

## Description

`display: nested` (TKT-MJKZQ3) applies ONE flat `columns:` list to every row at
both levels. Two consequences, both reported from real use:

1. **A heterogeneous `children:` relation cannot be configured at all** — not
merely degraded. It fails config load.
2. **The parent level has no way to declare its own columns**, so an epic row
shows whatever the child columns happen to mean for it.

## 1. Heterogeneous children do not load

`has-task: {from: [epic], to: [task, bug]}` is ordinary rela. Measured on the
real `tickets/schema.yaml`: **15 of 47 relations declare more than one `to:`
type** (`depends-on`, `caused-by`, `verifies`, `informs`, …). This is a third of
the schema, not an edge case.

Today that config is REFUSED:

```
✗ view "project": section[1] column[2] property "due" not in entity "epic"
```

Root cause: `determineTargetType`
(`internal/dataentryconfig/validate.go:1564-1591`) returns `""` when a relation
has several `to:` types, so `nestedChildDef` resolves no child type, so column
validation falls back to checking against the PARENT only — and rejects any
column that exists on a child type.

Even with validation relaxed, the render would still be wrong: one column list
applied to mixed types means a column meaningful for `task` (`due`) renders
blank on every `bug` row, with no way to show `severity` instead.

## 2. The parent has no columns of its own

`columns:` is applied to parent rows and child rows alike (`buildSectionRow` is
called with the same list for both, in `internal/dataentry/sections_nested.go`).
An epic and a task rarely want the same columns, and the current demo only looks
right because the two types happen to share `status` and `owner`.

## Config shape (decided)

Two keys, one per LEVEL. Each is a map keyed by entity type, so a level can also
cover a heterogeneous relation. No flat `columns:` fallback.

```yaml
- heading: Epics
  source: epics
  display: nested
  children: work
  parent_columns:
    epic:
      - {property: status}
      - {property: owner}
  child_columns:
    task:
      - {property: status}
      - {property: due}
    bug:
      - {property: status}
      - {property: severity}
```

Decisions taken (no longer open):

- **Split by LEVEL first, then by type.** A single type-keyed map cannot
express the case that motivates this: a parent and a child of the SAME type
wanting different columns. That is not hypothetical — a self-referential
containment (`project has-project`, `concept-depends-on`, `refines`,
`related-idea` in the real schema) puts one type at both levels, and a group row
legitimately shows a rollup-ish summary where its children show detail. Level is
the primary axis; type is the secondary one.
- **Each level is a map, not a list**, so a heterogeneous relation is
configurable at whichever level it appears. `children: work` reaching both
`task` and `bug` is the driving case, but `source:` can be heterogeneous too.
- **No fallback list.** A flat list plus per-level maps means two ways to say
the same thing and a precedence rule to remember. A type absent from its level's
map renders title and id only — the gantt's "absent means default, not error"
rule (`internal/dataentryconfig/config.go:989`), not a silent failure.
- **`columns:` is not used by `display: nested` at all.** It stays a list for
`table` and every other mode, untouched. A `columns:` on a nested section is a
load error pointing at `parent_columns`/`child_columns` — the shape TKT-MJKZQ3
shipped is unreleased, so this migrates nothing.

## Validation

`nestedChildDef` must resolve the FULL SET of possible child types rather than a
single one, so a column can be validated against the type it is declared for.
`determineTargetType` returning `""` is correct and should not change; the
caller needs the `to:` list instead.

A column declared for a type that the `children:` relation cannot produce should
be a load error — that is a typo, and the current three-way ambiguity
(RR-HQMQJB) collapses once the type is named explicitly.

## Acceptance criteria

1. A `children:` relation with several `to:` types loads and renders.
2. Columns can be declared per child type; a row renders the columns for ITS type.
3. The parent level can declare columns independent of the children.
4. A type with no declared columns still renders (title and id), rather than erroring or vanishing.
5. A column naming a property absent from the type it is declared for is a load error.
6. A parent and a child of the SAME entity type can show different columns.
7. `columns:` on a nested section is a load error naming `parent_columns` / `child_columns`; `columns:` is unchanged for every other display mode.

## Prior art

- `internal/dataentryconfig/config.go:989` — gantt `sources: map[string]GanttSource`, the precedent for per-type config and for "absent means default, not error".
- `internal/dataentry/sections_nested.go` — `buildSectionRow` / `fillPropertyCell`, where the column list is applied.
- `internal/dataentryconfig/validate.go:1564` — `determineTargetType`, and `nestedChildDef` just below it.
