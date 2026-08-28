---
id: RR-FTY3QA
type: review-response
title: Cardinality classifier mishandles absent bounds (unset↔0 min is a fake migration)
finding: 'compareCardinality treated any nil→value transition as tightening via an unreachable guard: `min_outgoing: unset → 0` — a semantic no-op — classified needs-migration, blocking the gate on a schema edit that changes nothing, with gen only able to emit a TODO comment (unresolvable without hand-editing).'
severity: significant
resolution: 'Rewrote compareCardinality on EFFECTIVE values (commit bddc13f3): absent min = 0, absent max = unbounded. Equal effective values produce no delta; loosening is additive; tightening is needs-migration. unset↔0 min and 0→unset min are now no-ops; max unset→N correctly tightens; bound removal correctly loosens. Pinned by TestCompareShapes_CardinalityEffectiveValues (6 cases).'
status: addressed
---
