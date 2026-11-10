---
id: RR-GE6WC2T
type: review-response
title: Ledger is one KV document rewritten per run
finding: Large max_rows ledgers are loaded and written whole.
severity: minor
reason: Fine at the default 100k rows; sharding the ledger is a follow-up for TKT-6GRZHXD.
status: deferred
---
