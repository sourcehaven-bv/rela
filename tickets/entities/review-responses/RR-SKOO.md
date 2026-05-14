---
id: RR-SKOO
type: review-response
title: 'Cranky #13: two-write pattern not documented at EntityManager interface'
finding: EntityManager.CreateEntity godoc says nothing about the 'engine sets properties, second write' shape. A non-Manager implementation could skip it.
severity: minor
reason: The two-write shape is a Manager implementation detail, not part of the EntityManager contract. A future implementation that batches into one write would be conformant. Manager.CreateEntity's own godoc explains the pipeline; the interface intentionally doesn't prescribe implementation strategy (CLAUDE.md 'names declare contracts; docs declare invariants').
status: wont-fix
---
