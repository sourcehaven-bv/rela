---
id: RR-MYLUSZ
type: review-response
title: FindOrphans gating is per-id sequential — batch by type like filterVisible
finding: The plan resolves each orphan id's type via GetEntity then gates per id. That is O(N) sequential PermitsRead probes — the exact fan-out RR-FRK1 removed from filterVisible. Load the entities (needed anyway for the type), group ids by type, then one PermitsReadMany per distinct type.
severity: minor
resolution: 'Plan revised: FindOrphans loads entities (type needed anyway), groups by type, one PermitsReadMany per distinct type — mirroring filterVisible''s RR-FRK1 batching (PLAN-RR12W4 Approach).'
status: addressed
---
