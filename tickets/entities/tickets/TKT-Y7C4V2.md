---
id: TKT-Y7C4V2
type: ticket
title: Close config-projection fidelity gaps (validations, automations, transforms, default_sort)
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Problem

The spike projected several constructs as **opaque YAML blobs** in entity
content, or skipped them entirely. Fidelity is a prerequisite for the lint being
usable, not a nice-to-have: §5.7 showed a lossy projection produces ~116
artifacts for every 1 real finding, which buries the signal.

## Gaps to close

| Construct | Spike behaviour | Note |
| --- | --- | --- |
| `validations` (120) | opaque blob | `ValidationRule.When`/`Then` are `[]string` — **more projectable than the spike assumed**; deferred by laziness, not necessity |
| `automations` (6) | opaque blob | nested `on:` / actions |
| `transforms` | not projected | `Cmd` XOR `Image` sum type — see below |
| `default_sort` | not projected | ordered list of specs on 18 entity types |
| form `steps:` | flattened into one field list | wizard structure lost |
| `includes` | not projected | no obvious graph representation |
| `filters`, `dashboard`, `documents`, `actions` | not attempted | |

## Sum types need no new metamodel feature

An earlier assumption that rela cannot express "exactly one of N shapes" was
**wrong**. `RelationDef.To` is `[]string`, so a tagged union is a heterogeneous
`to:` plus `max_outgoing: 1`:

```yaml
has-step-impl:
  from: [transform-step]
  to: [cmd-step, image-step]   # discriminant is the target's type
  min_outgoing: 1
  max_outgoing: 1
```

Confirmed in practice by `from-type`/`to-type`. No `one_of:` feature is
required.

## Open questions (resolve when work starts)

- **How deep should `When`/`Then` project?** As string properties (simple, greppable) or parsed into predicate-expression entities (queryable, but couples the projection to `internal/predicate`)? Start with strings; the parsed form is a later refinement.
- **Do automation actions become satellite entities** (`set`, `create_entity`, `create_relation` as typed entities under an `automation`), or stay opaque? Satellites make "which automations write to property X?" a graph query, which is valuable — but it is a large modelling job.
- **Is `includes` projectable at all**, or does it stay a documented non-goal? Composition across files may simply not be graph-shaped.
- **Does projecting form `steps:` require a `step` entity type** between form and field, and does that break the `has-field` ordering model?

## Context

Findings `.ignored/schemaspike/FINDINGS.md` §6 (what did not project), §5.4 (sum
types), §5.7 (why fidelity matters).
