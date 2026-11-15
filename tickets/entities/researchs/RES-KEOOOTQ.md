---
id: RES-KEOOOTQ
type: research
title: How should rela import external structured data through declarative mappings?
summary: New plan/apply package with one small path language over a normalized tree (JSON/XML/YAML/CSV), key-property identity, pure Lua hook, rela import --mapping
status: done
---

## Problem

How should rela import external structured data (JSON, XML, YAML, CSV; OSCAL
catalogs as the motivating case) into an operator's own entity types, with
stable identity, relations by key, an optional script hook and a reviewable diff
before apply?

## Context

What already exists:

- `internal/importer` + `rela import`: imports rela's *native* shape (`entities:`/`relations:`
lists with ids) and writes straight to `store.Store` (no audit, no automations).
No mapping.
- `internal/ingestspec` + `internal/ingest`: per-record find / create_if_missing / then mapping
for webhooks and feed subscriptions. One record = one entity, find scans the
type per record, no relations, no diff, writes immediately. Its `{{ref}}`
interpolation is reusable in spirit.
- `internal/remotesync`: rela-to-rela pull with ledger, gone guard, per-field merge. Bound to a
remote rela API, not to files.
- `internal/safexml`: hardened XML decoder (no DTD/XXE, entity expansion only for the fixed HTML
map, caps on bytes/depth/elements/attrs). Reuse for XML input.
- `internal/lua.NewSandbox` + `GoToLuaValue` / `ToGo`: a bare safe-library Lua state with no rela
bindings (used by `rela test`). The datamigration Lua step shows the "pure
transform, patch in, patch out, the engine applies" pattern.
- `external_ref` property type: (system, id) unique per face, writable only by scripts,
migrations and imports (`WriteExternalRefs`). A natural stable key for imported
rows.
- Constraints: all writes through `entitymanager.Manager` (PatchEntity for partial updates),
reads through the visibility reader, config is not secret, no external commands.

## Options

### Selector language

1. **JSONPath / XPath libraries per format.** Familiar, but two languages, two dependencies,
different semantics per format and filter expressions with their own evaluators.
Effort M.
2. **jq (gojq).** Very expressive, but a full language: unbounded work, hard to validate at load,
heavy for an operator file. Effort M.
3. **One small path language over one normalized tree.** Decode every format into the same node
tree (objects, arrays, scalars; XML element = object with `@attr`, child names,
`#text`). Steps: `name`, `*`, `..name` (descendants), `^` (enclosing object),
`[field=value]` filter, arrays flatten automatically. Parsed and checked at
load, linear evaluation, no functions. Covers OSCAL (`catalog..controls`,
`props[name=label].value`, `^.id`). Effort S-M.

### Identity

1. Explicit rela id template per type. Stable, but forces id schemes on projects that use
generated prefixed ids.
2. **Key property** per mapping (a string property, typically `unique:`, or an `external_ref`).
New rows get the project's generated id; re-import matches on the key. Optional
id template for projects that want deterministic ids.

### Engine placement

1. Extend `ingest` (per record, write-as-you-go). No diff, no cross-record relations.
2. **New package that plans then applies.** Decode, map every record, load current state once
(one list per type, one relation list per type), compute a plan (creates,
property updates, relation adds/removes, missing rows, per-record errors), print
it; apply executes the plan through the manager. Dry run = plan only.

### Lua hook

1. Full writer runtime (`script.NewWriterRuntime`). Gives graph writes, which makes a dry run
dishonest and duplicates the engine's writes.
2. **Pure transform in `lua.NewSandbox`.** `function(record, draft) return draft end`; no rela
bindings, no http, no io/os; nil skips the record. Same shape as the migration
Lua step.

## Recommendation

Path language option 3, key-property identity (with optional id template), a new
plan/apply package `internal/mapimport`, pure-transform Lua hook, exposed as
`rela import --mapping <name>` reading `imports/<name>.yaml` and validated by
`rela validate`. Missing rows and stale relations are reported; relation removal
only with `prune: true`; entities are never deleted. Accepted trade-offs: the
path language is deliberately small (no arithmetic, no functions); faced entity
types are refused for now; the hook cannot read the graph.
