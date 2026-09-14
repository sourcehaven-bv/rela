---
id: BUGA-ILW3AK
type: bug-analysis-checklist
title: 'Analysis: executeView traversal is ungated: hidden intermediaries leak reachable descendants in _views API, entity-detail sections, and view commands'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced by `TestACLViewTraversal_HiddenIntermediaryBlocksReachability`
(`internal/dataentry/acl_view_traversal_test.go`). Fixture:

```text
TKT-ENTRY --links--> SEC-HIDDEN --links--> TKT-VISIBLE
```

`alice` holds `read: [ticket]` only, so `SEC-HIDDEN` (type `secret`) is
row-gated. `TKT-VISIBLE` is reachable ONLY through the hidden node. With the
gate removed the recursive view returns `[TKT-VISIBLE]`; with it, `[]`.

Conditions: any recursive `traverse:` rule whose path crosses an entity the
principal cannot read. Needs no special environment — default fs backend, a
declarative `acl.yaml`, and any of the three view surfaces.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

why1-why3 are recorded on BUG-9Z20WH. Analysis on current `develop` **revised
why1**: the original text blamed `traverseViewOnce -> st.GetEntity`, which the
`viewsHandler` refactor (TKT-1U8XYN) has since deleted. The defect survived the
refactor in a sharper form — the recursive walk is now **id-only** and loads no
entities at all, so the leak is no longer "an ungated read" but "an ungated
**frontier expansion**". See the bug's revised why1 and the Fix note below.

why4/why5 (added during this analysis, recorded on the bug):

- **why4** — the read-side ACL decomposition (DEC-ZBI39P) modeled visibility as
  a property of *rows on the way out*, so every seam it introduced is a filter
  over a result set. Reachability is not a property of a row; it is a property
  of the *walk*. No seam in the design expresses "this node may not be traversed
  through", so there was nowhere for the traversal to plug into even if someone
  had looked.
- **why5** — systemic: the project has a rule for read-out paths ("go through
  the visibility wrappers") but no rule, type, or lint distinguishing a
  **read-out** from a **traversal**. Filtering a result set and gating a graph
  walk are different operations with different failure modes, and treating the
  second as an instance of the first is what makes this class of leak keep
  looking fixed when it is not (RR-3T18K9 already had to argue that
  result-filtering is not source-gating).

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Approach (revised against current `develop`).** The bug's original plan — gate
`traverseViewOnce`'s `GetEntity` — is no longer implementable: that function is
gone. The traversal now splits into id-collection
(`traverseViewMany` / `traverseViewBreadthFirst`) and a single batched load
(`loadViewEntities`). The fix therefore source-gates at **two** points:

1. **`traverseViewBreadthFirst`'s frontier** — the load-bearing gate. The walk
   never materializes an entity, so a hidden node is otherwise expanded on its
   id alone. This is the only point at which a hidden node's unreadability can
   stop the walk.
2. **`loadViewEntities`** — keeps a hidden entity out of a collection at all,
   including on the non-recursive path that never enters the BFS.

Plus the defensive entry gate in `executeViewRef`, which the command surface
(`commands.go`) genuinely depends on — it applies no entry gate of its own.

**Verified NOT sufficient** (measured, see IMPL-AP42ZG): gating only
`loadViewEntities` leaves the leak fully open, because the descendant is
readable in its own right and so passes both the load gate and the out-gate
(`viewReader.Filter`). This is the same trap RR-3T18K9 flagged, re-encountered
in the refactored shape.

**Regression test planned:** hidden-intermediary reachability through
`executeView`, the `_views` wire handler, and the side panel; a `where:`
probe over a hidden property; and a NopACL byte-identical guard.

**Related areas checked.** The three known consumers (`_views`, side-panel
sections, view commands) all funnel through the one `executeView`, so they
inherit the fix. `traverseViewMany` (non-recursive) is covered by the load gate.
No other traversal in `internal/dataentry` walks ids across an ACL boundary.
