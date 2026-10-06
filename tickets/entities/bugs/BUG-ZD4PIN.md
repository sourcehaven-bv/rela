---
id: BUG-ZD4PIN
type: bug
title: Create sets a hardcoded status when the schema declares no default
description: 'Creating an entity without a status writes status: draft (or the first enum value) even when the type has no status property or declares no default.'
priority: high
effort: s
why1: entitymanager.buildCandidateEntity (and the importer) sets status to EntityDef.GetDefaultStatus whenever status is empty, without checking that the type declares a status property.
why2: 'GetDefaultStatus never returns empty: with no status property, or a legacy/string status type, it returns the literal draft\; with an enum and no declared default it returns the first value.'
why3: The function predates custom types, state machines and per-type schemas. It encoded the original fixed requirement/decision model, where every entity had a status and draft was the first state.
why4: When types became schema-driven, the fallbacks were kept as convenience defaults. No rule said that defaults must come from config, so the hardcoded value looked harmless. It also ignored a state machine's initial value, so a machine whose initial differs from its first value rejected every create without a status.
why5: There was no single place that resolves a declared default, and no test pinned that rela writes only values the operator configured. Template generation duplicated the same invent-a-value logic (first enum value, false, 0).
prevention: metamodel.DeclaredDefault is now the one resolver for declared defaults, used by create, import and template generation. Tests on every create path assert that an undeclared status stays absent, including a state machine whose initial value is not its first value.
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

## Description

When an entity is created without a `status`, rela fills one in even when the
operator's config declares no default:

- A type with no `status` property gets `status: draft`.
- A type whose `status` is an enum without a declared default gets the first
enum value.
- A `status` of the legacy `status` type, or of type `string`, gets `draft`.

This happens on every create path: CLI and Lua (`rela.create_entity`), the HTTP
API (`POST /api/v1/<type>`, used by web forms), MCP `create_entity`, and `rela
import`.

## Expected

rela never invents a value. A create without `status` sets one only when the
config declares it: the property's `default:`, else the named type's `initial:`
(state machine entry), else the named type's `default:`. Otherwise the property
stays unset, and `required:` validation reports it as usual.

## Behaviour change

Projects that relied on the implicit value must declare it in config, for
example `default: draft` on the status property or its type.

## Reproduction

```yaml
entities:
  note:
    label: Note
    id_type: sequential
    id_prefix: "NOTE-"
    properties:
      title: {type: string, required: true}
```

`rela create note -P title=hello` writes `status: draft` into `NOTE-001`.
