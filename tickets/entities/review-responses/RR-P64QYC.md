---
id: RR-P64QYC
type: review-response
title: GC orphan scan is O(all content) per tick
finding: The drift-ledger design says each tick 'computes the orphan set' by scanning entities — a full content read of the graph every interval, on top of the existing version sweep's candidate scans. The orphan NAMES are already known from the schema diff (deleted properties, deleted types); the ledger should be keyed by schema name (type or type.property), populated from the classifier's drift report, and content should only be scanned when counting/applying — with targeted queries on pg where possible.
severity: minor
resolution: 'Amendment A6: drift ledger is keyed by schema name (type or type.property) and populated from the classifier''s drift report; content is scanned only when counting for dry-run/notices and when applying deletions, with targeted pg queries where possible. No full-graph scan per tick.'
status: addressed
---
