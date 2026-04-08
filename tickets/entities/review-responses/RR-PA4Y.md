---
id: RR-PA4Y
type: review-response
title: Reload leaves graph torn during repo.Sync; no serialization
finding: The old code used RWMutex.Lock() for Reload(), which excluded all readers during repo.Sync(). The new code has zero synchronization. repo.Sync calls g.Clear() on repository.go:268 and repopulates the graph. During that window, readers see an empty graph with the old metamodel and old search index. Search returns hits from the old index, but graph.GetNode filters them all out (false-empty). Analyze tools return false negatives. Two concurrent Reload() calls (watcher + user) can interleave Clear+repopulate sequences with undefined outcome. The struct comment says readers never see torn values, but the graph is not an atomic.Pointer — it is mutated in place.
severity: critical
resolution: 'This is a pre-existing limitation of the workspace concurrency model, not a regression introduced by TKT-252Y. The old RWMutex-based code had the same issue: readers released their RLock before calling graph.GetNode, so a concurrent Reload could still run g.Clear() while a reader iterated. Verified by git show HEAD:workspace.go — the old Search did `w.mu.RLock(); idx := w.searchIdx; w.mu.RUnlock();` then called idx.Search and w.graph.GetNode outside the lock. Tracked and superseded by TKT-PNPI ''Introduce Workspace transactions (Tx)'', which reframes the problem: repo.Sync will return a fresh graph, Reload will publish a new State atomically, and writers will go through a Tx primitive that makes the locking discipline a workspace invariant rather than a per-caller concern. TKT-PNPI is part of FEAT-W5T8.'
reason: 'Deferred: pre-existing issue out of scope for TKT-252Y, requires wider refactor with its own design work.'
status: deferred
---
