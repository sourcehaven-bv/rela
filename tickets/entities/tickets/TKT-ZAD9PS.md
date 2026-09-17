---
id: TKT-ZAD9PS
type: ticket
title: Per-parent rollup bar on a nested view section
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

Add a `rollup:` key to a `display: nested` view section: a per-parent bar
showing the **distribution of that parent's children across a named enum
property**. A project's epics each get a bar over the child task status enum.

Split out of TKT-MJKZQ3, which delivers the nested section itself. The rollup
was deferred because the nested view was the actual request and the rollup
carries all of the feature's ACL and validation risk.

## Semantics (settled, do not re-litigate)

The rollup is a **group-by, not a progress metric**. rela attaches no meaning to
the values — there is no done/ongoing/blocked vocabulary.

- Segments come from `metamodel.CustomType.Values`, in declared order.
- Segment names come from `CustomType.Labels`.
- Colours are assigned frontend-side from the existing `--badge-*` tokens in enum
order. The metamodel has no per-value colour, and `KanbanColumn` carries
`value`/`label`/`icon` with none either
(`internal/dataentryconfig/config.go:784-788`).

This generalises for free: `rollup: priority` works identically. It also avoids
inventing a terminal-state concept — the metamodel has `Transitions` and
`Initial` but no terminal/final marker, so "done" is genuinely not inferable.

## The two non-obvious problems

Both were found in design review of TKT-MJKZQ3 and are the reason this is its
own ticket rather than a small addition.

**1. Redacted values must still be counted (was RR-P751MM).**
`visibility.Redact` **removes** a hidden property from `Properties` and records
its name in `e.Redacted` (`internal/visibility/policyreader.go:242-252`). So a
child whose rollup property is `visible:`-restricted arrives with *no value*,
indistinguishable from genuinely unset.

If the fold skips valueless children, the segment counts stop summing to the
visible child count — and that arithmetic gap is itself an inference channel
revealing that a restricted property exists on that child. Fold them into an
explicit bucket (rendered neutrally) and assert `sum(segments) == len(visible
children)` across fixtures covering: redacted value, genuinely unset, and a
value absent from the enum.

**2. `rollup:` cannot always be validated against the child type (was
RR-HQMQJB).** `determineTargetType`
(`internal/dataentryconfig/validate.go:1564-1591`) returns `""` for any relation
with multiple `to:` types — legal config, and common in the real schema
(`depends-on`, `verifies`, `caused-by`). So validation is three-way:

| Case | Action |
| --- | --- |
| `children:` names an unknown bucket | hard error |
| known bucket, type determinable | validate the property is an enum |
| known bucket, type undeterminable | cannot validate — warn only |

Mirror the existing `ambiguousWidgetSource`/`widgetSectionDef` split
(`validate.go:1146-1200`), which exists for precisely this distinction.

## Implementation notes

- **Use `GetValidEnumValues` (`validate.go:779`) and nothing else.** Three
non-equivalent enum resolvers exist (that one, `enumValues` in
`validate_caldav.go:659`, `widgetPropertyHasValues` at :204) and they disagree
on an inline `values:` on a non-`enum` property. Kanban's `column_property`
check (`validate.go:1626-1645`) is the precedent to copy verbatim: resolve, then
`len(validValues) == 0` → "must be an enum type" error.
- **Values + Labels together**: no single metamodel accessor exists.
`resolvePropertyValues` (`internal/dataentry/helpers.go:269-277`) gives values
only, inline-first; `toV1PropertyDef` (`api_v1.go:75-95`) gives values + labels
with the **opposite** precedence (custom type wins, inline labels on a
custom-typed property deliberately ignored). Pick `toV1PropertyDef`'s rule or
the rollup labels will disagree with the rest of the API. Labels are not
currently on the section wire (`frontend/src/api/views.ts:10` carries `values?`
only).
- **Gating is structural, keep it that way.** `PolicyReader.Filter` calls
`r.redacted()` on each surviving row (`policyreader.go:88-93`), so
`viewResult.Collections` members are already row-gated AND field-redacted before
any section builder runs (`internal/dataentry/views.go:96-99`). Fold over the
filtered collection, never the store, and gate-before-fold holds by construction
per the `CLAUDE.md` aggregate rule.
- The rollup is **per-principal** and must never be cached across principals.

## Acceptance criteria

1. A `display: nested` section with `rollup:` renders a per-parent distribution bar.
2. Segments follow the enum's declared order and use its labels; no value is treated as special.
3. `sum(segment counts) == len(visible children)` for every parent, including redacted, unset, and out-of-enum values.
4. A child the principal may not read contributes to no segment.
5. Validation follows the three-way policy above; a non-enum `rollup:` property is a load error.

## Out of scope

- Weighted rollups (by estimate rather than child count).
- A section-level total across parents. Note this would reintroduce gantt's
multi-parent double-counting problem, which TKT-MJKZQ3 avoids precisely because
its rollup is flat and per-parent.
- Per-value colours in the metamodel.
