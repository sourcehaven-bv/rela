---
id: RR-YCMFES
type: review-response
title: 'Plan reinvents recursive traversal: ViewTraverse already does cycle-safe depth-limited recursion'
finding: 'The plan proposes a bespoke DFS with an ancestor set for cycle detection, and cites internal/tracer as only-partially-suitable. But internal/dataentry/views.go:163 already implements traverseViewRecursive(ctx, sourceID, rule, depth, maxDepth, visited) — a depth-limited DFS with a visited cycle guard, driven by ViewTraverse config (dataentryconfig/config.go:1017-1025: Follow, FollowIncoming, Recursive bool, MaxDepth int, Where). Default maxRecursionDepth=10 at views.go:91, overridable per rule. This is the existing, cycle-safe, ACL-aware precedent for exactly what the gantt needs. The plan must either reuse it or state explicitly why it cannot — the honest reason being that ViewTraverse lives in the view/section pipeline and flattens results into named collections, whereas a gantt needs the TREE SHAPE preserved for the roll-up fold. That is a real difference, but it is a reason to extract/generalise the traversal, not to write a second independent copy. CLAUDE.md warns against exactly this duplication (see the storeutil TopValues hoist, 9c87c5c5, where one of three copies had drifted).'
severity: significant
resolution: 'Plan updated: the Research section now documents traverseViewRecursive (views.go:163-176) and its ViewTraverse config (config.go:1017-1025) as ''the primitive to reuse''. The Approach section states traversal reuses the existing walk rather than a second copy, names the genuine difference (ViewTraverse flattens into named collections; the gantt needs tree shape retained for the fold), and prescribes extracting a tree-retaining variant that both call. Cites the storeutil TopValues hoist (9c87c5c5) as the precedent for why a second copy drifts. Separately added: load the edge set with one store.ListRelations per hierarchy relation type rather than querying per node.'
status: addressed
---

## Finding

The plan's Approach section says traversal is "a DFS over the union of
`hierarchy` relation types, carrying an ancestor set for cycle detection", and
dismisses `internal/tracer` as whole-graph and outgoing-only.

It never mentions that **`internal/dataentry` already has recursive traversal**:

- `traverseViewRecursive(ctx, sourceID, rule, depth, maxDepth, visited)` —
`internal/dataentry/views.go:163-176`, a depth-limited DFS with a `visited`
cycle guard
- Driven by `ViewTraverse` (`dataentryconfig/config.go:1017-1025`) which
already carries `Follow`, `FollowIncoming`, `Recursive bool`, `MaxDepth int`,
`Where`
- `maxRecursionDepth = 10` default at `views.go:91`, per-rule overridable at
`views.go:94-97`
- Single-hop primitive `traverseViewOnce` at `views.go:129` over
`store.ListRelations` with direction

That is the same cycle-safety, the same depth-capping, and the same
relation-following the plan proposes to write from scratch.

## The real difference (which the plan should state, not ignore)

`ViewTraverse` flattens its results into **named collections** for the
view/section pipeline. A gantt needs the **tree shape preserved** so the
post-order roll-up fold can run. That is a genuine structural difference — but
it argues for extracting/generalising the existing walk to optionally retain
parent/child structure, not for a second independent implementation.

CLAUDE.md is explicit about this class of mistake; commit `9c87c5c5` hoisted
`storeutil.TopValues` precisely because "one of three copies had already
drifted".

## Also worth copying rather than re-deriving

`views.go:47-68` shows how that pipeline handles ACL: traversal walks **raw**
store entities by id (a rule's `where:` may name a hidden property), then
row-gating and field redaction are applied **once on the way out** via
`visibility.Redact(...)` and `h.viewReader.Filter(...)`. The comment there also
documents a residual inference channel — which is directly relevant to RR-Y7MINP
and must be read before copying the pattern.

## Resolution

Update the Approach section to either (a) reuse `traverseViewRecursive` via a
tree-retaining variant, or (b) state concretely why a separate walk is
necessary. Option (a) is strongly preferred.
