---
id: RR-IDXSRT
type: review-response
title: fsstore index construction sorts entityOrder with the wrong comparator — panic on delete after reopen
finding: "PR-B converted every entityOrder MUTATION site to storeutil.CompareStateKeys (fsstore/entity.go:321,468,603,606; watcher.go:215,250) but left both index CONSTRUCTION sites on plain sortStrings (fsstore/index.go:127 cached-index restore, :246 cold directory scan). The slice is therefore sorted by plain string order at every process start and then binary-searched with CompareStateKeys. SortedRemoveFunc misses a key that IS present and hits its deliberate panic. Reproduced: create PAGE-1/PAGE-10/PAGE-2 each with a published state, close, reopen, DeleteEntity(PAGE-1) -> `panic: storeutil: SortedRemoveFunc called with missing key: PAGE-1@published` at storeutil.go:160 via fsstore/entity.go:468. This is NOT world-gated: it fires on the ordinary default-world delete path with no WorldScope anywhere. The world path is also silently wrong before it gets there (PAGE-1 yields twice, and at limit 1 serves the draft face)."
severity: critical
status: addressed
resolution: "Added fsstore.sortStateKeys (slices.SortFunc over storeutil.CompareStateKeys) and applied it at both construction sites; relationOrder deliberately stays on sortStrings since relations carry no face. Root cause of the test gap: the shared conformance factory only ever builds a FRESH store and never reopens one, so it is structurally blind to index-reconstruction bugs — the earlier mutation-testing of CompareStateKeys proved the comparator was load-bearing but could not prove the index was actually IN that order at open. Added TestReopenPreservesStateKeyOrdering (fsstore/states_reopen_order_test.go) covering BOTH construction paths (cached-restore and cold-scan sort entityOrder separately), the world paging oracle at limits 1/2/3, and the default-world delete."
---

**Finding (code review, TKT-WAV8XP PR-B).**

Found by `/code-review` and reproduced independently before fixing.

The comparator change (RULING 1) was applied thoroughly to every place that
*mutates* `entityOrder`, and not at all to the two places that *build* it. The
two halves only meet after a restart, which is why every test passed: the
conformance suite's `factory()` calls `fsstore.New` on an empty MemFS and never
reopens.

```
panic: storeutil: SortedRemoveFunc called with missing key: PAGE-1@published
  storeutil.SortedRemoveFunc            storeutil.go:160
  fsstore.(*FSStore).deleteEntity       entity.go:468
  fsstore.(*FSStore).DeleteEntity       tx.go:64
```

The hazard ids are the RULING 1 set for the same reason: `@` is 0x40 and the
digits are 0x30-0x39, so `PAGE-10`'s family lands inside `PAGE-1`'s under plain
string order.

**Why this one matters beyond the immediate fix.** The panic has nothing to do
with worlds — it is a regression on the plain delete path that would fire on
any project after a process restart. A conformance suite that never reopens a
store cannot see it. That gap is now closed for fsstore, but the general point
stands for any future index-reconstruction work.
