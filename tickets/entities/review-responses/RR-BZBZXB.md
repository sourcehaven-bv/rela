---
id: RR-BZBZXB
type: review-response
title: A context cancelled at COMMIT poisons the store identically, with no panic required
finding: 'COMMIT ran on the caller''s ctx, so a request cancelled between the last write and the commit made it fail — and the error path returned WITHOUT rolling back, leaving the transaction open on a pooled connection. Same permanent poisoning as the panic case but far more likely in production: a desktop user closing a window or a browser tab mid-save. Reviewer measured 40/40 subsequent Tx failures. The WithoutCancel reasoning was already present on the ROLLBACK path and correct there; it simply had not been applied to COMMIT, which is the path that actually leaves the transaction open.'
severity: critical
resolution: COMMIT now runs on context.WithoutCancel(ctx) — a transaction that has reached the commit is complete, and abandoning it because the caller went away helps no one. The deferred rollback from RR-1 is the backstop. Covered by storetest.RunTxAbnormalExitTests/ContextCancelledDuringTxLeavesStoreUsable; verified non-vacuous by reverting (fails with 'the store must remain usable after a cancelled Tx').
status: addressed
---
