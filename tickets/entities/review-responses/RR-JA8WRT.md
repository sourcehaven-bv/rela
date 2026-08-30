---
id: RR-JA8WRT
type: review-response
title: Version history now contradicts the audit log on a partial cascade
finding: RR-181AFY made DeletedRelations the single source for all relation-delete capture and I consumed it for audit only
severity: significant
resolution: Confirmed against RR-181AFY. deleteEntityInTx now returns the capture on the error path too and auditPartialCascade became recordPartialCascade - it emits BOTH the audit record and the relation delete-version for each relation the store actually removed. Driven by res.DeletedRelations (only what was really removed) with the captured pre-delete snapshots supplying the version content. The two logs can no longer contradict each other.
status: addressed
---

`deleteEntityInTx` now propagates `res` but still returns `nil` for `captured`,
and `DeleteEntity`'s `txErr` branch audits and returns — never reaching the `if
captured != nil` block that calls `recordRelationVersion`.

So after a partial cascade the audit log has `delete-relation` rows and relation
version history has **no delete marker** for those same relations.

RR-181AFY (critical, resolved) established the rule I just broke:

> `DeleteResult.DeletedRelations` is now the single source for ALL
> relation-delete capture (cascade + explicit) so there is one path, not two
> that drift.

I made `DeletedRelations` non-empty on the error path and consumed it for audit
only. Per that same finding, the rows are gone from disk so **no sweep can
backfill them** — silent, unrecoverable history loss.

Half-fixing observability is worse than not fixing it: one log now tells the
truth, the other still does not, and they disagree.
