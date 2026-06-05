---
id: RR-ITQN87
type: review-response
title: NOTIFY can't be 'in the same transaction' for 5 single-statement writes
finding: 'Design-review verification: the plan says each write emits pg_notify ''in the same transaction so it only fires on commit'', but 5 of the write methods are SINGLE-STATEMENT AUTOCOMMIT, not explicit transactions: CreateEntity (entity.go:219 single QueryRow INSERT...RETURNING), UpdateEntity (entity.go:249), CreateRelation (relation.go:151), UpdateRelation (relation.go:178), DeleteRelation (relation.go:193 Exec). Only DeleteEntity, RenameEntity, AttachFile use tx:=s.db.Begin(). For a single autocommit statement there is no open tx to attach pg_notify to. Options: (a) wrap those 5 in an explicit tx so the write + pg_notify commit atomically; (b) call pg_notify as a separate statement after the write (NOT atomic — write commits, process could die before notify; the seq catch-up backstop would still recover it, so this is acceptable but means notify is best-effort, which is fine since it''s already a hint).'
severity: critical
resolution: 'Implemented the chosen option: wrapped the 5 single-statement writes (CreateEntity, UpdateEntity, CreateRelation, UpdateRelation, DeleteRelation) in an explicit tx := s.db.Begin(); notify(ctx, tx, ev) is called inside the tx just before Commit, and emit() after commit. NOTIFY now fires atomically with the write and never on rollback. This unifies all write paths on the same tx pattern DeleteEntity/RenameEntity/AttachFile already used.'
status: addressed
---

## Resolution (plan update)

The notification is already a best-effort hint (the seq catch-up is the
durability backbone), so strict atomicity isn't required. Decision: **wrap the 5
single-statement writes in an explicit transaction** anyway, so write +
`pg_notify` are atomic and a notification never fires for a write that rolled
back (cleaner, and the txn overhead on a single statement is negligible). This
unifies all write paths on the tx pattern DeleteEntity/RenameEntity/ AttachFile
already use. The producer helper becomes `notify(ctx, tx, event)` called inside
every write's tx, just before commit.

Alternative (rejected as messier): keep autocommit + a separate post-commit
`pg_notify` and lean on catch-up for the crash window. Rejected because wrapping
in a tx is simple and removes the phantom-notify-on-rollback case entirely.
