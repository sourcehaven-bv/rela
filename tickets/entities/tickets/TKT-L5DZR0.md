---
id: TKT-L5DZR0
type: ticket
title: 'fs/mem state-family follow-ups from PR-A review: prefix-walk scans, ReopenFactory, typed default-face'
kind: refactor
priority: medium
effort: s
status: backlog
---

Deferred items from TKT-DOFYR1 PR-A's code review (#1386), bundled — all small,
none blocking the arc:

1. **RR-GSOQY1 (the actual deferral):** fsstore/memstore delete + rename do
O(n) full scans to find an id's state family. Fix: keep per-id family membership
discoverable via a sorted-prefix walk (ids sort with their `id@pointer`
siblings) or a small family index. Only matters at large entity counts;
correctness is already pinned by storetest cascades.
2. **Reviewer leverage idea — `storetest.ReopenFactory`:** the CRITICAL
reopen bug (rebuildPropCache double-counting state rows) was only catchable by a
close-and-reopen cycle, which the conformance harness cannot express for
persistent backends. A ReopenFactory capability would make persistence contracts
(caches, persisted indexes, watchers) testable uniformly — pgstore would benefit
too.
3. **Reviewer leverage idea — typed default-face wrapper:** the Step-1
observer rule "indexers skip non-default-pointer events" is enforced by
convention at each observer. A small typed wrapper (e.g. the bridge hands
indexing observers a default-face-only event stream) would make the rule
compiler-enforced until Step 5's per-world indexing replaces it.

Each item stands alone; do them in one pass or cherry-pick.
