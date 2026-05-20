---
id: TKT-GFQK
type: ticket
title: Unify data-entry handling of incoming and outgoing relations
kind: refactor
priority: medium
effort: l
status: done
---

## Problem

Data-entry treats incoming and outgoing relation edits as two distinct code
paths even though the data model has no such asymmetry. The metamodel defines an
edge between A and B; from the graph's point of view it is symmetric. The words
"incoming" and "outgoing" exist so that a relation type can attach extra
properties (and a separate display label) on each side, NOT because the edge has
an owner.

Yet today the SPA, the wire, and the backend reconciler all behave as if
**outgoing edges are the canonical representation** and incoming edges are a
second-class projection that needs special plumbing:

- **Wire**: the unified PATCH from TKT-6WLSW / TKT-ZEKO4 carries outgoing
edges only. Incoming widgets fall back to per-edge `POST/PATCH/DELETE
/relations/{type}/{targetId}?direction=incoming`.
- **Frontend**: `DynamicForm.savePendingIncomingChanges` is a separate
N-request fan-out that runs after the unified PATCH. `RelationPicker` has an
`isIncoming` branch with its own `incomingValue` / `incomingOriginal` snapshots
that mirror what `RelationCards` already maintains for outgoing.
- **Backend**: `handleV1GetRelationType` and `handleV1EntityRelations` swap
source/target inline; the unified-PATCH reconciler (`relations_modern.go`)
doesn't grep-match `incoming` at all — it only mutates outgoing edges of the
path entity.

This is wrong: an edge `A blocks B` is one edge. The widget on A's form ("things
I block") and the widget on B's form ("things blocking me") are two views onto
the same edge. Editing it from either side should produce the same file change.

## What we want

The data model already gives us the right mental model: **a relation widget is a
view onto a peer set**, where the relation type and the path entity together
describe a slice of edges. The widget says "I want this entity to have these
peers via relation `r`." The save path computes the diff and applies it, no
matter which side of the edge the path entity sits on.

`direction` becomes a SPA-side display concern (which set of peers to show,
which side's labels to render) and disappears from the save path entirely.

## Approach (replaces the previous three options)

Since edges are reversible, the wire format already names the edge fully via the
path entity plus the per-edge resource identifier. To unify, the backend
reconciler simply needs to know: "this PATCH's relations entries may refer to
edges where the path entity is the SOURCE or the TARGET; treat both as in-scope
writes of the same canonical edge."

Concretely:

1. **Backend**: extend `relations_modern.go` to compute the desired-state
diff against BOTH outgoing AND incoming edges for any relation named in the
body. The relation type already declares which side accepts which property bag
(`from.properties` vs `to.properties`); per-edge meta keys are resolved against
the side the path entity sits on, not against a hardcoded "outgoing" assumption.
Add a way for the resource identifier to disambiguate when both directions of
the same relation would match the same peer ID (rare, but possible for
self-loops or for relations defined symmetrically) — most naturally by accepting
the inverse name directly in the body's relation-name key. The SPA can use
either the canonical name or the inverse name; the backend treats them as the
same underlying edge set, scoped by direction.

2. **Frontend**: collapse `isIncoming` in `RelationPicker`. The widget
loads its peer list from the existing GET (which already swaps source/target for
incoming), edits it as a peer set, and emits the modern shape into the unified
PATCH using whatever relation key matches the widget's configured `direction`.
No second save channel, no per-edge fan-out.

3. **Cleanup**: delete `savePendingIncomingChanges`. Remove the
`?direction=incoming` query string from the per-edge endpoints (they stay around
for other callers but stop being the SPA's reverse-save path).

This keeps the wire format honest about what the metamodel says: edges are
edges. Direction is a labelling convenience, not a separate storage class.

## Acceptance criteria sketch

(Full ACs after planning verifies the backend reconciler can be extended this
way without churning the on-disk format.)

1. A form with a RelationCards widget configured `direction: incoming`
saves per-edge meta via the same unified PATCH as outgoing. One request, not N.
2. A form with a RelationPicker `direction: incoming` batches into the
same PATCH body.
3. `DynamicForm.savePendingIncomingChanges` is deleted.
4. `RelationPicker.isIncoming` branch is gone; the widget is direction-
agnostic at save time. (Display-time it still uses `direction` to pick which
side's label, `from:`/`to:` properties, and inverse name to render.)
5. The backend's unified-PATCH reconciler accepts both the canonical
relation name and (where defined) the inverse name as the body key, resolving
both to the same edge set scoped by direction relative to the path entity.
6. Existing reverse-relations e2e tests pass without modification.

## Out of scope

- Retiring the per-edge endpoints. They stay around for non-SPA callers;
this ticket only stops the SPA from using them for direction:incoming.
- "Incoming-direction relation columns in lists" (FEAT-0c9l / FEAT-tr9f)
— already shipped and direction-agnostic at the read side.

## Relation to other work

- **Depends on**: TKT-ZEKO4 — establishes the unified PATCH wire format
this ticket extends to be direction-agnostic.
- **Implements**: FEAT-R7SOT — broadens its scope from "widget" to the
whole stack.
- **Unblocks**: TKT-E6094 (autosave) — forms with incoming widgets become
eligible for the same per-property autosave queue without a second channel.

## Effort

l. Backend reconciler extension + SPA widget refactor + e2e. The wire format
change is minimal (the reconciler accepts inverse names as aliases); the SPA
simplification is the bulk of the work.
