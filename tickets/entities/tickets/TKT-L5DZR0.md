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

   **UPDATE 2026-08-22 — a SECOND reopen-only critical has now been found,
   which makes this a pattern rather than bad luck. Raise the priority.**
   TKT-WAV8XP PR-B's code review found that both fsstore index-CONSTRUCTION
   sites (`index.go:127` cached-index restore, `:246` cold directory scan)
   still sorted `entityOrder` with plain `sortStrings` while every MUTATION
   site had been converted to `storeutil.CompareStateKeys`. The two halves
   only meet after a restart, so the result was a **panic on the ordinary
   default-world delete path after any process restart** (`SortedRemoveFunc
   called with missing key`), plus silently wrong world paging. See
   RR-IDXSRT.

   Both criticals share one root cause in the harness, not in the code:
   `storetest`'s factory only ever builds a FRESH store, so the suite is
   **structurally blind to any bug living in index/cache reconstruction** —
   no amount of care in writing conformance cases can catch this class.
   Two independent occurrences in two consecutive tickets is the argument
   for building `ReopenFactory` rather than continuing to patch it
   per-backend. TKT-WAV8XP PR-B added a local
   `TestReopenPreservesStateKeyOrdering` (covering both construction paths),
   which closes this instance for fsstore only.

   **SECOND STRUCTURAL GAP in the same harness (2026-08-22): optional
   capabilities are tested through whichever implementation the suite
   happens to reach, which is often a FALLBACK rather than a native one.**
   `store.HeaderReader` is the worked example: fsstore does NOT implement
   `ListEntityHeaders` and is served by the generic `store.ListEntityHeaders`
   fallback over `ListEntities`, which inherits correctness for free;
   memstore and pgstore implement it NATIVELY. So a conformance case can
   pass on every backend while no native implementation is actually
   exercised for the property under test. Found during TKT-WAV8XP PR-B:
   the header path had zero world coverage (`grep -c World
   storetest/header.go` = 0) and the gap was invisible precisely BECAUSE
   the fallback works. See RR-HDRWLD.

   The same shape appeared twice more in that ticket: RR-GQWRLD
   (`GraphQuery` looked covered because the `EntityQuery` path was
   correct) and the `ValidateEntityQuery` 3-vs-4 call-site asymmetry
   (looked like a missing guard because fsstore inherits one via the
   fallback). Three instances, one shape.

   **Proposed check:** for each optional store capability, assert the
   suite exercises the NATIVE implementation on backends that have one,
   not merely the fallback. Proof method that works today: neuter the
   native implementation and require the case to fail (this is how
   RR-HDRWLD's case was validated — testing it via fsstore's fallback
   would have proved nothing, since that arm cannot fail that way).

   **Why both gaps belong on one ticket:** they share a root cause —
   `storetest` cannot distinguish "this backend IMPLEMENTS the contract"
   from "this backend INHERITS something that happens to satisfy it,"
   whether the inheritance is a fresh-store default or a generic
   fallback. Two distinct blind spots in one harness is a considerably
   stronger case for investing in it than either alone.
3. **Reviewer leverage idea — typed default-face wrapper:** the Step-1
observer rule "indexers skip non-default-pointer events" is enforced by
convention at each observer. A small typed wrapper (e.g. the bridge hands
indexing observers a default-face-only event stream) would make the rule
compiler-enforced until Step 5's per-world indexing replaces it.

Each item stands alone; do them in one pass or cherry-pick.
