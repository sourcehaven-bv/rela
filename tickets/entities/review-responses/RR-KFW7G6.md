---
id: RR-KFW7G6
type: review-response
title: applyMigration used a DEFERRED transaction, contradicting the package's measured BEGIN IMMEDIATE rule
finding: applyMigration used db.BeginTx(ctx, nil), which is DEFERRED. The package doc and Store.Tx both record, from the DEC-LFSYNY spike, that a deferred transaction reading before writing must upgrade its lock mid-flight and that the upgrade returns SQLITE_BUSY regardless of busy_timeout. The ladder was exempt only incidentally — the process lock is held and the one current step is write-only. A backfill (read rows, transform, write back) is the shape a future step is most likely to take, and mid-migration on user data is the worst moment to rediscover the rule.
severity: significant
resolution: applyMigration now pins a connection and issues BEGIN IMMEDIATE / COMMIT explicitly, matching Store.Tx. It also carries Tx's deferred-ROLLBACK-on-WithoutCancel guard, because a connection returned to the pool with a transaction still open poisons every later use of it. The migration element type changed from []string to func(context.Context, *sql.Conn) error so a backfill fits at all; sqlSteps keeps the pure-DDL case a one-liner.
status: addressed
---
