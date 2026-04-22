---
id: RR-IL1WV
type: review-response
title: 'Leverage: factory doesn''t wire SafeFS.OnPostWrite to FSStore.RecordWrite'
finding: app/factory.go constructs FSStore but never calls safeFS.OnPostWrite(s.RecordWrite). The entire self-echo LRU is dead code in production — every store write triggers a duplicate reconcile from the watcher.
severity: significant
reason: 'Pre-existing bug (predates this PR) — not a regression. Should be tracked as its own ticket since it''s a correctness/performance issue independent of the RootedFS migration. Created BUG follow-up: TKT-TODO (to be filed after this PR). Explicitly out of scope for TKT-3TA1H.'
status: deferred
---
