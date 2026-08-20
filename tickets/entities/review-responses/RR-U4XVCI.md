---
id: RR-U4XVCI
type: review-response
title: enum_values ledger entries permanently wedge the GC sweep
finding: 'RecordDrift wrote entries with Kind enum_values, but GC.collect had no case for that kind: it returned an error that aborted the whole tick before SaveLedger, and subjectInShape could never prune the entry. One removed enum value therefore poisoned every future GC tick (server sweep and CLI alike) forever — all legitimate drift GC silently stopped. Confirmed by execution during review.'
severity: critical
resolution: 'Two-layer fix (commit bddc13f3): (1) enum-value drift is no longer ledgered at all — consistent with the ticket''s out-of-scope decision that stale enum values are map_values territory, not orphan cleanup (the property still exists; a sweep must never erase live-looking values); (2) collect() now drops-and-warns on any unknown ledger kind instead of erroring, so a legacy or future entry can never wedge the sweep (the ledger is rebuildable bookkeeping). Pinned by TestGC_EnumValueDriftNeverWedgesTheSweep, which covers both layers.'
status: addressed
---
