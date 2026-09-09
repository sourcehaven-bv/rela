---
id: TKT-A23L87
type: ticket
title: Audit already-deleted relations when a cascade delete fails partway
kind: enhancement
priority: low
effort: s
status: done
---

## Description

When a cascade delete fails partway — an I/O error removing relation R2 after R1
is already off disk — `fsstore.deleteEntity` returns `nil, err`. The manager
returns early on that error, so the relations that **were** deleted get no audit
record. The audit log therefore does not reflect the real system state.

The fail-secure ordering from #899 (relations first, abort before touching the
entity file) is correct and stays; this is the remaining edge case it left. The
store's own comment acknowledges it: *"Not transactional: a relation file
removed before a later failure stays removed."*

Note fsstore's `Tx` is a write mutex with no rollback, so on that backend the
partial deletion genuinely persists.

GitHub issue #929. Severity: low. Basis: POLICY-015 §4 — audit records must
reflect actual system state.

## Third call site (IB-review, RR-YGYNO1)

The first fix covered `Manager.DeleteEntity` and `cascadeHost.DeleteEntity`.
It missed `dropEntitiesStep.Run` in `internal/datamigration/steps.go`, which
calls `Store.DeleteEntity` directly, bypassing entitymanager, and returned on
error without reading `del.DeletedRelations`.

That path had the exact defect this ticket exists to fix. It is covered now,
with both call sites sharing `captureCascaded`.

The lesson is the search, not the fix: "every caller of DeleteEntity" is the
question that finds all three, and grepping for the entitymanager methods finds
only two. A store method reached directly by a non-obvious caller is the shape
worth checking whenever an audit gap is closed at the manager layer.
