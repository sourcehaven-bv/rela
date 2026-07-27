---
id: BUG-9Z20WH
type: bug
title: 'executeView traversal is ungated: hidden intermediaries leak reachable descendants in _views API, entity-detail sections, and view commands'
description: 'executeView''s multi-pass traversal reads the raw store with no read-gate; the _views API, entity-detail sections, and view commands all gate only the entry entity, then serve the ungated traversal closure. A hidden intermediary H on Entry->H->V leaks the visible descendant V to a principal who could never read it directly. Fix: source-gate traverseViewOnce''s GetEntity + the entry through the visibility seam so hidden targets never enter the working set.'
priority: high
effort: l
why1: executeView's multi-pass traversal (traverseViewOnce -> st.GetEntity on the raw store) reads entities with no ACL read-gate, and all three consumers serve the traversal result to the user after gating only the entry entity.
why2: The traversal predates internal/visibility (DEC-ZBI39P). When the read-side ACL was decomposed into visibility decorators, every other read-out path was wrapped, but this traversal reader was not — it was missed, not deliberately excluded.
why3: There was no structural enforcement that a new read-out reader must go through a visibility wrapper; the rule ('read-out paths go through visibility wrappers') is a CLAUDE.md convention checked by review, not by a lint or a type boundary, so a pre-existing reader that predates the rule stays invisible to it.
status: backlog
---

## Summary

`executeView`'s multi-pass traversal reads the **raw store** with no read-gate,
and it is served to users by three surfaces. All three gate only the **entry**
entity, then leak the ungated traversal closure. A hidden intermediary `H` on a
path `Entry → H → V` leaks the visible descendant `V` to a principal who could
never read it directly — a read-side confidentiality leak.

Found during TKT-2FDTJE's design review (RR-3T18K9 / RR-WAE2E4), which split it
out because it affects surfaces beyond commands.

## The leak (verified)

`traverseViewRecursive` (`views.go`) recurses through every immediate result via
the raw store, ungated:

```go
immediate := a.traverseViewOnce(ctx, sourceID, rule)  // st := a.store; st.GetEntity(targetID)
for _, e := range immediate {
    all = append(all, a.traverseViewRecursive(ctx, e.ID, rule, depth+1, maxDepth, visited)...)
}
```

On `Entry → H → V` with `H` hidden and `V` visible:
- a **source-gated** traversal stops at `H` → `{}`
- the **current** ungated traversal → `{V}`

The presence of `V` (reachable only through `H`) leaks that `H` exists and links
`Entry` to `V`. Secondary: `applyViewTraverse` runs `filterEntities(found,
rule.Where)`, which reads `e.Properties` of traversed-but-hidden entities, so
collection membership is a function of hidden entities' property values.

## The three affected surfaces

1. **`_views` API** (`api_v1.go:2609-2645`). `gateReadOrNotFound` gates the
entry, then `executeView` runs, then `buildSections(result)` → the wire
`ViewResponse` sent to the browser. The comment at `api_v1.go:2611` claims
"_views is an entity-read chokepoint just like GET /{plural}/{id}" — **it is
not**: `GET /{id}` gates the one entity; `_views` gates only the entry and
serves the traversal closure.
2. **Entity-detail side-panel sections** (`sections.go:341`). Same pattern via
a synthetic `ViewConfig`; `buildSections(result)` feeds the side panel.
3. **View commands** (`commands.go:436`). The surface whose planning found this.

## Fix

Route the traversal's entity reads through the visibility seam:

- `executeView`'s entry load (`a.store.GetEntity(entryID)`) → `getVisible`.
- `traverseViewOnce`'s `st.GetEntity(targetID)` → gated read, so a hidden
target is neither placed in a collection nor recursed through.

A hidden target must be dropped **before** recursion so it cannot serve as an
intermediary. Gate at the read and both the reachability and where-clause leaks
close, because hidden entities never enter the working set.

**Do NOT** filter the result post-hoc — RR-3T18K9 proved result-filtering ≠
source-gating (it leaves the reachability leak).

## Scope

**In scope:**

- Gate `executeView` entry + `traverseViewOnce` target reads via the visibility seam
- All three consumers inherit the fix
- Tests: hidden-intermediary reachability (`Entry → H → V`) for the `_views`
API and sections; where-clause-over-hidden case
- **NopACL byte-identical regression for all three consumers** — a principal
who can read the whole traversal sees no change (the guard that the fix doesn't
alter behavior for full-read principals)

**Out of scope:**

- Command payload entity/list scoping (TKT-2FDTJE)
- Making `context: view` commands grantable (TKT-2FDTJE, once this lands)

## Prevention

The systemic gap (why3): a new read-out reader can be added without any
structural signal that it must go through a visibility wrapper. Candidate
preventions to record when this is fixed:
- An arch-lint / grep rule flagging `store.GetEntity` / `ListEntities` /
`ListRelations` calls in `internal/dataentry` outside the visibility seam
(allowlist the seam itself + write-prep paths).
- Or a test that enumerates read-out entry points and asserts each resolves
through a gated reader.

## Acceptance criteria

- A `_views` request whose traversal passes through a hidden intermediary does
not return descendants reachable only via that intermediary
- Entity-detail sections behave the same (hidden intermediary → its descendants
absent from the side panel)
- A `where` clause cannot be used to infer a hidden entity's property values
- Under `NopACL`, `_views` responses and section output are byte-identical to
today (regression)
- The `api_v1.go:2611` "chokepoint" comment becomes true (or is corrected)
- A prevention measure is added (see Prevention)

## Relationship to TKT-2FDTJE

TKT-2FDTJE (command payload read-gating) **depends on this**. Once `executeView`
is source-gated, TKT-2FDTJE's view-command support consumes the already-gated
traversal — no command-local traversal gating — and can lift the `context: view`
deferral safely. Entity + list scoping in TKT-2FDTJE is independent and can
proceed in parallel.
