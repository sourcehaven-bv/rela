---
id: RR-7CB0
type: review-response
title: RelationFilePath path traversal is exploitable via API v1 URL parsing
finding: 'Plan classified Medium #8 (RelationFilePath using relType without sanitisation) as ''theoretical''. Verification shows relType is extracted from URL path at handleV1EntityRelationType (around line 252) as `parts[3]` and passed straight to relation file path construction. A request to `/api/v1/tickets/TKT-001/relations/..%2Fevil/TKT-002` reaches RelationFilePath with relType=`../evil`, producing `relations/TKT-001--../evil--TKT-002.md`. This is a real path traversal write primitive, not theoretical. Severity should be Critical.'
severity: critical
resolution: 'Plan updated: relType sanitisation is reclassified Critical and validated at the API handler entry (handleV1EntityRelationType and any other URL-parsing site) before reaching RelationFilePath. Validation: must match the metamodel''s known relation types (allowlist), not just reject `..`/`/`. Defensive check also added at RelationFilePath itself as defence in depth.'
status: addressed
---
