---
id: RR-FTQE3U
type: review-response
title: DROP/CREATE INDEX at every boot on a large table is a lock/latency hazard
finding: 'DROP INDEX takes ACCESS EXCLUSIVE; running convergence at every boot contends against live traffic during rolling deploys. CONCURRENTLY can''t run in the advisory-locked tx (tension). Resolve: advisory-lock-gated leader does DDL, others fast-path; skip create/drop when already converged so steady-state boots do zero DDL.'
severity: significant
resolution: 'Steady-state boots do ZERO DDL: reconcile diffs desired-vs-actual and only issues CREATE/DROP for the difference (the intersection is a no-op reported as DerivedEnforced). Combined with the advisory lock (RR-QY5S4C), only the leader on an actual change touches DDL; converged boots just introspect. DROP is non-concurrent inside the lock but rare (only when a declaration is removed). Verified by TestDerivedUnique_ReconcileIdempotentAndDrop (second run = enforced/no-op).'
status: addressed
---

DROP INDEX takes ACCESS EXCLUSIVE on the table. Running create/drop convergence
at EVERY store-open means every booting process contends for that lock against
live traffic. Combined with the missing advisory lock and cross-schema hazard, a
rolling deploy of N processes each drop/recreating indexes could stall writes.
Options: DROP/CREATE INDEX CONCURRENTLY (but CONCURRENTLY can't run inside a tx
— conflicts with wrapping reconcile in one advisory-locked tx, a real tension),
OR gate reconcile behind the advisory lock so only ONE process does DDL and the
rest fast-path a no-op introspection. Migration's non-concurrent CREATE INDEX
(0003) is fine only because migration "holds the world"; reconcile-at-boot
doesn't.

REQUIRED: resolve the CONCURRENTLY-vs-tx tension. Recommended: advisory-lock
gate + only-leader-does-DDL + others fast-path; keep DDL non-concurrent inside
the lock, but SKIP a create/drop when the object already matches desired (no-op
when converged) so steady-state boots do zero DDL.
