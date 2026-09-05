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
