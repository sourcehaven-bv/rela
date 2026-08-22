---
id: RR-OP2U10
type: review-response
title: pg lock + pool_max_conns=1 self-deadlocks the migration
finding: TryMigrationLock pins one pool connection for the whole run while the migration's Tx batches acquire further connections from the same pool; with pool_max_conns=1 the first batch blocks forever against the lock's pinned connection — a hard hang, not an error.
severity: significant
resolution: 'Preflight added (commit d514cc8e): TryMigrationLock refuses when pool.Config().MaxConns < 2 with a message naming the remedy ("migration lock needs pool_max_conns >= 2 …"). The pinned-connection requirement itself is correct (session-scoped advisory lock, sweep-tick rule) and documented at the method.'
status: addressed
---
