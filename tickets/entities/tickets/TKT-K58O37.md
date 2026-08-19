---
id: TKT-K58O37
type: ticket
title: 'YAML-backed config store: AST + diff-apply write path for projected config'
kind: enhancement
priority: medium
effort: xl
status: backlog
---

## Problem

Make the projected config **writable** against YAML files, so edits made through
the graph (SPA, API, MCP) land back in `schema.yaml` / `data-entry.yaml`.

## Approach

The store keeps the parsed `yaml.Node` AST alongside the projection. On write it
diffs the incoming projection against the previous one and applies that diff to
the AST, then re-emits. Untouched nodes stay byte-identical.

This is the same shape as `internal/migration/yaml_util.go` (`GetMapValue` /
`SetMapValue` / `SetMapNode`), which fourteen migrations already ship on, plus
`internal/metamodel/rename.go` and `internal/schema/cleanup.go`. Not novel — but
more work at the delete/reorder end than the existing helpers cover.

## Proven by the spike

Editing `label` on `entity-type/ticket` through the SPA round-tripped to
`schema.yaml` as a **1-line diff** on a 2171-line file, comments and formatting
intact, still passing `rela validate`.

**Critical detail:** default `yaml.Marshal` re-emits at 4-space indent, turning
a 1-line change into a **4314-line diff**. `yaml.NewEncoder` + `SetIndent(2)`
was the entire fix. Without it the approach looks unusable.

## NOT proven — the real work

The spike round-tripped **four scalar fields on one entity type**. Untested:

- **adding** a property / entity type / relation type
- **deleting** anything
- **reordering** (properties, form fields, list columns)
- editing forms, lists, fields, validations, automations
- renames

Deletion and reordering are the cases to de-risk first. `yaml.v3` attaches
comments positionally (`HeadComment`/`LineComment`/`FootComment`), so removing
or moving a node can orphan or misattach them. **If comments project as data
(TKT-IB5C8S), this problem largely dissolves** — the comment travels with its
owning entity by construction.

> "Scalar update round-trips cleanly" is proven.
> "Bidirectional editing works" is not.

## Open questions (resolve when work starts)

- **Does comments-as-data actually remove the need for positional AST comment handling**, or is a hybrid needed (data for key-attached comments, AST fidelity for standalone banners)?
- **Is this a real `store.Store` implementation, or a narrower write-back service?** `store.Store` is ten embedded interfaces; a config store has no meaningful attachments and arguably no meaningful `Tx`. `storetest.Capabilities` already gates attachments — check what else needs gating, and whether the conformance suite is the right bar for a store whose entity space is constrained by its own meta-schema.
- **Reload atomicity:** a write that produces an invalid `schema.yaml` must not take the project down. Validate-before-persist is mandatory; decide whether the store or the caller owns that.
- **Index invalidation:** the spike hit stale `.rela/fsstore-index.json` three times, showing phantom duplicates (28 vs 24 entity types). A projection regenerating underneath a running store needs explicit invalidation.
- **Does `appbuild.SharedBase`'s no-mutation invariant** (one `*metamodel.Metamodel` pointer shared across tenants) block in-place schema writes, forcing a rebuild-and-reassemble path?

## Context

Findings `.ignored/schemaspike/FINDINGS.md` §4 (round-trip, proven vs unproven),
§5.6 (index staleness).
