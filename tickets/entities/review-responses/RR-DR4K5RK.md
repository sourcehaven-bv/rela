---
id: RR-DR4K5RK
type: review-response
title: 'Code: sqlite has no statement budget test'
finding: pgstore pins one statement per call at 10 and 50 rows; sqlite has no equivalent.
severity: minor
reason: sqlitestore has no statement counter (store.QueryStats is wired only on pg pools). Each sqlite method issues exactly one QueryRowContext or collectRows call by construction, and its fallback is elementnaive, whose store-call budget is pinned by TestRanges_ReadsDoNotGrowWithRows. Adding a counter to sqlitestore is outside this ticket.
status: deferred
---
