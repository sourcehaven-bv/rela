---
id: TKT-I3T5ST
type: ticket
title: 'pgstore: schema-level state-family invariant via self-referential composite FK'
kind: refactor
priority: medium
effort: s
status: backlog
---

Leverage idea from TKT-DOFYR1 PR-B review (#1388). The state-family write
invariant (no headless states — a non-default row requires its default row) is
currently enforced app-level with a `FOR SHARE` probe on the default row
(check-then-act was demonstrably racy under READ COMMITTED before that fix).

The converged shape is schema-level: a self-referential composite FK — each
non-default state row `(id, pointer<>'')` references the family's default row
`(id, '')` (e.g. via a generated/constant column), with `ON DELETE CASCADE` and
`ON UPDATE CASCADE`. That makes headless states UNREPRESENTABLE in pg and gives
the family delete/rename cascade for free, replacing the probe.

Considerations for whoever picks this up:
- fs/mem keep app-level enforcement — the storetest contract stays the
shared truth; this is pg defense-in-depth, not a contract change.
- Interaction with rename's atomic re-key (ON UPDATE CASCADE ordering) and
with migration 0011's compound PK — needs a forward-only migration.
- The load-tolerance rule is unaffected (fsstore disk families may still be
headless; pg simply cannot represent them — that asymmetry is fine and should be
documented).
