---
id: RR-EKRU4E
type: review-response
title: 'PR 8: DeclarativeGate.WithWorld has no caller'
finding: WithWorld was never called, so its gate always read the default world implicitly.
severity: minor
resolution: Removed WithWorld and the field; GateTraversal names store.DefaultWorld explicitly, pinned on the defaultWorld allowlist until TKT-7IZHP0.
status: addressed
---
