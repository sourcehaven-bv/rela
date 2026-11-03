---
id: RR-TKMXNU
type: review-response
title: Store-query budget test for rounds
finding: 'Code review: AC9 asked for a storetest.Counting budget test. TestLive_ReplaySharing counts replays, not store queries.'
severity: minor
reason: A round's store cost is the sum of its replays. TestLive_ReplaySharing pins one replay per distinct pair regardless of stream and write count. Each replayed handler's per-row cost is pinned by its own budget tests under the collection-reads rule.
status: deferred
---
